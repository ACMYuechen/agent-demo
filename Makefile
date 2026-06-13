.PHONY: all build test lint ingest single multi milvus-up milvus-down clean

# 默认目标：构建全部
all: build

# 编译所有包
build:
	go build ./...

# 运行单元测试
test:
	go test ./...

# 运行代码检查（需安装 golangci-lint）
lint:
	golangci-lint run ./...

# 索引示例知识库到 Milvus
ingest:
	go run ./cmd/ingest --config configs/agent.yaml --file data/sample_kb.md

# 运行交互式单 Agent 演示
single:
	go run ./cmd/single-agent --config configs/agent.yaml

# 运行 MCP 多 Agent 协作演示
multi:
	go run ./cmd/multi-agent --config configs/agent.yaml --port 8080

# 启动 Milvus 服务（Docker Compose）
milvus-up:
	cd deployments && docker compose up -d

# 停止 Milvus 服务
milvus-down:
	cd deployments && docker compose down

# 清理：停止并删除 Milvus 数据卷，同时清理 Go 测试缓存
clean:
	cd deployments && docker compose down -v
	go clean -testcache
