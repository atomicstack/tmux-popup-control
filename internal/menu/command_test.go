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

func TestCommandActionReturnsPromptMsgWithTrailingSpace(t *testing.T) {
	item := Item{ID: "display-message", Label: "display-message"}
	cmd := CommandAction(Context{}, item)
	if cmd == nil {
		t.Fatalf("expected command")
	}
	msg := cmd()
	prompt, ok := msg.(CommandPromptMsg)
	if !ok {
		t.Fatalf("expected CommandPromptMsg, got %T", msg)
	}
	expected := "display-message "
	if prompt.Command != expected {
		t.Fatalf("expected command %q, got %q", expected, prompt.Command)
	}
	if prompt.Label != item.Label {
		t.Fatalf("expected label %q, got %q", item.Label, prompt.Label)
	}
}
