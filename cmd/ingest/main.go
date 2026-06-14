// Command ingest 将示例知识索引到 Milvus，供 RAG 使用。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/yuechen/agent-demo/internal/config"
	"github.com/yuechen/agent-demo/internal/embedding"
	"github.com/yuechen/agent-demo/internal/milvus"
	"github.com/yuechen/agent-demo/internal/rag"
)

func main() {
	var (
		configPath = flag.String("config", "configs/agent.yaml", "path to config file")
		filePath   = flag.String("file", "data/sample_kb.md", "path to markdown knowledge file")
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

	store, err := milvus.NewStore(ctx, &cfg.Milvus, embedder)
	if err != nil {
		logrus.WithError(err).Fatal("create milvus store")
	}
	defer store.Close(ctx)

	idx, err := milvus.NewIndexer(store)
	if err != nil {
		logrus.WithError(err).Fatal("create indexer")
	}

	pipeline, err := rag.NewPipeline(
		&rag.PipelineConfig{
			Loader:   rag.NewFileLoader(),
			Splitter: rag.NewRecursiveSplitter(),
			Indexer:  idx,
		})
	if err != nil {
		logrus.WithError(err).Fatal("create rag pipeline")
	}

	ids, err := pipeline.IndexSource(ctx, *filePath)
	if err != nil {
		logrus.WithError(err).Fatal("index source")
	}

	fmt.Printf("Indexed %d chunks from %s\n", len(ids), *filePath)

	// 验证检索效果。
	ret, err := milvus.NewRetriever(store)
	if err != nil {
		logrus.WithError(err).Fatal("create retriever")
	}
	docs, err := ret.Retrieve(ctx, "What is Eino?")
	if err != nil {
		logrus.WithError(err).Fatal("retrieve")
	}
	fmt.Printf("Retrieved %d documents\n", len(docs))
	for i, doc := range docs {
		fmt.Printf("[%d] %s: %.100s...\n", i+1, doc.ID, doc.Content)
	}

	os.Exit(0)
}
