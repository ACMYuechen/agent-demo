package tools

import (
	"context"
	"strings"
	"testing"
)

func TestCalculator(t *testing.T) {
	calc, err := NewCalculator()
	if err != nil {
		t.Fatalf("create calculator: %v", err)
	}
	res, err := calc.InvokableRun(context.Background(), `{"expression":"2 + 3 * 4"}`)
	if err != nil {
		t.Fatalf("run calculator: %v", err)
	}
	if !strings.Contains(res, "14") {
		t.Fatalf("expected 14, got %s", res)
	}
}

func TestCurrentTime(t *testing.T) {
	tt, err := NewCurrentTime()
	if err != nil {
		t.Fatalf("create current_time: %v", err)
	}
	res, err := tt.InvokableRun(context.Background(), `{"timezone":"UTC"}`)
	if err != nil {
		t.Fatalf("run current_time: %v", err)
	}
	if !strings.Contains(res, "T") {
		t.Fatalf("expected RFC3339 time, got %s", res)
	}
}

func TestRegistry(t *testing.T) {
	reg := NewRegistry()
	calc, _ := NewCalculator()
	if err := reg.Register(calc); err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(reg.List()) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(reg.List()))
	}
	_, ok := reg.Get("calculator")
	if !ok {
		t.Fatal("expected calculator in registry")
	}
}
