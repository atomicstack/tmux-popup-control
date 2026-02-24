package menu

import (
	"fmt"
	"testing"
)

func TestLoadKeybindingMenuParsesOutput(t *testing.T) {
	restore := listKeysFn
	t.Cleanup(func() { listKeysFn = restore })

	listKeysFn = func(string) (string, error) {
		return "bind-key -T prefix d detach-client\nbind-key -T prefix c new-window\n", nil
	}

	items, err := loadKeybindingMenu(Context{SocketPath: "/tmp/test.sock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Label != "bind-key -T prefix d detach-client" {
		t.Fatalf("unexpected label: %q", items[0].Label)
	}
}

func TestLoadKeybindingMenuError(t *testing.T) {
	restore := listKeysFn
	t.Cleanup(func() { listKeysFn = restore })

	listKeysFn = func(string) (string, error) {
		return "", fmt.Errorf("connection refused")
	}

	items, err := loadKeybindingMenu(Context{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if items != nil {
		t.Fatalf("expected nil items, got %v", items)
	}
}

func TestLoadKeybindingMenuEmptyOutput(t *testing.T) {
	restore := listKeysFn
	t.Cleanup(func() { listKeysFn = restore })

	listKeysFn = func(string) (string, error) {
		return "", nil
	}

	items, err := loadKeybindingMenu(Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}
