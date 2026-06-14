// Command single-agent 运行带 RAG 和工具调用的交互式单 Agent Demo。
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/sirupsen/logrus"
	"github.com/yuechen/agent-demo/internal/agent"
	"github.com/yuechen/agent-demo/internal/config"
	"github.com/yuechen/agent-demo/internal/contextmgr"
	"github.com/yuechen/agent-demo/internal/embedding"
	llmModel "github.com/yuechen/agent-demo/internal/model"
	"github.com/yuechen/agent-demo/internal/milvus"
	"github.com/yuechen/agent-demo/internal/tools"
)

func main() {
	var (
		configPath = flag.String("config", "configs/agent.yaml", "path to config file")
		threadID   = flag.String("thread", "", "conversation thread id")
	)
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logrus.WithError(err).Fatal("load config")
	}
	if level, err := logrus.ParseLevel(cfg.LogLevel); err == nil {
		logrus.SetLevel(level)
	}

	ctx := context.Background()
	embedder, err := embedding.NewOpenAIEmbedder(ctx, &cfg.OpenAI)
	if err != nil {
		logrus.WithError(err).Fatal("create embedder")
	}

	// 若启用 RAG，初始化 Milvus 检索器。
	var retriever *milvus.Retriever
	if cfg.Agent.EnableRAG {
		store, err := milvus.NewStore(ctx, &cfg.Milvus, embedder)
		if err != nil {
			logrus.WithError(err).Fatal("create milvus store")
		}
		defer store.Close(ctx)
		retriever, err = milvus.NewRetriever(store)
		if err != nil {
			logrus.WithError(err).Fatal("create retriever")
		}
	}

	chatModel, err := llmModel.NewOpenAIChatModel(ctx, &cfg.OpenAI)
	if err != nil {
		logrus.WithError(err).Fatal("create chat model")
	}

	// 若启用工具，加载默认工具集。
	var toolList []tool.InvokableTool
	if cfg.Agent.EnableTools {
		toolList, err = tools.DefaultTools()
		if err != nil {
			logrus.WithError(err).Fatal("create tools")
		}
	}

	contextMgr := contextmgr.NewInMemoryManager()
	svc, err := agent.NewService(
		&agent.ServiceConfig{
			Config:     &cfg.Agent,
			ChatModel:  chatModel,
			ContextMgr: contextMgr,
			Tools:      toolList,
			Retriever:  retriever,
		})
	if err != nil {
		logrus.WithError(err).Fatal("create agent service")
	}

	// 未指定线程 ID 时自动创建。
	if *threadID == "" {
		thread, err := contextMgr.CreateThread(ctx, "single-agent-demo")
		if err != nil {
			logrus.WithError(err).Fatal("create thread")
		}
		*threadID = thread.ID
	}

	fmt.Printf("Single Agent Demo (thread=%s)\n", *threadID)
	fmt.Println("Type your message and press Enter. Use '/quit' to exit.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\nYou: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/quit" {
			break
		}

		resp, err := svc.Run(ctx, *threadID, input)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Agent: %s\n", resp.Content)
	}
}
