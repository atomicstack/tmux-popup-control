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
	m := NewModel("", 0, 0, false, false, nil, "")
	got := m.menuHeader()
	want := defaultRootTitle
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMenuHeaderNestedLevels(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "")
	m.stack = append(m.stack, newLevel("session", "session", nil, nil))
	got := m.menuHeader()
	want := "session"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMenuHeaderDeepLevels(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "")
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

func TestRootMenuOverrideSetsInitialLevel(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "window")
	if got := m.stack[0].ID; got != "window" {
		t.Fatalf("expected root id window, got %s", got)
	}
	if m.rootMenuID != "window" {
		t.Fatalf("expected rootMenuID to be window, got %s", m.rootMenuID)
	}
	if header := m.menuHeader(); header != "window" {
		t.Fatalf("expected header window, got %s", header)
	}
}

func TestRootMenuOverrideIncludesRootInHeaderBreadcrumb(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "window")
	m.stack = append(m.stack, newLevel("window:swap", "swap", nil, nil))
	if header := m.menuHeader(); header != "window→swap" {
		t.Fatalf("expected breadcrumb window→swap, got %s", header)
	}
}

func TestInvalidRootMenuFallsBackToDefault(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "does-not-exist")
	if got := m.stack[0].ID; got != "root" {
		t.Fatalf("expected default root id, got %s", got)
	}
	if m.rootMenuID != "" {
		t.Fatalf("expected empty rootMenuID, got %s", m.rootMenuID)
	}
	if m.errMsg == "" {
		t.Fatalf("expected error message for invalid root menu")
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
	lvl.MultiSelect = true
	lvl.Cursor = 0
	lvl.ToggleCurrentSelection()
	if len(lvl.Selected) != 1 || !lvl.IsSelected("a") {
		t.Fatalf("expected first item selected, got %#v", lvl.Selected)
	}
	lvl.Cursor = 1
	lvl.ToggleCurrentSelection()
	if len(lvl.Selected) != 2 {
		t.Fatalf("expected two selections, got %#v", lvl.Selected)
	}
	lvl.ToggleCurrentSelection()
	if lvl.IsSelected("b") {
		t.Fatalf("expected deselection of second item")
	}
}

func TestLevelCursorPaging(t *testing.T) {
	items := make([]menu.Item, 12)
	for i := range items {
		items[i] = menu.Item{ID: fmt.Sprintf("item-%d", i)}
	}
	lvl := newLevel("test", "Test", items, nil)
	lvl.Cursor = 0
	if !lvl.MoveCursorPageDown(5) || lvl.Cursor != 5 {
		t.Fatalf("expected cursor at 5, got %d", lvl.Cursor)
	}
	if !lvl.MoveCursorPageDown(5) || lvl.Cursor != 10 {
		t.Fatalf("expected cursor at 10, got %d", lvl.Cursor)
	}
	if !lvl.MoveCursorPageDown(5) || lvl.Cursor != 11 {
		t.Fatalf("expected cursor at end, got %d", lvl.Cursor)
	}
	if lvl.MoveCursorPageDown(5) {
		t.Fatalf("expected no movement past end")
	}
	if !lvl.MoveCursorPageUp(5) || lvl.Cursor != 6 {
		t.Fatalf("expected cursor at 6, got %d", lvl.Cursor)
	}
	if !lvl.MoveCursorPageUp(5) || lvl.Cursor != 1 {
		t.Fatalf("expected cursor at 1, got %d", lvl.Cursor)
	}
	if !lvl.MoveCursorPageUp(5) || lvl.Cursor != 0 {
		t.Fatalf("expected cursor at start, got %d", lvl.Cursor)
	}
	if lvl.MoveCursorPageUp(5) {
		t.Fatalf("expected no movement past start")
	}
	lvl.Cursor = 2
	if !lvl.MoveCursorPageDown(0) || lvl.Cursor != len(items)-1 {
		t.Fatalf("expected cursor jump to end with unknown page size, got %d", lvl.Cursor)
	}
}

func TestLevelCursorHomeEnd(t *testing.T) {
	lvl := newLevel("test", "Test", []menu.Item{{ID: "a"}, {ID: "b"}, {ID: "c"}}, nil)
	lvl.Cursor = 1
	if !lvl.MoveCursorHome() || lvl.Cursor != 0 {
		t.Fatalf("expected home to set cursor to 0, got %d", lvl.Cursor)
	}
	if lvl.MoveCursorHome() {
		t.Fatalf("expected no movement when already at home")
	}
	if !lvl.MoveCursorEnd() || lvl.Cursor != 2 {
		t.Fatalf("expected end to set cursor to last item, got %d", lvl.Cursor)
	}
	if lvl.MoveCursorEnd() {
		t.Fatalf("expected no movement when already at end")
	}
	empty := newLevel("empty", "Empty", nil, nil)
	empty.Cursor = 5
	if empty.MoveCursorHome() {
		t.Fatalf("expected no movement for empty menu")
	}
	if empty.Cursor != 0 {
		t.Fatalf("expected empty menu cursor reset to 0, got %d", empty.Cursor)
	}
	if empty.MoveCursorEnd() {
		t.Fatalf("expected no movement for empty menu on end")
	}
	if empty.Cursor != 0 {
		t.Fatalf("expected empty menu cursor stay at 0, got %d", empty.Cursor)
	}
}

func TestStartPaneSwapAddsLevel(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "")
	m.panes.SetEntries([]menu.PaneEntry{{ID: "a", Label: "paneA"}, {ID: "b", Label: "paneB"}})
	initialLevels := len(m.stack)
	m.startPaneSwap(menu.PaneSwapPrompt{Context: m.menuContext(), First: menu.Item{ID: "a", Label: "paneA"}})
	if len(m.stack) != initialLevels+1 {
		t.Fatalf("expected level push, got %d", len(m.stack))
	}
	lvl := m.stack[len(m.stack)-1]
	if lvl.ID != "pane:swap-target" {
		t.Fatalf("unexpected level id %s", lvl.ID)
	}
	if len(lvl.Items) != 1 || lvl.Items[0].ID != "b" {
		t.Fatalf("unexpected items %#v", lvl.Items)
	}
}
