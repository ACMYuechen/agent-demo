package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yuechen/agent-demo/internal/domain"
)

// Client 封装用于调用远端 Agent 的 MCP 客户端。
type Client struct {
	client    *client.Client
	agentInfo *domain.AgentInfo
}

// NewSSEClient 通过 SSE 连接远端 Agent，完成初始化握手。
func NewSSEClient(ctx context.Context, endpoint string) (*Client, error) {
	c, err := client.NewSSEMCPClient(endpoint)
	if err != nil {
		return nil, fmt.Errorf("create sse client: %w", err)
	}
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("start client: %w", err)
	}

	// MCP 初始化请求。
	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := c.Initialize(initCtx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			ClientInfo: mcp.Implementation{
				Name:    "agent-demo-client",
				Version: "1.0.0",
			},
		},
	}); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("initialize client: %w", err)
	}

	return &Client{client: c}, nil
}

// Close 关闭客户端连接。
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// FetchAgentInfo 读取远端 Agent 信息资源。
func (c *Client) FetchAgentInfo(ctx context.Context) (*domain.AgentInfo, error) {
	res, err := c.client.ReadResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "agent://info"},
	})
	if err != nil {
		return nil, fmt.Errorf("read resource: %w", err)
	}
	if len(res.Contents) == 0 {
		return nil, fmt.Errorf("empty resource response")
	}
	text, ok := res.Contents[0].(mcp.TextResourceContents)
	if !ok {
		return nil, fmt.Errorf("unexpected resource content type")
	}
	var info domain.AgentInfo
	if err := json.Unmarshal([]byte(text.Text), &info); err != nil {
		return nil, fmt.Errorf("unmarshal agent info: %w", err)
	}
	c.agentInfo = &info
	return &info, nil
}

// DelegateTask 向远端 Agent 发送任务并返回结果载荷。
func (c *Client) DelegateTask(ctx context.Context, task *domain.AgentTask) (map[string]any, error) {
	result, err := c.client.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "delegate_task",
			Arguments: map[string]any{
				"type":        task.Type,
				"description": task.Description,
				"input":       task.Input,
				"context":     task.Context,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("call delegate_task: %w", err)
	}
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty tool result")
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		return nil, fmt.Errorf("unexpected tool result content type")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		return map[string]any{"raw": text.Text}, nil
	}
	return payload, nil
}

// AgentInfo 返回缓存的 Agent 信息。
func (c *Client) AgentInfo() *domain.AgentInfo {
	return c.agentInfo
}
