// Package model 提供大语言模型便捷构造器。
package model

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/yuechen/agent-demo/internal/config"
)

// NewOpenAIChatModel 从配置创建 OpenAI 对话模型。
func NewOpenAIChatModel(ctx context.Context, cfg *config.OpenAI) (model.BaseChatModel, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("openai api_key is required")
	}
	if cfg.ChatModel == "" {
		return nil, fmt.Errorf("openai chat_model is required")
	}

	// 取地址以满足 OpenAI 组件的指针参数。
	temperature := cfg.Temperature
	maxTokens := cfg.MaxTokens

	chatModel, err := openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{
		APIKey:      cfg.APIKey,
		BaseURL:     cfg.BaseURL,
		Model:       cfg.ChatModel,
		Timeout:     cfg.Timeout,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("create openai chat model: %w", err)
	}
	return chatModel, nil
}
