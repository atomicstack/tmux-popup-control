package menu

import (
	"fmt"
	"testing"
)

func TestLoadCommandMenuParsesOutput(t *testing.T) {
	restore := listCommandsFn
	t.Cleanup(func() { listCommandsFn = restore })

	listCommandsFn = func(string) (string, error) {
		return "attach-session (attach) [-dErx] [-c working-directory]\nbind-key (bind) [-nr] [-T key-table]\n", nil
	}

	items, err := loadCommandMenu(Context{SocketPath: "/tmp/test.sock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "attach-session" {
		t.Fatalf("expected ID %q, got %q", "attach-session", items[0].ID)
	}
	if items[1].ID != "bind-key" {
		t.Fatalf("expected ID %q, got %q", "bind-key", items[1].ID)
	}
}

func TestLoadCommandMenuError(t *testing.T) {
	restore := listCommandsFn
	t.Cleanup(func() { listCommandsFn = restore })

	listCommandsFn = func(string) (string, error) {
		return "", fmt.Errorf("connection refused")
	}

	items, err := loadCommandMenu(Context{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if items != nil {
		t.Fatalf("expected nil items, got %v", items)
	}
}

func TestLoadCommandMenuEmptyOutput(t *testing.T) {
	restore := listCommandsFn
	t.Cleanup(func() { listCommandsFn = restore })

	listCommandsFn = func(string) (string, error) {
		return "", nil
	}

	items, err := loadCommandMenu(Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestRunCommandReturnsActionResult(t *testing.T) {
	cmd := RunCommand("/tmp/nonexistent.sock", "display-message hello")
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	// We expect an error because the socket doesn't exist, but the key thing
	// is that RunCommand returns an ActionResult.
	if result.Err == nil && result.Info == "" {
		t.Fatal("expected either error or info in ActionResult")
	}
}

func TestRunCommandEmptyReturnsError(t *testing.T) {
	cmd := RunCommand("/tmp/test.sock", "")
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected error for empty command")
	}
}
