package milvus

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Retriever 实现 Eino retriever.Retriever 接口，基于 Milvus 检索文档。
type Retriever struct {
	store *Store // 底层 Milvus 存储
}

// NewRetriever 创建基于 Milvus 的检索器。
func NewRetriever(store *Store) (*Retriever, error) {
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	return &Retriever{store: store}, nil
}

// Retrieve 在 Milvus 中搜索与查询语义相似的文档。
func (r *Retriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) (docs []*schema.Document, err error) {
	co := retriever.GetCommonOptions(&retriever.Options{
		TopK:           intPtr(r.store.config.TopK),
		ScoreThreshold: &r.store.config.ScoreThreshold,
		Embedding:      r.store.embedder,
	}, opts...)

	// 解析搜索选项，未指定时使用配置默认值。
	topK := r.store.config.TopK
	if co.TopK != nil {
		topK = *co.TopK
	}
	scoreThreshold := r.store.config.ScoreThreshold
	if co.ScoreThreshold != nil {
		scoreThreshold = *co.ScoreThreshold
	}

	// 设置 Eino 回调上下文。
	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfRetriever)
	ctx = callbacks.OnStart(ctx, &retriever.CallbackInput{
		Query:          query,
		TopK:           topK,
		ScoreThreshold: &scoreThreshold,
	})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	start := time.Now()
	vectors, err := co.Embedding.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("expected 1 query vector, got %d", len(vectors))
	}

	// 将查询向量转换为 float32。
	vec := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		vec[i] = float32(v)
	}

	// 构建 ANN 搜索选项。
	searchOpt := milvusclient.NewSearchOption(
		r.store.collection,
		topK,
		[]entity.Vector{entity.FloatVector(vec)},
	).
		WithANNSField(fieldVector).
		WithOutputFields(fieldID, fieldContent, fieldMetadata).
		WithConsistencyLevel(entity.ClBounded).
		WithAnnParam(r.store.buildAnnParam())

	results, err := r.store.client.Search(ctx, searchOpt)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	// 解析搜索结果并按分数阈值过滤。
	docs = make([]*schema.Document, 0, topK)
	for _, res := range results {
		if res.Err != nil {
			return nil, fmt.Errorf("milvus search result error: %w", res.Err)
		}
		for idx := 0; idx < res.Len(); idx++ {
			slice := res.Slice(idx, idx+1)
			doc, err := r.parseResult(&slice)
			if err != nil {
				return nil, fmt.Errorf("parse search result at %d: %w", idx, err)
			}
			if len(doc) == 0 {
				continue
			}
			doc[0].WithScore(float64(res.Scores[idx]))
			if float64(res.Scores[idx]) < scoreThreshold {
				continue
			}
			docs = append(docs, doc[0])
		}
	}

	latency := time.Since(start).Milliseconds()
	callbacks.OnEnd(ctx, &retriever.CallbackOutput{
		Docs: docs,
	})

	// 保留 latency 未来使用
	_ = latency
	return docs, nil
}

// GetType 返回组件类型。
func (r *Retriever) GetType() string {
	return "MilvusRetriever"
}

// IsCallbacksEnabled 启用 Eino 回调。
func (r *Retriever) IsCallbacksEnabled() bool {
	return true
}

// parseResult 将 Milvus 搜索结果解析为 Eino Document。
func (r *Retriever) parseResult(res *milvusclient.ResultSet) ([]*schema.Document, error) {
	idCol := res.GetColumn(fieldID)
	contentCol := res.GetColumn(fieldContent)
	metadataCol := res.GetColumn(fieldMetadata)

	if idCol == nil || contentCol == nil {
		return nil, fmt.Errorf("missing required output fields")
	}

	docs := make([]*schema.Document, 0, res.Len())
	for idx := 0; idx < res.Len(); idx++ {
		id, err := idCol.GetAsString(idx)
		if err != nil {
			return nil, err
		}
		content, err := contentCol.GetAsString(idx)
		if err != nil {
			return nil, err
		}

		metadata := make(map[string]any)
		if metadataCol != nil {
			raw, err := metadataCol.GetAsString(idx)
			if err == nil && raw != "" {
				_ = sonic.UnmarshalString(raw, &metadata)
			}
		}

		docs = append(docs, &schema.Document{
			ID:       id,
			Content:  content,
			MetaData: metadata,
		})
	}
	return docs, nil
}

// intPtr 返回 int 指针。
func intPtr(v int) *int {
	return &v
}
