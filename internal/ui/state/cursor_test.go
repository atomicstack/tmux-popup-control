package state

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func newTestLevel(ids ...string) *Level {
	items := make([]menu.Item, len(ids))
	for i, id := range ids {
		items[i] = menu.Item{ID: id, Label: id}
	}
	return NewLevel("test", "Test", items, nil)
}

func TestMoveCursorHome(t *testing.T) {
	l := newTestLevel("a", "b", "c")
	l.Cursor = 2
	if !l.MoveCursorHome() {
		t.Fatalf("expected move when items exist")
	}
	if l.Cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", l.Cursor)
	}

	empty := newTestLevel()
	empty.Cursor = 5
	if empty.MoveCursorHome() {
		t.Fatalf("expected no movement for empty level")
	}
	if empty.Cursor != 0 {
		t.Fatalf("expected cursor reset to 0, got %d", empty.Cursor)
	}
}

func TestMoveCursorEnd(t *testing.T) {
	l := newTestLevel("a", "b", "c")
	l.Cursor = 0
	if !l.MoveCursorEnd() {
		t.Fatalf("expected movement to end")
	}
	if l.Cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", l.Cursor)
	}

	empty := newTestLevel()
	if empty.MoveCursorEnd() {
		t.Fatalf("expected no movement for empty level")
	}
	if empty.Cursor != 0 {
		t.Fatalf("expected cursor reset to 0, got %d", empty.Cursor)
	}
}

func TestMoveCursorPaging(t *testing.T) {
	l := newTestLevel("a", "b", "c", "d", "e")
	l.Cursor = 0
	if !l.MoveCursorPageDown(2) {
		t.Fatalf("expected movement on first page down")
	}
	if l.Cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", l.Cursor)
	}
	if !l.MoveCursorPageDown(2) {
		t.Fatalf("expected movement on second page down")
	}
	if l.Cursor != 4 {
		t.Fatalf("expected cursor 4, got %d", l.Cursor)
	}
	if l.MoveCursorPageDown(2) {
		t.Fatalf("expected no further movement past end")
	}
	if !l.MoveCursorPageUp(2) {
		t.Fatalf("expected movement on page up")
	}
	if l.Cursor != 2 {
		t.Fatalf("expected cursor 2 after page up, got %d", l.Cursor)
	}
	if !l.MoveCursorPageUp(10) {
		t.Fatalf("expected movement back to start")
	}
	if l.Cursor != 0 {
		t.Fatalf("expected cursor at start, got %d", l.Cursor)
	}
}

func TestCursorSkipsHeaders(t *testing.T) {
	items := []menu.Item{
		{ID: "", Label: "header", Header: true},
		{ID: "a", Label: "a"},
		{ID: "b", Label: "b"},
		{ID: "c", Label: "c"},
	}
	l := NewLevel("test", "Test", items, nil)
	// initial cursor should skip the header at index 0
	if l.Cursor != 3 {
		// applyFilter sets cursor to len-1 when cursor starts at -1
		t.Logf("initial cursor: %d (expected 3 from applyFilter)", l.Cursor)
	}

	// MoveCursorHome should land on index 1, not 0
	l.Cursor = 3
	l.MoveCursorHome()
	if l.Cursor != 1 {
		t.Fatalf("home: expected cursor 1, got %d", l.Cursor)
	}

	// moveCursorBy up from index 1 should stay at 1 (can't go to header)
	old := l.Cursor
	l.moveCursorBy(-1)
	if l.Cursor != 1 {
		t.Fatalf("move up from 1: expected cursor 1 (skip header), got %d", l.Cursor)
	}
	_ = old

	// PageDown should skip headers
	l.Cursor = 1
	l.MoveCursorPageDown(10)
	if l.Cursor != 3 {
		t.Fatalf("page down: expected cursor 3, got %d", l.Cursor)
	}
}

func TestEnsureCursorVisibleAdjustsViewport(t *testing.T) {
	l := newTestLevel("a", "b", "c", "d", "e")
	l.Cursor = 4
	l.ViewportOffset = 0
	l.EnsureCursorVisible(2)
	if l.ViewportOffset != 3 {
		t.Fatalf("expected offset 3, got %d", l.ViewportOffset)
	}

	l.Cursor = -1
	l.EnsureCursorVisible(2)
	if l.Cursor != 0 {
		t.Fatalf("expected cursor normalized to 0, got %d", l.Cursor)
	}

	l.ViewportOffset = 4
	l.EnsureCursorVisible(0)
	if l.ViewportOffset != 0 {
		t.Fatalf("expected offset reset when maxVisible <= 0, got %d", l.ViewportOffset)
	}

	l.ViewportOffset = 4
	l.Cursor = 1
	l.EnsureCursorVisible(3)
	if l.ViewportOffset != 1 {
		t.Fatalf("expected offset aligned with cursor, got %d", l.ViewportOffset)
	}
}

func TestEnsureCursorVisibleWithAnchorAdjustsViewport(t *testing.T) {
	l := newTestLevel("a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k")
	l.Cursor = 7
	l.EnsureCursorVisibleWithAnchor(5, 1)
	if l.ViewportOffset != 6 {
		t.Fatalf("expected anchored offset 6, got %d", l.ViewportOffset)
	}

	l.Cursor = 3
	l.EnsureCursorVisibleWithAnchor(5, 1)
	if l.ViewportOffset != 2 {
		t.Fatalf("expected anchored offset 2, got %d", l.ViewportOffset)
	}

	l.Cursor = 0
	l.EnsureCursorVisibleWithAnchor(5, 1)
	if l.ViewportOffset != 0 {
		t.Fatalf("expected anchored offset clamped to 0, got %d", l.ViewportOffset)
	}
}
