package ui

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleEscapeKeyFromRootQuits(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "", "")
	cmd := m.handleEscapeKey()
	if cmd == nil {
		t.Fatalf("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHandleEscapeKeyPopsLevelAndClearsSwapState(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "", "")
	parent := m.currentLevel()
	parent.Items = []menu.Item{{ID: "one"}, {ID: "two"}, {ID: "window:swap-target"}}
	parent.Cursor = 1
	parent.LastCursor = 2

	swap := newLevel("window:swap-target", "Swap", []menu.Item{{ID: "a", Label: "A"}}, nil)
	m.stack = append(m.stack, swap)
	m.pendingWindowSwap = &menu.Item{ID: "a", Label: "A"}
	m.errMsg = "previous error"

	cmd := m.handleEscapeKey()
	if cmd != nil {
		t.Fatalf("expected no command when popping a level")
	}
	if len(m.stack) != 1 {
		t.Fatalf("expected stack to shrink to 1, got %d", len(m.stack))
	}
	if parent.Cursor != 2 {
		t.Fatalf("expected parent cursor restored to 2, got %d", parent.Cursor)
	}
	if parent.LastCursor != -1 {
		t.Fatalf("expected parent LastCursor reset, got %d", parent.LastCursor)
	}
	if m.pendingWindowSwap != nil {
		t.Fatalf("expected pending window swap cleared")
	}
	if m.errMsg != "" {
		t.Fatalf("expected error message cleared, got %q", m.errMsg)
	}
}
