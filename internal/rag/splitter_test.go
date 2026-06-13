package rag

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/document"
	einoSchema "github.com/cloudwego/eino/schema"
)

func TestRecursiveSplitter(t *testing.T) {
	splitter := NewRecursiveSplitter()
	splitter.ChunkSize = 100
	splitter.ChunkOverlap = 10

	docs := []*einoSchema.Document{{
		ID:      "doc1",
		Content: strings.Repeat("a ", 200),
	}}

	chunks, err := splitter.Transform(context.Background(), docs)
	if err != nil {
		t.Fatalf("transform: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if c.ID == "" || c.Content == "" {
			t.Fatalf("invalid chunk: %+v", c)
		}
	}
}

func TestFileLoader(t *testing.T) {
	loader := NewFileLoader()
	f, err := os.CreateTemp("", "rag_test_*.md")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString("# Hello\n\nThis is a test document."); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	_ = f.Close()

	docs, err := loader.Load(context.Background(), document.Source{URI: f.Name()})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(docs) != 1 || !strings.Contains(docs[0].Content, "test document") {
		t.Fatalf("unexpected load result: %+v", docs)
	}
}
