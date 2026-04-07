package menu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestRootItemsIncludesCustomizeMode(t *testing.T) {
	items := RootItems()
	for _, item := range items {
		if item.ID == "customize-mode" && item.Label == "customize-mode" {
			return
		}
	}
	t.Fatal("expected root items to include customize-mode")
}

func TestCustomizeModeActionRunsTmuxCustomizeMode(t *testing.T) {
	tmpDir := t.TempDir()
	argsFile := filepath.Join(tmpDir, "args.txt")
	tmuxPath := filepath.Join(tmpDir, "tmux")
	script := "#!/bin/sh\nprintf '%s\n' \"$@\" >\"$CODEX_ARGS_FILE\"\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	oldPath := os.Getenv("PATH")
	oldArgsFile := os.Getenv("CODEX_ARGS_FILE")
	if err := os.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	if err := os.Setenv("CODEX_ARGS_FILE", argsFile); err != nil {
		t.Fatalf("set CODEX_ARGS_FILE: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", oldPath)
		_ = os.Setenv("CODEX_ARGS_FILE", oldArgsFile)
	})

	cmd := CustomizeModeAction(Context{SocketPath: "/tmp/test.sock"}, Item{ID: "customize-mode", Label: "customize-mode"})
	msg := cmd()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if got := result.Info; got != "Executed customize-mode" {
		t.Fatalf("unexpected info message %q", got)
	}

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := "-S\n/tmp/test.sock\ncustomize-mode"
	if got != want {
		t.Fatalf("tmux args = %q, want %q", got, want)
	}
}
