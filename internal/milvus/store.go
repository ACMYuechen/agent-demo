// Package milvus 基于 Milvus Go SDK v2 提供可优化的向量存储实现。
package milvus

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/sirupsen/logrus"
	"github.com/yuechen/agent-demo/internal/config"
)

const (
	fieldID       = "id"
	fieldContent  = "content"
	fieldVector   = "vector"
	fieldMetadata = "metadata"
)

// Store 封装 Milvus 客户端与集合管理逻辑。
type Store struct {
	client     *milvusclient.Client // Milvus 客户端
	config     *config.Milvus       // 向量库配置
	embedder   embedding.Embedder   // 嵌入模型
	collection string               // 集合名称
}

// NewStore 创建 Milvus 存储，自动确保集合并加载。
func NewStore(ctx context.Context, cfg *config.Milvus, embedder embedding.Embedder) (*Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("milvus config is nil")
	}
	if embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	clientConfig := milvusclient.ClientConfig{
		Address: cfg.Endpoint,
		DBName:  cfg.DBName,
	}
	if cfg.Token != "" {
		clientConfig.APIKey = cfg.Token // Token 作为 APIKey 使用
	}

	cli, err := milvusclient.New(connectCtx, &clientConfig)
	if err != nil {
		return nil, fmt.Errorf("create milvus client: %w", err)
	}

	store := &Store{
		client:     cli,
		config:     cfg,
		embedder:   embedder,
		collection: cfg.Collection,
	}

	if err := store.ensureCollection(ctx); err != nil {
		_ = cli.Close(ctx)
		return nil, fmt.Errorf("ensure collection: %w", err)
	}

	return store, nil
}

// Close 释放 Milvus 客户端连接。
func (s *Store) Close(ctx context.Context) error {
	if s.client != nil {
		return s.client.Close(ctx)
	}
	return nil
}

// Client 暴露底层 Milvus 客户端以供高级操作。
func (s *Store) Client() *milvusclient.Client {
	return s.client
}

// CollectionName 返回配置的集合名。
func (s *Store) CollectionName() string {
	return s.collection
}

// Embedder 返回配置的嵌入模型。
func (s *Store) Embedder() embedding.Embedder {
	return s.embedder
}

// ensureCollection 检查集合并自动创建、加载。
func (s *Store) ensureCollection(ctx context.Context) error {
	has, err := s.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(s.collection))
	if err != nil {
		return fmt.Errorf("has collection: %w", err)
	}

	if has {
		logrus.WithField("collection", s.collection).Info("collection exists, ensuring loaded")
		return s.loadCollection(ctx)
	}

	logrus.WithField("collection", s.collection).Info("creating collection")
	schema := entity.NewSchema().
		WithName(s.collection).
		WithDescription("agent knowledge base collection").
		WithField(entity.NewField().
			WithName(fieldID).
			WithDataType(entity.FieldTypeVarChar).
			WithIsPrimaryKey(true).
			WithMaxLength(255)).
		WithField(entity.NewField().
			WithName(fieldContent).
			WithDataType(entity.FieldTypeVarChar).
			WithMaxLength(int64(s.config.MaxTextLength))).
		WithField(entity.NewField().
			WithName(fieldMetadata).
			WithDataType(entity.FieldTypeJSON)).
		WithField(entity.NewField().
			WithName(fieldVector).
			WithDataType(entity.FieldTypeFloatVector).
			WithDim(int64(s.config.VectorDim)))

	// 创建向量索引选项。
	indexOpt := milvusclient.NewCreateIndexOption(s.collection, fieldVector, s.buildVectorIndex())

	createOpt := milvusclient.NewCreateCollectionOption(s.collection, schema).
		WithConsistencyLevel(entity.ClBounded).
		WithIndexOptions(indexOpt)

	if err := s.client.CreateCollection(ctx, createOpt); err != nil {
		return fmt.Errorf("create collection: %w", err)
	}

	return s.loadCollection(ctx)
}

// loadCollection 加载集合并等待其可用。
func (s *Store) loadCollection(ctx context.Context) error {
	_, err := s.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(s.collection))
	if err != nil {
		return fmt.Errorf("load collection: %w", err)
	}

	// 轮询等待集合加载完成，最长 30 秒。
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		state, err := s.client.GetLoadState(ctx, milvusclient.NewGetLoadStateOption(s.collection))
		if err != nil {
			return fmt.Errorf("get load state: %w", err)
		}
		if state.State == entity.LoadStateLoaded {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("collection %s did not load in time", s.collection)
}

// buildVectorIndex 根据配置构建向量索引。
func (s *Store) buildVectorIndex() index.Index {
	metric := s.parseMetricType()
	switch s.config.IndexType {
	case "HNSW":
		logrus.WithFields(logrus.Fields{
			"type": "HNSW",
			"M":    s.config.HNSWM,
			"ef":   s.config.HNSWEfConstruction,
		}).Info("building vector index")
		return index.NewHNSWIndex(metric, s.config.HNSWM, s.config.HNSWEfConstruction)
	case "IVF_FLAT":
		logrus.WithFields(logrus.Fields{
			"type":   "IVF_FLAT",
			"nlist":  s.config.IVFFLATNList,
		}).Info("building vector index")
		return index.NewIvfFlatIndex(metric, s.config.IVFFLATNList)
	case "FLAT":
		logrus.Info("building FLAT vector index")
		return index.NewFlatIndex(metric)
	default:
		logrus.WithField("type", s.config.IndexType).Warn("unknown index type, falling back to HNSW")
		return index.NewHNSWIndex(metric, s.config.HNSWM, s.config.HNSWEfConstruction)
	}
}

// parseMetricType 解析距离度量类型。
func (s *Store) parseMetricType() entity.MetricType {
	switch s.config.MetricType {
	case "L2":
		return entity.L2
	case "IP":
		return entity.IP
	case "COSINE":
		return entity.COSINE
	default:
		return entity.COSINE
	}
}

// buildAnnParam 构建对应索引的搜索参数。
func (s *Store) buildAnnParam() index.AnnParam {
	switch s.config.IndexType {
	case "HNSW":
		return index.NewHNSWAnnParam(s.config.SearchEf)
	case "IVF_FLAT":
		return index.NewIvfAnnParam(s.config.SearchNProbe)
	case "FLAT":
		return index.NewCustomAnnParam()
	default:
		return index.NewHNSWAnnParam(s.config.SearchEf)
	}
}

// float64ToFloat32 将 float64 向量批量转换为 float32。
func float64ToFloat32(in [][]float64) [][]float32 {
	out := make([][]float32, len(in))
	for i, vec := range in {
		out[i] = make([]float32, len(vec))
		for j, v := range vec {
			out[i][j] = float32(v)
		}
	}
	return out
}
