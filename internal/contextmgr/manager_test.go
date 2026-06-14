package contextmgr

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestInMemoryManager(t *testing.T) {
	ctx := context.Background()
	mgr := NewInMemoryManager()

	thread, err := mgr.CreateThread(ctx, "test")
	if err != nil {
		t.Fatalf("create thread: %v", err)
	}
	if thread.ID == "" {
		t.Fatal("expected thread id")
	}

	if err := mgr.AddMessage(ctx, thread.ID, FromEinoMessage(thread.ID, &schema.Message{
		Role:    schema.User,
		Content: "hello",
	})); err != nil {
		t.Fatalf("add message: %v", err)
	}

	msgs, err := mgr.GetMessages(ctx, thread.ID, 10)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	if err := mgr.SetState(ctx, thread.ID, "key", "value"); err != nil {
		t.Fatalf("set state: %v", err)
	}
	val, ok, err := mgr.GetState(ctx, thread.ID, "key")
	if err != nil || !ok || val != "value" {
		t.Fatalf("expected state value, got %v %v %v", val, ok, err)
	}
}
