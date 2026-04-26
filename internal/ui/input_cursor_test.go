package ui

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	uistate "github.com/atomicstack/tmux-popup-control/internal/ui/state"
)

func TestPromptCursorColumnEmptyFilter(t *testing.T) {
	lvl := uistate.NewLevel("root", "Main Menu", []menu.Item{{ID: "a", Label: "Alpha"}}, nil)
	col, ok := promptCursorColumn(lvl)
	if !ok {
		t.Fatalf("expected ok=true for level with selectable filter")
	}
	// prompt prefix is "» " (2 cells); empty filter places cursor right after.
	if col != 2 {
		t.Fatalf("expected col=2 for empty filter, got %d", col)
	}
}

func TestPromptCursorColumnAdvancesWithText(t *testing.T) {
	lvl := uistate.NewLevel("root", "Main Menu", []menu.Item{{ID: "a", Label: "Alpha"}}, nil)
	lvl.SetFilter("abc", 3)
	col, ok := promptCursorColumn(lvl)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if col != 5 { // 2 (prompt) + 3 (a,b,c)
		t.Fatalf("expected col=5 for 'abc' cursor at end, got %d", col)
	}
}

func TestPromptCursorColumnHandlesWideRune(t *testing.T) {
	lvl := uistate.NewLevel("root", "Main Menu", []menu.Item{{ID: "a", Label: "Alpha"}}, nil)
	// '中' is a 2-cell wide rune. Cursor placed at rune offset 1 (after the char).
	lvl.SetFilter("中", 1)
	col, ok := promptCursorColumn(lvl)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if col != 4 { // 2 (prompt) + 2 (wide rune)
		t.Fatalf("expected col=4 after one CJK char, got %d", col)
	}
}

func TestPromptCursorColumnNilLevel(t *testing.T) {
	col, ok := promptCursorColumn(nil)
	if ok {
		t.Fatalf("expected ok=false for nil level")
	}
	if col != 0 {
		t.Fatalf("expected col=0 for nil level, got %d", col)
	}
}
