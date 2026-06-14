// Package agent 实现 ReAct 风格的单 Agent 编排层。
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/sirupsen/logrus"
	"github.com/yuechen/agent-demo/internal/config"
	"github.com/yuechen/agent-demo/internal/contextmgr"
)

// Service 编排大模型、RAG 检索与工具调用。
type Service struct {
	cfg          *config.Agent
	chatModel    model.BaseChatModel
	contextMgr   contextmgr.Manager
	toolsNode    *compose.ToolsNode
	toolInfos    []*schema.ToolInfo
	enableRAG    bool
	retriever    retriever.Retriever
	systemPrompt string
	maxToolSteps int
	maxHistory   int
}

// ServiceConfig 保存 Agent 服务依赖。
type ServiceConfig struct {
	Config     *config.Agent
	ChatModel  model.BaseChatModel
	ContextMgr contextmgr.Manager
	Tools      []tool.InvokableTool
	Retriever  retriever.Retriever
}

// NewService 创建 Agent 服务。
func NewService(cfg *ServiceConfig) (*Service, error) {
	if cfg.ChatModel == nil {
		return nil, fmt.Errorf("chat model is required")
	}
	if cfg.ContextMgr == nil {
		return nil, fmt.Errorf("context manager is required")
	}

	svc := &Service{
		cfg:          cfg.Config,
		chatModel:    cfg.ChatModel,
		contextMgr:   cfg.ContextMgr,
		enableRAG:    cfg.Config.EnableRAG,
		retriever:    cfg.Retriever,
		systemPrompt: cfg.Config.SystemPrompt,
		maxToolSteps: cfg.Config.MaxToolSteps,
		maxHistory:   cfg.Config.MaxHistory,
	}

	// 如果配置了工具，则创建 ToolsNode 并收集 ToolInfo。
	if len(cfg.Tools) > 0 {
		toolList := make([]tool.BaseTool, 0, len(cfg.Tools))
		infos := make([]*schema.ToolInfo, 0, len(cfg.Tools))
		for _, t := range cfg.Tools {
			toolList = append(toolList, t)
			info, err := t.Info(context.Background())
			if err != nil {
				return nil, fmt.Errorf("get tool info: %w", err)
			}
			infos = append(infos, info)
		}
		tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
			Tools: toolList,
		})
		if err != nil {
			return nil, fmt.Errorf("create tools node: %w", err)
		}
		svc.toolsNode = tn
		svc.toolInfos = infos
	}

	return svc, nil
}

// Run 执行一次对话回合（支持工具调用循环）。
func (s *Service) Run(ctx context.Context, threadID string, userInput string) (*schema.Message, error) {
	thread, err := s.contextMgr.GetThread(ctx, threadID)
	if err != nil {
		// 若线程不存在则自动创建。
		newThread, err := s.contextMgr.CreateThread(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("create thread: %w", err)
		}
		thread = newThread
		threadID = thread.ID
	}

	if err := s.contextMgr.AddMessage(ctx, threadID, contextmgr.FromEinoMessage(threadID, &schema.Message{
		Role:    schema.User,
		Content: userInput,
	})); err != nil {
		return nil, fmt.Errorf("store user message: %w", err)
	}

	messages, err := s.buildMessages(ctx, threadID, userInput)
	if err != nil {
		return nil, err
	}

	// ReAct 循环：模型生成 → 工具调用 → 再次生成，直到无工具调用或达到最大步数。
	var response *schema.Message
	for step := 0; step < s.maxToolSteps; step++ {
		opts := []model.Option{}
		if len(s.toolInfos) > 0 {
			opts = append(opts, model.WithTools(s.toolInfos))
		}

		resp, err := s.chatModel.Generate(ctx, messages, opts...)
		if err != nil {
			return nil, fmt.Errorf("model generate: %w", err)
		}
		response = resp

		if len(resp.ToolCalls) == 0 {
			break
		}

		logrus.WithFields(logrus.Fields{
			"thread_id": threadID,
			"step":      step,
			"tools":     toolCallNames(resp.ToolCalls),
		}).Info("executing tool calls")

		toolMsgs, err := s.toolsNode.Invoke(ctx, resp)
		if err != nil {
			return nil, fmt.Errorf("tool execution: %w", err)
		}

		messages = append(messages, resp)
		messages = append(messages, toolMsgs...)
	}

	if err := s.contextMgr.AddMessage(ctx, threadID, contextmgr.FromEinoMessage(threadID, response)); err != nil {
		return nil, fmt.Errorf("store assistant message: %w", err)
	}

	return response, nil
}

// RunWithRAG 使用显式检索结果增强后执行对话回合。
func (s *Service) RunWithRAG(ctx context.Context, threadID string, userInput string, query string) (*schema.Message, error) {
	if s.retriever == nil {
		return s.Run(ctx, threadID, userInput)
	}
	docs, err := s.retriever.Retrieve(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}
	if len(docs) > 0 {
		context := formatRetrievedContext(docs)
		userInput = fmt.Sprintf("%s\n\nRelevant context:\n%s", userInput, context)
	}
	return s.Run(ctx, threadID, userInput)
}

// buildMessages 组装模型输入消息列表。
func (s *Service) buildMessages(ctx context.Context, threadID string, userInput string) ([]*schema.Message, error) {
	history, err := s.contextMgr.GetMessages(ctx, threadID, s.maxHistory)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	// 若启用 RAG，则检索相关上下文。
	var contextText string
	if s.enableRAG && s.retriever != nil {
		docs, err := s.retriever.Retrieve(ctx, userInput, retriever.WithTopK(5))
		if err != nil {
			logrus.WithError(err).Warn("rag retrieval failed")
		} else if len(docs) > 0 {
			contextText = formatRetrievedContext(docs)
		}
	}

	content := userInput
	if contextText != "" {
		content = fmt.Sprintf("%s\n\nRelevant context:\n%s", userInput, contextText)
	}

	messages := make([]*schema.Message, 0, len(history)+2)
	messages = append(messages, &schema.Message{
		Role:    schema.System,
		Content: s.systemPrompt,
	})
	messages = append(messages, contextmgr.ToEinoMessages(history)...)
	// 用 RAG 增强后的内容替换最后一条用户消息（已入库）。
	if len(messages) > 0 && messages[len(messages)-1].Role == schema.User {
		messages[len(messages)-1].Content = content
	} else {
		messages = append(messages, &schema.Message{
			Role:    schema.User,
			Content: content,
		})
	}

	return messages, nil
}

// formatRetrievedContext 格式化检索到的文档为上下文文本。
func formatRetrievedContext(docs []*schema.Document) string {
	var b strings.Builder
	for i, doc := range docs {
		b.WriteString(fmt.Sprintf("[%d] ", i+1))
		b.WriteString(doc.Content)
		if i < len(docs)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// toolCallNames 提取工具调用名称列表。
func toolCallNames(calls []schema.ToolCall) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Function.Name
	}
	return names
}
