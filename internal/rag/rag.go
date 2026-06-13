package rag

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	einoSchema "github.com/cloudwego/eino/schema"
)

// Pipeline 编排知识加载、切分与索引。
type Pipeline struct {
	loader    document.Loader
	splitter  document.Transformer
	indexer   indexer.Indexer
	retriever retriever.Retriever
}

// PipelineConfig 保存 RAG 流水线所需组件。
type PipelineConfig struct {
	Loader    document.Loader
	Splitter  document.Transformer
	Indexer   indexer.Indexer
	Retriever retriever.Retriever
}

// NewPipeline 从配置组件创建 RAG 流水线。
func NewPipeline(cfg *PipelineConfig) (*Pipeline, error) {
	if cfg.Indexer == nil {
		return nil, fmt.Errorf("indexer is required")
	}
	if cfg.Retriever == nil {
		return nil, fmt.Errorf("retriever is required")
	}
	if cfg.Loader == nil {
		cfg.Loader = NewStringLoader()
	}
	if cfg.Splitter == nil {
		cfg.Splitter = NewRecursiveSplitter()
	}
	return &Pipeline{
		loader:    cfg.Loader,
		splitter:  cfg.Splitter,
		indexer:   cfg.Indexer,
		retriever: cfg.Retriever,
	}, nil
}

// IndexSource 从源 URI 加载、切分并存储文档。
func (p *Pipeline) IndexSource(ctx context.Context, uri string) ([]string, error) {
	docs, err := p.loader.Load(ctx, document.Source{URI: uri})
	if err != nil {
		return nil, fmt.Errorf("load source: %w", err)
	}
	chunks, err := p.splitter.Transform(ctx, docs)
	if err != nil {
		return nil, fmt.Errorf("split documents: %w", err)
	}
	return p.indexer.Store(ctx, chunks)
}

// IndexDocuments 存储已有文档，可选先切分。
func (p *Pipeline) IndexDocuments(ctx context.Context, docs []*einoSchema.Document, split bool) ([]string, error) {
	if split {
		chunks, err := p.splitter.Transform(ctx, docs)
		if err != nil {
			return nil, fmt.Errorf("split documents: %w", err)
		}
		docs = chunks
	}
	return p.indexer.Store(ctx, docs)
}

// Retrieve 从知识库检索相关文档。
func (p *Pipeline) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*einoSchema.Document, error) {
	return p.retriever.Retrieve(ctx, query, opts...)
}
