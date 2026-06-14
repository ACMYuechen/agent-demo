// Package mcp 实现基于 Model Context Protocol 的 Agent 间通信封装。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mark3labs/mcp-go/mcp"
	mcpServer "github.com/mark3labs/mcp-go/server"
	"github.com/yuechen/agent-demo/internal/domain"
)

// AgentHandler 当其他 Agent 委派任务时被调用的回调。
type AgentHandler func(ctx context.Context, task *domain.AgentTask) (*domain.AgentTask, error)

// Server 封装 MCP 服务器，暴露 Agent 能力。
type Server struct {
	info    *domain.AgentInfo
	server  *mcpServer.MCPServer
	sse     *mcpServer.SSEServer
	handler AgentHandler
}

// ServerConfig MCP Agent 服务器配置。
type ServerConfig struct {
	AgentInfo *domain.AgentInfo // Agent 元信息
	Handler   AgentHandler      // 任务处理回调
}

// NewServer 为指定 Agent 创建 MCP 服务器。
func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg.AgentInfo == nil {
		return nil, fmt.Errorf("agent info is required")
	}
	if cfg.Handler == nil {
		return nil, fmt.Errorf("agent handler is required")
	}

	s := mcpServer.NewMCPServer(
		cfg.AgentInfo.Name,
		"1.0.0",
		mcpServer.WithToolCapabilities(true),
		mcpServer.WithResourceCapabilities(true, true),
		mcpServer.WithPromptCapabilities(true),
	)

	m := &Server{
		info:    cfg.AgentInfo,
		server:  s,
		handler: cfg.Handler,
	}

	m.registerTools()
	m.registerResources()
	m.registerPrompts()

	return m, nil
}

// registerTools 注册工具：任务委派与状态查询。
func (m *Server) registerTools() {
	delegateTool := mcp.NewTool(
		"delegate_task",
		mcp.WithDescription(fmt.Sprintf("Delegate a task to %s. Provide task description and context.", m.info.Name)),
		mcp.WithInputSchema[DelegateTaskInput](),
	)

	m.server.AddTool(delegateTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		input, err := parseDelegateInput(request)
		if err != nil {
			return nil, err
		}
		task := &domain.AgentTask{
			Type:        input.Type,
			Description: input.Description,
			Input:       input.Input,
			Context:     input.Context,
			ToAgent:     m.info.ID,
			Status:      domain.TaskRunning,
		}
		result, err := m.handler(ctx, task)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("task failed: %v", err)), nil
		}
		output, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(output)), nil
	})

	statusTool := mcp.NewTool(
		"agent_status",
		mcp.WithDescription("Get the current status and capabilities of this agent."),
	)
	m.server.AddTool(statusTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		output, _ := json.Marshal(m.info)
		return mcp.NewToolResultText(string(output)), nil
	})
}

// registerResources 注册 Agent 信息资源。
func (m *Server) registerResources() {
	infoRes := mcp.Resource{
		URI:      "agent://info",
		Name:     fmt.Sprintf("%s info", m.info.Name),
		MIMEType: "application/json",
	}
	m.server.AddResource(infoRes, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		data, _ := json.Marshal(m.info)
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      request.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		}, nil
	})
}

// registerPrompts 注册 Agent 角色 Prompt。
func (m *Server) registerPrompts() {
	rolePrompt := mcp.Prompt{
		Name:        "agent_role",
		Description: fmt.Sprintf("Role and capabilities of %s", m.info.Name),
	}
	m.server.AddPrompt(rolePrompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: rolePrompt.Description,
			Messages: []mcp.PromptMessage{
				{
					Role: mcp.RoleUser,
					Content: mcp.TextContent{
						Type: "text",
						Text: fmt.Sprintf("You are %s. %s Skills: %v",
							m.info.Name, m.info.Description, m.info.Skills),
					},
				},
			},
		}, nil
	})
}

// StartSSE 在指定地址启动 SSE 传输服务。
func (m *Server) StartSSE(addr string) error {
	m.sse = mcpServer.NewSSEServer(m.server)
	return m.sse.Start(addr)
}

// Shutdown 优雅关闭服务器。
func (m *Server) Shutdown(ctx context.Context) error {
	if m.sse != nil {
		return m.sse.Shutdown(ctx)
	}
	return nil
}

// ServeHTTP 将 MCP 服务以 HTTP handler 形式暴露，便于接入现有路由。
func (m *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.sse == nil {
		m.sse = mcpServer.NewSSEServer(m.server)
	}
	m.sse.ServeHTTP(w, r)
}

// DelegateTaskInput MCP 工具输入参数结构。
type DelegateTaskInput struct {
	Type        string         `json:"type" jsonschema:"required" jsonschema_description:"Task type"`
	Description string         `json:"description" jsonschema:"required" jsonschema_description:"Task description"`
	Input       string         `json:"input" jsonschema:"required" jsonschema_description:"Task input payload"`
	Context     map[string]any `json:"context" jsonschema_description:"Shared context from the orchestrator"`
}

// parseDelegateInput 解析 MCP 工具调用参数。
func parseDelegateInput(req mcp.CallToolRequest) (*DelegateTaskInput, error) {
	b, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal arguments: %w", err)
	}
	var input DelegateTaskInput
	if err := json.Unmarshal(b, &input); err != nil {
		return nil, fmt.Errorf("unmarshal arguments: %w", err)
	}
	return &input, nil
}
