// Package embedding 提供嵌入模型便捷构造器。
package embedding

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	openaiEmbedding "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/yuechen/agent-demo/internal/config"
)

// NewOpenAIEmbedder 从配置创建 OpenAI 嵌入客户端。
func NewOpenAIEmbedder(ctx context.Context, cfg *config.OpenAI) (embedding.Embedder, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai api_key is required")
	}
	if cfg.EmbedModel == "" {
		return nil, fmt.Errorf("openai embed_model is required")
	}

	emb, err := openaiEmbedding.NewEmbedder(ctx, &openaiEmbedding.EmbeddingConfig{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.EmbedModel,
		Timeout: cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("create openai embedder: %w", err)
	}
	return emb, nil
}
