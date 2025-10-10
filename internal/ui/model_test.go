package ui

import (
	"fmt"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestSessionSwitchItemsFiltersCurrent(t *testing.T) {
	ctx := menu.Context{
		IncludeCurrent: false,
		Sessions: []menu.SessionEntry{
			{Name: "one", Label: "one", Current: true},
			{Name: "two", Label: "two"},
		},
	}
	items := sessionSwitchItems(ctx)
	if len(items) != 1 || items[0].ID != "two" {
		t.Fatalf("expected only non-current session, got %#v", items)
	}
}

func TestMenuHeaderRootLevel(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil)
	got := m.menuHeader()
	want := "main menu"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMenuHeaderNestedLevels(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil)
	m.stack = append(m.stack, newLevel("session", "session", nil, nil))
	got := m.menuHeader()
	want := "session"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMenuHeaderDeepLevels(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil)
	m.stack = append(m.stack, newLevel("pane", "pane", nil, nil))
	m.stack = append(m.stack, newLevel("pane:resize", "Resize", nil, nil))
	m.stack = append(m.stack, newLevel("pane:resize:left", "Left", nil, nil))
	got := m.menuHeader()
	want := "pane→resize→left"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	m.stack = m.stack[:1]
	m.stack = append(m.stack, newLevel("window", "window", nil, nil))
	m.stack = append(m.stack, newLevel("window:swap-target", "Swap A with…", nil, nil))
	got = m.menuHeader()
	want = "window→swap target"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestCurrentWindowMenuItem(t *testing.T) {
	ctx := menu.Context{CurrentWindowID: "s:1", CurrentWindowLabel: "s:1 main"}
	item, ok := currentWindowMenuItem(ctx)
	if !ok {
		t.Fatalf("expected item")
	}
	if item.ID != "s:1" || item.Label[:9] != "[current]" {
		t.Fatalf("unexpected item: %#v", item)
	}
}

func TestLevelToggleSelection(t *testing.T) {
	lvl := newLevel("test", "Test", []menu.Item{{ID: "a"}, {ID: "b"}}, nil)
	lvl.multiSelect = true
	lvl.cursor = 0
	lvl.toggleCurrentSelection()
	if len(lvl.selected) != 1 || !lvl.isSelected("a") {
		t.Fatalf("expected first item selected, got %#v", lvl.selected)
	}
	lvl.cursor = 1
	lvl.toggleCurrentSelection()
	if len(lvl.selected) != 2 {
		t.Fatalf("expected two selections, got %#v", lvl.selected)
	}
	lvl.toggleCurrentSelection()
	if lvl.isSelected("b") {
		t.Fatalf("expected deselection of second item")
	}
}

func TestLevelCursorPaging(t *testing.T) {
	items := make([]menu.Item, 12)
	for i := range items {
		items[i] = menu.Item{ID: fmt.Sprintf("item-%d", i)}
	}
	lvl := newLevel("test", "Test", items, nil)
	lvl.cursor = 0
	if !lvl.moveCursorPageDown(5) || lvl.cursor != 5 {
		t.Fatalf("expected cursor at 5, got %d", lvl.cursor)
	}
	if !lvl.moveCursorPageDown(5) || lvl.cursor != 10 {
		t.Fatalf("expected cursor at 10, got %d", lvl.cursor)
	}
	if !lvl.moveCursorPageDown(5) || lvl.cursor != 11 {
		t.Fatalf("expected cursor at end, got %d", lvl.cursor)
	}
	if lvl.moveCursorPageDown(5) {
		t.Fatalf("expected no movement past end")
	}
	if !lvl.moveCursorPageUp(5) || lvl.cursor != 6 {
		t.Fatalf("expected cursor at 6, got %d", lvl.cursor)
	}
	if !lvl.moveCursorPageUp(5) || lvl.cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", lvl.cursor)
	}
	if !lvl.moveCursorPageUp(5) || lvl.cursor != 0 {
		t.Fatalf("expected cursor at start, got %d", lvl.cursor)
	}
	if lvl.moveCursorPageUp(5) {
		t.Fatalf("expected no movement past start")
	}
	lvl.cursor = 2
	if !lvl.moveCursorPageDown(0) || lvl.cursor != len(items)-1 {
		t.Fatalf("expected cursor jump to end with unknown page size, got %d", lvl.cursor)
	}
}

func TestLevelCursorHomeEnd(t *testing.T) {
	lvl := newLevel("test", "Test", []menu.Item{{ID: "a"}, {ID: "b"}, {ID: "c"}}, nil)
	lvl.cursor = 1
	if !lvl.moveCursorHome() || lvl.cursor != 0 {
		t.Fatalf("expected home to set cursor to 0, got %d", lvl.cursor)
	}
	if lvl.moveCursorHome() {
		t.Fatalf("expected no movement when already at home")
	}
	if !lvl.moveCursorEnd() || lvl.cursor != 2 {
		t.Fatalf("expected end to set cursor to last item, got %d", lvl.cursor)
	}
	if lvl.moveCursorEnd() {
		t.Fatalf("expected no movement when already at end")
	}
	empty := newLevel("empty", "Empty", nil, nil)
	empty.cursor = 5
	if empty.moveCursorHome() {
		t.Fatalf("expected no movement for empty menu")
	}
	if empty.cursor != 0 {
		t.Fatalf("expected empty menu cursor reset to 0, got %d", empty.cursor)
	}
	if empty.moveCursorEnd() {
		t.Fatalf("expected no movement for empty menu on end")
	}
	if empty.cursor != 0 {
		t.Fatalf("expected empty menu cursor stay at 0, got %d", empty.cursor)
	}
}

func TestStartPaneSwapAddsLevel(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil)
	m.panes.SetEntries([]menu.PaneEntry{{ID: "a", Label: "paneA"}, {ID: "b", Label: "paneB"}})
	initialLevels := len(m.stack)
	m.startPaneSwap(menu.PaneSwapPrompt{Context: m.menuContext(), First: menu.Item{ID: "a", Label: "paneA"}})
	if len(m.stack) != initialLevels+1 {
		t.Fatalf("expected level push, got %d", len(m.stack))
	}
	lvl := m.stack[len(m.stack)-1]
	if lvl.id != "pane:swap-target" {
		t.Fatalf("unexpected level id %s", lvl.id)
	}
	if len(lvl.items) != 1 || lvl.items[0].ID != "b" {
		t.Fatalf("unexpected items %#v", lvl.items)
	}
}
