// Package domain 定义 Agent Demo 共享的域实体。
package domain

import (
	"time"

	einoSchema "github.com/cloudwego/eino/schema"
)

// Thread 表示一个会话线程。
type Thread struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message 上下文管理使用的统一消息包装。
type Message struct {
	einoSchema.Message
	ThreadID  string    `json:"thread_id"`
	CreatedAt time.Time `json:"created_at"`
}

// AgentTask 描述派发给 Agent 的工作单元。
type AgentTask struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Description string         `json:"description"`
	Input       string         `json:"input"`
	Context     map[string]any `json:"context"`
	FromAgent   string         `json:"from_agent"`
	ToAgent     string         `json:"to_agent"`
	Status      TaskStatus     `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// TaskStatus Agent 任务生命周期状态。
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskDelegated TaskStatus = "delegated"
)

// AgentInfo 多 Agent 系统中注册的 Agent 元数据。
type AgentInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Skills      []string `json:"skills"`
	Endpoint    string   `json:"endpoint"`
	Description string   `json:"description"`
}

// RetrievalResult 在 Eino 文档基础上补充查询延迟元数据。
type RetrievalResult struct {
	Documents []*einoSchema.Document `json:"documents"`
	Query     string                 `json:"query"`
	LatencyMs int64                  `json:"latency_ms"`
	TopK      int                    `json:"top_k"`
}
