// Command multi-agent 演示基于 MCP 的多 Agent 协作。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/yuechen/agent-demo/internal/domain"
	"github.com/yuechen/agent-demo/internal/mcp"
	"github.com/yuechen/agent-demo/internal/orchestrator"
)

func main() {
	var (
		port     = flag.Int("port", 8080, "MCP server port")
		endpoint = flag.String("endpoint", "", "Remote worker MCP endpoint (e.g. http://localhost:8080/sse)")
	)
	flag.Parse()

	logrus.SetLevel(logrus.InfoLevel)

	ctx := context.Background()

	// 定义一个负责研究问题的 Worker Agent。
	workerInfo := &domain.AgentInfo{
		ID:          "researcher",
		Name:        "Research Agent",
		Role:        "research",
		Skills:      []string{"knowledge", "retrieval", "summarization"},
		Endpoint:    fmt.Sprintf("http://localhost:%d/sse", *port),
		Description: "Specializes in retrieving and summarizing knowledge.",
	}

	// 为 Worker Agent 启动 MCP 服务器。
	workerHandler := func(ctx context.Context, task *domain.AgentTask) (*domain.AgentTask, error) {
		logrus.WithFields(logrus.Fields{
			"type": task.Type,
			"desc": task.Description,
		}).Info("worker received task")

		result := *task
		result.Status = domain.TaskCompleted
		result.Context = mergeMaps(task.Context, map[string]any{
			"worker_answer": fmt.Sprintf("Research result for '%s': Eino is CloudWeGo's Go LLM framework; Milvus is a vector database; MCP enables agent context sharing.", task.Input),
		})
		return &result, nil
	}

	server, err := mcp.NewServer(&mcp.ServerConfig{
		AgentInfo: workerInfo,
		Handler:   workerHandler,
	})
	if err != nil {
		logrus.WithError(err).Fatal("create mcp server")
	}

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	go func() {
		logrus.WithField("addr", addr).Info("starting MCP worker server")
		if err := server.StartSSE(addr); err != nil {
			logrus.WithError(err).Fatal("mcp server failed")
		}
	}()

	// 等待服务器启动。
	time.Sleep(500 * time.Millisecond)

	// 创建编排器注册表并发现 Worker。
	registry := orchestrator.NewRegistry()
	workerEndpoint := *endpoint
	if workerEndpoint == "" {
		workerEndpoint = fmt.Sprintf("http://localhost:%d/sse", *port)
	}
	if err := registry.Discover(ctx, "researcher", workerEndpoint); err != nil {
		logrus.WithError(err).Fatal("discover worker")
	}
	defer registry.Close()

	scheduler := orchestrator.NewScheduler(registry)

	// 向 Worker 派发任务。
	task := &domain.AgentTask{
		Type:        "research",
		Description: "Summarize the agent demo stack",
		Input:       "Eino + Milvus + MCP",
		Context:     map[string]any{"requester": "coordinator"},
	}

	fmt.Printf("Dispatching task to agent %s...\n", task.ToAgent)
	result, err := scheduler.Dispatch(ctx, task)
	if err != nil {
		logrus.WithError(err).Fatal("dispatch task")
	}

	fmt.Printf("Task %s completed by %s\n", result.ID, result.ToAgent)
	fmt.Printf("Context: %+v\n", result.Context)

	// 演示并行派发。
	fmt.Println("\nParallel dispatch demo:")
	results, err := scheduler.DispatchParallel(ctx, &domain.AgentTask{
		Type:        "research",
		Description: "Parallel research request",
		Input:       "vector database index optimization",
	}, []string{"researcher"})
	if err != nil {
		logrus.WithError(err).Fatal("parallel dispatch")
	}
	for _, r := range results {
		fmt.Printf("- %s: %s\n", r.ToAgent, r.Status)
	}

	os.Exit(0)
}

// mergeMaps 合并两个 map。
func mergeMaps(a, b map[string]any) map[string]any {
	if a == nil {
		a = make(map[string]any)
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}
