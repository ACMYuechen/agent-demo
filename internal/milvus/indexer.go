package milvus

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Indexer 实现 Eino indexer.Indexer 接口，基于 Milvus 存储文档。
type Indexer struct {
	store *Store // 底层 Milvus 存储
}

// NewIndexer 创建基于 Milvus 的索引器。
func NewIndexer(store *Store) (*Indexer, error) {
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	return &Indexer{store: store}, nil
}

// Store 将文档嵌入后持久化到 Milvus。
func (i *Indexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) (ids []string, err error) {
	co := indexer.GetCommonOptions(&indexer.Options{Embedding: i.store.embedder}, opts...)

	// 设置 Eino 回调上下文。
	ctx = callbacks.EnsureRunInfo(ctx, i.GetType(), components.ComponentOfIndexer)
	ctx = callbacks.OnStart(ctx, &indexer.CallbackInput{Docs: docs})
	defer func() {
		if err != nil {
			callbacks.OnError(ctx, err)
		}
	}()

	// 提取文档内容用于批量嵌入。
	texts := make([]string, len(docs))
	for idx, doc := range docs {
		texts[idx] = doc.Content
	}

	vectors, err := co.Embedding.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed documents: %w", err)
	}
	if len(vectors) != len(docs) {
		return nil, fmt.Errorf("embedding count mismatch: expected %d, got %d", len(docs), len(vectors))
	}

	// 初始化 Milvus 列数据。
	idCol := column.NewColumnVarChar(fieldID, nil)
	contentCol := column.NewColumnVarChar(fieldContent, nil)
	metadataCol := column.NewColumnJSONBytes(fieldMetadata, nil)
	vectorCol := column.NewColumnFloatVector(fieldVector, i.store.config.VectorDim, nil)

	for idx, doc := range docs {
		id := doc.ID
		if id == "" {
			id = fmt.Sprintf("doc_%d", idx)
		}
		if err := idCol.AppendValue(id); err != nil {
			return nil, fmt.Errorf("append id: %w", err)
		}
		if err := contentCol.AppendValue(doc.Content); err != nil {
			return nil, fmt.Errorf("append content: %w", err)
		}

		metaBytes, merr := sonic.Marshal(doc.MetaData)
		if merr != nil {
			return nil, fmt.Errorf("marshal metadata: %w", merr)
		}
		if err := metadataCol.AppendValue(metaBytes); err != nil {
			return nil, fmt.Errorf("append metadata: %w", err)
		}

		// 将嵌入结果转换为 float32 向量。
		vec := make([]float32, len(vectors[idx]))
		for j, v := range vectors[idx] {
			vec[j] = float32(v)
		}
		if err := vectorCol.AppendValue(vec); err != nil {
			return nil, fmt.Errorf("append vector: %w", err)
		}
	}

	insertOpt := milvusclient.NewColumnBasedInsertOption(
		i.store.collection,
		idCol,
		contentCol,
		metadataCol,
		vectorCol,
	)

	res, err := i.store.client.Insert(ctx, insertOpt)
	if err != nil {
		return nil, fmt.Errorf("milvus insert: %w", err)
	}

	// Flush 使新写入数据对搜索可见。
	if _, err := i.store.client.Flush(ctx, milvusclient.NewFlushOption(i.store.collection)); err != nil {
		return nil, fmt.Errorf("flush collection: %w", err)
	}

	inserted := make([]string, res.IDs.Len())
	for idx := 0; idx < res.IDs.Len(); idx++ {
		s, err := res.IDs.GetAsString(idx)
		if err != nil {
			return nil, fmt.Errorf("get inserted id: %w", err)
		}
		inserted[idx] = s
	}

	callbacks.OnEnd(ctx, &indexer.CallbackOutput{IDs: inserted})
	return inserted, nil
}

// GetType 返回组件类型。
func (i *Indexer) GetType() string {
	return "MilvusIndexer"
}

// IsCallbacksEnabled 启用 Eino 回调。
func (i *Indexer) IsCallbacksEnabled() bool {
	return true
}
