// Package config 集中管理应用配置，支持文件、环境变量与默认值。
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// AppConfig 根配置对象，聚合所有子模块配置。
type AppConfig struct {
	LogLevel string `mapstructure:"log_level"` // 日志级别
	OpenAI   OpenAI `mapstructure:"openai"`    // OpenAI 配置
	Milvus   Milvus `mapstructure:"milvus"`    // Milvus 配置
	Agent    Agent  `mapstructure:"agent"`     // Agent 运行参数
	MCP      MCP    `mapstructure:"mcp"`       // 多 Agent 协调配置
}

// OpenAI 大语言模型与 Embedding 设置。
type OpenAI struct {
	APIKey      string        `mapstructure:"api_key"`       // API 密钥
	BaseURL     string        `mapstructure:"base_url"`      // 自定义 base URL（可选）
	ChatModel   string        `mapstructure:"chat_model"`    // 对话模型
	EmbedModel  string        `mapstructure:"embed_model"`   // 嵌入模型
	Timeout     time.Duration `mapstructure:"timeout"`       // 请求超时
	Temperature float32       `mapstructure:"temperature"`   // 采样温度
	MaxTokens   int           `mapstructure:"max_tokens"`    // 最大生成 token 数
}

// Milvus 向量数据库设置，包含索引与查询优化参数。
type Milvus struct {
	Endpoint           string        `mapstructure:"endpoint"`             // Milvus 地址
	Token              string        `mapstructure:"token"`                // 认证 Token
	DBName             string        `mapstructure:"db_name"`              // 数据库名
	Collection         string        `mapstructure:"collection"`           // 集合名
	VectorDim          int           `mapstructure:"vector_dim"`           // 向量维度
	MetricType         string        `mapstructure:"metric_type"`          // 距离度量类型
	IndexType          string        `mapstructure:"index_type"`           // 索引类型
	HNSWM              int           `mapstructure:"hnsw_m"`               // HNSW 每层最大连接数
	HNSWEfConstruction int           `mapstructure:"hnsw_ef_construction"` // HNSW 构建质量
	IVFFLATNList       int           `mapstructure:"ivf_flat_nlist"`       // IVF_FLAT 聚类数
	TopK               int           `mapstructure:"top_k"`                // 默认召回数量
	ScoreThreshold     float64       `mapstructure:"score_threshold"`      // 分数阈值
	SearchEf           int           `mapstructure:"search_ef"`            // HNSW 搜索参数 ef
	SearchNProbe       int           `mapstructure:"search_nprobe"`        // IVF_FLAT 搜索探测桶数
	MaxTextLength      int           `mapstructure:"max_text_length"`      // 文本字段最大长度
	ConnectTimeout     time.Duration `mapstructure:"connect_timeout"`      // 连接超时
}

// Agent Agent 运行时参数。
type Agent struct {
	SystemPrompt    string        `mapstructure:"system_prompt"`   // 系统提示词
	MaxHistory      int           `mapstructure:"max_history"`     // 最大历史消息数
	MaxToolSteps    int           `mapstructure:"max_tool_steps"`  // 单轮最大工具调用步数
	EnableRAG       bool          `mapstructure:"enable_rag"`      // 是否启用 RAG
	EnableTools     bool          `mapstructure:"enable_tools"`    // 是否启用工具
	ResponseTimeout time.Duration `mapstructure:"response_timeout"` // 响应超时
}

// MCP 多 Agent 协调设置。
type MCP struct {
	Enabled     bool     `mapstructure:"enabled"`      // 是否启用 MCP
	Host        string   `mapstructure:"host"`         // 监听地址
	Port        int      `mapstructure:"port"`         // 监听端口
	Transports  []string `mapstructure:"transports"`   // 传输方式
	RegistryTTL int      `mapstructure:"registry_ttl"` // 注册中心 TTL
}

// Load 从配置文件、环境变量和默认值读取配置。
func Load(path string) (*AppConfig, error) {
	v := viper.New()
	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	// 环境变量前缀为 AGENT_，如 AGENT_OPENAI_API_KEY。
	v.SetEnvPrefix("AGENT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置配置默认值。
func setDefaults(v *viper.Viper) {
	v.SetDefault("log_level", "info")

	v.SetDefault("openai.chat_model", "gpt-4o-mini")
	v.SetDefault("openai.embed_model", "text-embedding-3-small")
	v.SetDefault("openai.timeout", "60s")
	v.SetDefault("openai.temperature", 0.3)
	v.SetDefault("openai.max_tokens", 2048)

	v.SetDefault("milvus.endpoint", "localhost:19530")
	v.SetDefault("milvus.db_name", "default")
	v.SetDefault("milvus.collection", "agent_kb")
	v.SetDefault("milvus.vector_dim", 1536)
	v.SetDefault("milvus.metric_type", "COSINE")
	v.SetDefault("milvus.index_type", "HNSW")
	v.SetDefault("milvus.hnsw_m", 16)
	v.SetDefault("milvus.hnsw_ef_construction", 200)
	v.SetDefault("milvus.ivf_flat_nlist", 128)
	v.SetDefault("milvus.top_k", 5)
	v.SetDefault("milvus.score_threshold", 0.0)
	v.SetDefault("milvus.search_ef", 64)
	v.SetDefault("milvus.search_nprobe", 16)
	v.SetDefault("milvus.max_text_length", 8192)
	v.SetDefault("milvus.connect_timeout", "10s")

	v.SetDefault("agent.system_prompt", defaultSystemPrompt)
	v.SetDefault("agent.max_history", 20)
	v.SetDefault("agent.max_tool_steps", 5)
	v.SetDefault("agent.enable_rag", true)
	v.SetDefault("agent.enable_tools", true)
	v.SetDefault("agent.response_timeout", "120s")

	v.SetDefault("mcp.enabled", false)
	v.SetDefault("mcp.host", "0.0.0.0")
	v.SetDefault("mcp.port", 8080)
	v.SetDefault("mcp.transports", []string{"sse"})
	v.SetDefault("mcp.registry_ttl", 60)
}

// validate 校验必填配置。
func (c *AppConfig) validate() error {
	if c.OpenAI.APIKey == "" {
		return fmt.Errorf("openai.api_key is required")
	}
	if c.Milvus.VectorDim <= 0 {
		return fmt.Errorf("milvus.vector_dim must be positive")
	}
	return nil
}

const defaultSystemPrompt = `You are a helpful assistant powered by Eino and Milvus.
Answer user questions using the provided context when available.
If tools are available and the question requires computation or real-time data, call the appropriate tool.
Be concise and cite sources when using retrieved knowledge.`
