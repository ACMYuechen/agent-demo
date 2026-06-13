// Package rag 实现 RAG 流水线：加载、切分、嵌入、索引、检索。
package rag

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/document"
	einoSchema "github.com/cloudwego/eino/schema"
)

// FileLoader 从本地文件加载文档。
type FileLoader struct {
	SourceURI string // 源 URI，会写入文档元数据
}

// NewFileLoader 创建文件加载器。
func NewFileLoader() *FileLoader {
	return &FileLoader{}
}

// Load 读取 src.URI 指定文件并返回单个文档。
func (l *FileLoader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*einoSchema.Document, error) {
	uri := src.URI
	if uri == "" {
		return nil, fmt.Errorf("source URI is empty")
	}
	data, err := os.ReadFile(uri)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", uri, err)
	}
	return []*einoSchema.Document{{
		ID:      uri,
		Content: string(data),
		MetaData: map[string]any{
			"source": uri,
		},
	}}, nil
}

// StringLoader 从内存字符串加载文档。
type StringLoader struct{}

// NewStringLoader 创建字符串加载器。
func NewStringLoader() *StringLoader {
	return &StringLoader{}
}

// Load 将源 URI 作为文档标识与内容。
func (l *StringLoader) Load(ctx context.Context, src document.Source, opts ...document.LoaderOption) ([]*einoSchema.Document, error) {
	return []*einoSchema.Document{{
		ID:       src.URI,
		Content:  src.URI,
		MetaData: map[string]any{"source": "string"},
	}}, nil
}

// RecursiveSplitter 按段落切分，再按固定大小与重叠量二次切分。
type RecursiveSplitter struct {
	ChunkSize    int      // 单块最大字符数
	ChunkOverlap int      // 块间重叠字符数
	Separators   []string // 分隔符优先级（当前实现主要按段落）
}

// NewRecursiveSplitter 创建带默认参数的切分器。
func NewRecursiveSplitter() *RecursiveSplitter {
	return &RecursiveSplitter{
		ChunkSize:    800,
		ChunkOverlap: 100,
		Separators:   []string{"\n\n", "\n", ". ", " ", ""},
	}
}

// Transform 将输入文档切分为小块。
func (s *RecursiveSplitter) Transform(ctx context.Context, src []*einoSchema.Document, opts ...document.TransformerOption) ([]*einoSchema.Document, error) {
	if s.ChunkSize <= 0 {
		s.ChunkSize = 800
	}
	if s.ChunkOverlap < 0 {
		s.ChunkOverlap = 0
	}

	out := make([]*einoSchema.Document, 0)
	for _, doc := range src {
		chunks := s.splitText(doc.Content)
		for i, chunk := range chunks {
			meta := make(map[string]any)
			for k, v := range doc.MetaData {
				meta[k] = v
			}
			meta["chunk_index"] = i
			meta["chunk_total"] = len(chunks)
			out = append(out, &einoSchema.Document{
				ID:       fmt.Sprintf("%s_chunk_%d", doc.ID, i),
				Content:  chunk,
				MetaData: meta,
			})
		}
	}
	return out, nil
}

// splitText 先按段落切分，过大时按固定大小二次切分。
func (s *RecursiveSplitter) splitText(text string) []string {
	parts := strings.Split(text, "\n\n")
	if len(parts) == 1 && len(text) <= s.ChunkSize {
		return []string{text}
	}

	var chunks []string
	var current strings.Builder
	for _, part := range parts {
		if current.Len()+len(part)+2 > s.ChunkSize && current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			overlap := s.lastN(current.String(), s.ChunkOverlap)
			current.Reset()
			current.WriteString(overlap)
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(part)
	}
	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	// 若段落块仍过大，则按固定大小再次切分。
	final := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if len(chunk) > s.ChunkSize {
			final = append(final, s.splitFixed(chunk)...)
		} else {
			final = append(final, chunk)
		}
	}
	return final
}

// splitFixed 按固定大小与重叠量切分文本。
func (s *RecursiveSplitter) splitFixed(text string) []string {
	var chunks []string
	for i := 0; i < len(text); i += s.ChunkSize - s.ChunkOverlap {
		end := i + s.ChunkSize
		if end > len(text) {
			end = len(text)
		}
		chunks = append(chunks, strings.TrimSpace(text[i:end]))
		if end == len(text) {
			break
		}
	}
	return chunks
}

// lastN 取文本最后 n 个字符。
func (s *RecursiveSplitter) lastN(text string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(text) <= n {
		return text
	}
	return text[len(text)-n:]
}
