// Package contextmgr 提供 Agent 会话上下文与记忆管理。
package contextmgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/yuechen/agent-demo/internal/domain"
)

// Manager 定义线程级上下文操作接口。
type Manager interface {
	CreateThread(ctx context.Context, title string) (*domain.Thread, error)
	GetThread(ctx context.Context, threadID string) (*domain.Thread, error)
	ListThreads(ctx context.Context) ([]*domain.Thread, error)
	DeleteThread(ctx context.Context, threadID string) error
	AddMessage(ctx context.Context, threadID string, msg *domain.Message) error
	GetMessages(ctx context.Context, threadID string, limit int) ([]*domain.Message, error)
	GetMessagesByRole(ctx context.Context, threadID string, role schema.RoleType, limit int) ([]*domain.Message, error)
	ClearMessages(ctx context.Context, threadID string) error
	SetState(ctx context.Context, threadID string, key string, value any) error
	GetState(ctx context.Context, threadID string, key string) (any, bool, error)
	DeleteState(ctx context.Context, threadID string, key string) error
}

// InMemoryManager 线程安全的内存上下文管理实现。
type InMemoryManager struct {
	mu      sync.RWMutex
	threads map[string]*domain.Thread
	msgs    map[string][]*domain.Message
	state   map[string]map[string]any
}

// NewInMemoryManager creates a new in-memory context manager.
func NewInMemoryManager() *InMemoryManager {
	return &InMemoryManager{
		threads: make(map[string]*domain.Thread),
		msgs:    make(map[string][]*domain.Message),
		state:   make(map[string]map[string]any),
	}
}

// CreateThread initializes a new conversation thread.
func (m *InMemoryManager) CreateThread(ctx context.Context, title string) (*domain.Thread, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	thread := &domain.Thread{
		ID:        uuid.New().String(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.threads[thread.ID] = thread
	m.msgs[thread.ID] = make([]*domain.Message, 0)
	m.state[thread.ID] = make(map[string]any)
	return thread, nil
}

// GetThread returns a thread by ID.
func (m *InMemoryManager) GetThread(ctx context.Context, threadID string) (*domain.Thread, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	thread, ok := m.threads[threadID]
	if !ok {
		return nil, fmt.Errorf("thread %s not found", threadID)
	}
	return thread, nil
}

// ListThreads returns all stored threads.
func (m *InMemoryManager) ListThreads(ctx context.Context) ([]*domain.Thread, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	threads := make([]*domain.Thread, 0, len(m.threads))
	for _, t := range m.threads {
		threads = append(threads, t)
	}
	return threads, nil
}

// DeleteThread removes a thread and its messages/state.
func (m *InMemoryManager) DeleteThread(ctx context.Context, threadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.threads[threadID]; !ok {
		return fmt.Errorf("thread %s not found", threadID)
	}
	delete(m.threads, threadID)
	delete(m.msgs, threadID)
	delete(m.state, threadID)
	return nil
}

// AddMessage appends a message to a thread and updates the thread timestamp.
func (m *InMemoryManager) AddMessage(ctx context.Context, threadID string, msg *domain.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	thread, ok := m.threads[threadID]
	if !ok {
		return fmt.Errorf("thread %s not found", threadID)
	}

	msg.ThreadID = threadID
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}
	m.msgs[threadID] = append(m.msgs[threadID], msg)
	thread.UpdatedAt = msg.CreatedAt
	return nil
}

// GetMessages returns the most recent messages up to limit.
func (m *InMemoryManager) GetMessages(ctx context.Context, threadID string, limit int) ([]*domain.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msgs, ok := m.msgs[threadID]
	if !ok {
		return nil, fmt.Errorf("thread %s not found", threadID)
	}
	if limit <= 0 || limit > len(msgs) {
		limit = len(msgs)
	}
	start := len(msgs) - limit
	out := make([]*domain.Message, limit)
	copy(out, msgs[start:])
	return out, nil
}

// GetMessagesByRole filters messages by role before applying the limit.
func (m *InMemoryManager) GetMessagesByRole(ctx context.Context, threadID string, role schema.RoleType, limit int) ([]*domain.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msgs, ok := m.msgs[threadID]
	if !ok {
		return nil, fmt.Errorf("thread %s not found", threadID)
	}

	filtered := make([]*domain.Message, 0)
	for _, msg := range msgs {
		if msg.Role == role {
			filtered = append(filtered, msg)
		}
	}
	if limit <= 0 || limit > len(filtered) {
		limit = len(filtered)
	}
	start := len(filtered) - limit
	return filtered[start:], nil
}

// ClearMessages removes all messages from a thread but keeps thread metadata and state.
func (m *InMemoryManager) ClearMessages(ctx context.Context, threadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.threads[threadID]; !ok {
		return fmt.Errorf("thread %s not found", threadID)
	}
	m.msgs[threadID] = make([]*domain.Message, 0)
	return nil
}

// SetState stores a key-value pair in thread state.
func (m *InMemoryManager) SetState(ctx context.Context, threadID string, key string, value any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.threads[threadID]; !ok {
		return fmt.Errorf("thread %s not found", threadID)
	}
	if m.state[threadID] == nil {
		m.state[threadID] = make(map[string]any)
	}
	m.state[threadID][key] = value
	return nil
}

// GetState retrieves a value from thread state.
func (m *InMemoryManager) GetState(ctx context.Context, threadID string, key string) (any, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.threads[threadID]; !ok {
		return nil, false, fmt.Errorf("thread %s not found", threadID)
	}
	val, ok := m.state[threadID][key]
	return val, ok, nil
}

// DeleteState removes a key from thread state.
func (m *InMemoryManager) DeleteState(ctx context.Context, threadID string, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.threads[threadID]; !ok {
		return fmt.Errorf("thread %s not found", threadID)
	}
	delete(m.state[threadID], key)
	return nil
}

// ToEinoMessages converts internal messages to a slice of eino schema messages.
func ToEinoMessages(msgs []*domain.Message) []*schema.Message {
	out := make([]*schema.Message, len(msgs))
	for i, m := range msgs {
		out[i] = &m.Message
	}
	return out
}

// FromEinoMessage builds an internal message from an eino schema message.
func FromEinoMessage(threadID string, msg *schema.Message) *domain.Message {
	return &domain.Message{
		Message:   *msg,
		ThreadID:  threadID,
		CreatedAt: time.Now(),
	}
}
