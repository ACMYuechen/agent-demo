package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yuechen/agent-demo/internal/domain"
)

// Scheduler 决定由哪个 Agent 处理任务并负责派发。
type Scheduler struct {
	registry *Registry
}

// NewScheduler 创建绑定到注册表的调度器。
func NewScheduler(registry *Registry) *Scheduler {
	return &Scheduler{registry: registry}
}

// Dispatch 将任务派发给最匹配的 Agent 并返回结果。
func (s *Scheduler) Dispatch(ctx context.Context, task *domain.AgentTask) (*domain.AgentTask, error) {
	if task.ID == "" {
		task.ID = uuid.New().String() // 自动生成任务 ID
	}
	task.Status = domain.TaskRunning
	task.CreatedAt = time.Now()

	// 未指定目标 Agent 时自动选择。
	agentID := task.ToAgent
	if agentID == "" {
		selected := s.selectAgent(task)
		if selected == "" {
			return nil, fmt.Errorf("no suitable agent found for task type %s", task.Type)
		}
		agentID = selected
	}

	task.ToAgent = agentID
	client, ok := s.registry.Client(agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}

	result, err := client.DelegateTask(ctx, task)
	if err != nil {
		task.Status = domain.TaskFailed
		task.UpdatedAt = time.Now()
		return task, fmt.Errorf("delegate to agent %s: %w", agentID, err)
	}

	task.Status = domain.TaskCompleted
	task.UpdatedAt = time.Now()
	if result != nil {
		task.Context = mergeContext(task.Context, result)
	}
	return task, nil
}

// DispatchParallel 并发将任务派发给多个 Agent。
func (s *Scheduler) DispatchParallel(ctx context.Context, task *domain.AgentTask, agentIDs []string) ([]*domain.AgentTask, error) {
	if len(agentIDs) == 0 {
		return nil, fmt.Errorf("no agents specified")
	}

	var wg sync.WaitGroup
	results := make([]*domain.AgentTask, len(agentIDs))
	errs := make([]error, len(agentIDs))

	for i, id := range agentIDs {
		wg.Add(1)
		go func(idx int, agentID string) {
			defer wg.Done()
			t := *task
			t.ToAgent = agentID
			res, err := s.Dispatch(ctx, &t)
			results[idx] = res
			errs[idx] = err
		}(i, id)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return results, fmt.Errorf("one or more parallel dispatches failed: %w", err)
		}
	}
	return results, nil
}

// RoutePlan 将复杂任务拆分为子任务并顺序派发，共享上下文。
func (s *Scheduler) RoutePlan(ctx context.Context, plan []*domain.AgentTask) ([]*domain.AgentTask, error) {
	results := make([]*domain.AgentTask, 0, len(plan))
	sharedContext := make(map[string]any)

	for _, subtask := range plan {
		if subtask.Context == nil {
			subtask.Context = make(map[string]any)
		}
		for k, v := range sharedContext {
			subtask.Context[k] = v
		}

		res, err := s.Dispatch(ctx, subtask)
		if err != nil {
			return results, err
		}
		results = append(results, res)
		if res.Context != nil {
			for k, v := range res.Context {
				sharedContext[k] = v
			}
		}
	}
	return results, nil
}

// selectAgent 根据技能/角色匹配选择最合适的 Agent。
func (s *Scheduler) selectAgent(task *domain.AgentTask) string {
	agents := s.registry.List()
	var best *domain.AgentInfo
	bestScore := 0

	for _, info := range agents {
		score := matchScore(info, task)
		if score > bestScore {
			bestScore = score
			best = info
		}
	}
	if best == nil {
		return ""
	}
	return best.ID
}

// matchScore 计算 Agent 与任务的匹配分数。
func matchScore(info *domain.AgentInfo, task *domain.AgentTask) int {
	score := 0
	taskLower := strings.ToLower(task.Type + " " + task.Description)
	for _, skill := range info.Skills {
		if strings.Contains(taskLower, strings.ToLower(skill)) {
			score += 2
		}
	}
	if strings.Contains(strings.ToLower(info.Role), strings.ToLower(task.Type)) {
		score += 3
	}
	if strings.Contains(strings.ToLower(info.Description), strings.ToLower(task.Type)) {
		score += 1
	}
	return score
}

// mergeContext 合并两层上下文。
func mergeContext(base, overlay map[string]any) map[string]any {
	if base == nil {
		base = make(map[string]any)
	}
	for k, v := range overlay {
		base[k] = v
	}
	return base
}
