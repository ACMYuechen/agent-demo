// Package orchestrator 实现多 Agent 任务调度与上下文传递。
package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/yuechen/agent-demo/internal/domain"
	"github.com/yuechen/agent-demo/internal/mcp"
)

// Registry 维护可用 Agent 目录及其 MCP 连接。
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*domain.AgentInfo // Agent 元信息
	clients map[string]*mcp.Client      // Agent ID 对应的 MCP 客户端
}

// NewRegistry 创建空的 Agent 注册表。
func NewRegistry() *Registry {
	return &Registry{
		agents:  make(map[string]*domain.AgentInfo),
		clients: make(map[string]*mcp.Client),
	}
}

// Register 注册本地 Agent 描述。
func (r *Registry) Register(info *domain.AgentInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if info.ID == "" {
		return fmt.Errorf("agent id is required")
	}
	r.agents[info.ID] = info
	return nil
}

// Discover 连接远端 Agent 并注册其信息。
func (r *Registry) Discover(ctx context.Context, agentID, endpoint string) error {
	client, err := mcp.NewSSEClient(ctx, endpoint)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", endpoint, err)
	}
	info, err := client.FetchAgentInfo(ctx)
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("fetch agent info from %s: %w", endpoint, err)
	}
	if agentID != "" {
		info.ID = agentID // 允许外部指定 Agent ID
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[info.ID] = info
	if old, ok := r.clients[info.ID]; ok {
		_ = old.Close() // 关闭旧连接
	}
	r.clients[info.ID] = client
	return nil
}

// Get 按 ID 获取 Agent 信息。
func (r *Registry) Get(id string) (*domain.AgentInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, ok := r.agents[id]
	return info, ok
}

// List 返回所有已注册 Agent。
func (r *Registry) List() []*domain.AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*domain.AgentInfo, 0, len(r.agents))
	for _, info := range r.agents {
		out = append(out, info)
	}
	return out
}

// Client 返回指定 Agent 的 MCP 客户端。
func (r *Registry) Client(id string) (*mcp.Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.clients[id]
	return c, ok
}

// Close 关闭所有管理的 MCP 客户端。
func (r *Registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var firstErr error
	for _, c := range r.clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.clients = make(map[string]*mcp.Client)
	return firstErr
}
