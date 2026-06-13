# Agent Demo

基于 [Eino](https://github.com/cloudwego/eino) 框架构建的 Go 语言 Agent 项目，演示以下能力：

1. **基础 Agent 架构** — RAG 知识库检索、工具调用、会话上下文管理。
2. **向量检索优化** — 基于 Milvus 向量数据库，支持 HNSW/IVF_FLAT 索引配置与查询参数调优。
3. **多 Agent 协作** — 基于 MCP（Model Context Protocol）协议实现 Agent 间上下文传递与任务调度。

## 项目结构

```
.
├── cmd/
│   ├── ingest/          # 将知识文档索引到 Milvus
│   ├── single-agent/    # 交互式单 Agent 演示
│   └── multi-agent/     # 基于 MCP 的多 Agent 协作演示
├── internal/
│   ├── agent/           # ReAct 风格单 Agent 编排
│   ├── config/          # 配置管理
│   ├── contextmgr/      # 会话记忆/线程上下文
│   ├── domain/          # 共享领域实体
│   ├── embedding/       # Embedding 模型构造
│   ├── milvus/          # 优化的 Milvus 索引器与检索器
│   ├── model/           # 对话模型构造
│   ├── rag/             # 文档加载/切分/RAG 流水线
│   ├── tools/           # 内置工具
│   ├── mcp/             # MCP 服务端/客户端封装
│   └── orchestrator/    # Agent 注册表与任务调度器
├── deployments/         # Milvus Docker Compose
├── configs/             # 示例配置
└── data/                # 示例知识库
```

## 环境准备

- Go 1.25+
- OpenAI API Key
- Docker 与 Docker Compose（运行 Milvus）

## 快速开始

### 1. 启动 Milvus

```bash
cd deployments
docker compose up -d
```

Milvus 将监听 `localhost:19530`。

### 2. 配置

复制 `configs/agent.yaml` 并填写 OpenAI API Key，或直接导出环境变量：

```bash
export OPENAI_API_KEY=sk-...
```

如需调整模型、Milvus 地址或索引参数，可编辑 `configs/agent.yaml`。

### 3. 索引知识

```bash
go run ./cmd/ingest --config configs/agent.yaml --file data/sample_kb.md
```

该命令会读取 `data/sample_kb.md`，切分文本、调用 OpenAI 生成 Embedding，并写入 Milvus（默认使用 HNSW 索引）。

### 4. 运行单 Agent 演示

```bash
go run ./cmd/single-agent --config configs/agent.yaml
```

输入示例：

- `什么是 Eino？`
- `Milvus 支持哪些索引类型？`
- `123 * 456 等于多少？`（调用计算器工具）
- `Asia/Shanghai 现在几点？`（调用当前时间工具）

输入 `/quit` 退出。

### 5. 运行多 Agent 演示

```bash
go run ./cmd/multi-agent --config configs/agent.yaml --port 8080
```

该命令会启动一个 Worker Agent 的 MCP 服务，并通过编排器向其派发任务，同时演示并行派发。

## 配置说明

`configs/agent.yaml` 中关键的 Milvus 优化参数：

| 配置项 | 说明 |
|--------|------|
| `milvus.index_type` | 索引类型：`HNSW`、`IVF_FLAT`、`FLAT` |
| `milvus.hnsw_m` | HNSW 每层最大连接数 |
| `milvus.hnsw_ef_construction` | HNSW 构建质量 |
| `milvus.ivf_flat_nlist` | IVF_FLAT 聚类桶数 |
| `milvus.search_ef` | HNSW 搜索时 ef，越大召回越高、延迟越大 |
| `milvus.search_nprobe` | IVF_FLAT 搜索探测桶数 |
| `milvus.metric_type` | 距离度量：`COSINE`、`L2`、`IP` |
| `milvus.top_k` | 默认召回文档数 |

通过调整 `search_ef` / `search_nprobe` 可在延迟与召回率之间取舍。

## 架构要点

### 单 Agent

- `agent.Service` 通过 `contextmgr.Manager` 维护会话历史。
- 每一轮用户输入可选地触发 Milvus 检索，将相关上下文注入模型提示。
- 模型接收「系统提示 + 历史 + 检索上下文 + 用户输入」。
- 若模型输出 `ToolCalls`，`compose.ToolsNode` 执行工具调用，并继续循环（最多 `max_tool_steps` 步）。

### RAG 流水线

- `rag.FileLoader` 读取 Markdown 文件。
- `rag.RecursiveSplitter` 先按段落切分，再按固定大小与重叠量二次切分。
- `milvus.Indexer` 调用 OpenAI 生成 Embedding 并写入 Milvus。
- `milvus.Retriever` 对查询生成 Embedding，使用配置的索引与搜索参数执行 ANN 检索。

### 多 Agent 协作（MCP）

- 每个 Agent 暴露一个 MCP 服务器，包含：
  - `delegate_task` 工具：接收任务描述与上下文
  - `agent://info` 资源：描述 Agent 能力
  - `agent_role` Prompt：描述 Agent 角色
- `orchestrator.Registry` 发现并连接远端 Agent。
- `orchestrator.Scheduler` 按技能/角色匹配选择最佳 Agent，支持串行与并行派发，并在子任务间共享上下文。

## 主要依赖

- `github.com/cloudwego/eino` — Eino 核心编排框架
- `github.com/cloudwego/eino-ext/components/model/openai` — OpenAI 对话模型
- `github.com/cloudwego/eino-ext/components/embedding/openai` — OpenAI Embedding
- `github.com/milvus-io/milvus/client/v2` — Milvus Go SDK v2
- `github.com/mark3labs/mcp-go` — MCP 协议实现
- `github.com/spf13/viper` — 配置管理
- `github.com/sirupsen/logrus` — 日志

## 许可证

MIT
