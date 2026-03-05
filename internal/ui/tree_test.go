package ui

import (
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
)

func testTreeModel(sessions []menu.SessionEntry, windows []menu.WindowEntry, panes []menu.PaneEntry, allExpanded bool) *Model {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	ts := menu.NewTreeState(allExpanded)
	items := ts.BuildTreeItems(sessions, windows, panes)
	node, _ := m.registry.Find("session:tree")
	lvl := newLevel("session:tree", "tree", items, node)
	lvl.Data = ts
	lvl.Cursor = 0 // tree starts at the top
	m.treeSessions = sessions
	m.treeWindows = windows
	m.treePanes = panes
	m.stack = append(m.stack, lvl)
	m.syncViewport(lvl)
	return m
}

func TestTreeRendersSessionsCollapsed(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "alpha", Windows: 2},
		{Name: "beta", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "alpha"},
		{ID: "1", Label: "1:vim", Session: "alpha"},
		{ID: "0", Label: "0:zsh", Session: "beta"},
	}

	m := testTreeModel(sessions, windows, nil, false)
	view := m.View()

	if !strings.Contains(view, "alpha") {
		t.Fatalf("expected view to contain 'alpha', got:\n%s", view)
	}
	if !strings.Contains(view, "beta") {
		t.Fatalf("expected view to contain 'beta', got:\n%s", view)
	}
	// Collapsed: should show ▶ indicators
	if !strings.Contains(view, "▶") {
		t.Fatalf("expected ▶ indicator for collapsed sessions, got:\n%s", view)
	}
	// Collapsed: should NOT show window labels
	if strings.Contains(view, "0:bash") {
		t.Fatalf("collapsed tree should not show window labels, got:\n%s", view)
	}
}

func TestTreeRendersExpanded(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "alpha", Windows: 2},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "alpha"},
		{ID: "1", Label: "1:vim", Session: "alpha"},
	}
	panes := []menu.PaneEntry{
		{ID: "%0", Label: "%0", Session: "alpha", Window: "0"},
	}

	m := testTreeModel(sessions, windows, panes, true)
	view := m.View()

	if !strings.Contains(view, "alpha") {
		t.Fatalf("expected view to contain 'alpha', got:\n%s", view)
	}
	if !strings.Contains(view, "0:bash") {
		t.Fatalf("expected view to contain '0:bash', got:\n%s", view)
	}
	if !strings.Contains(view, "▼") {
		t.Fatalf("expected ▼ indicator for expanded nodes, got:\n%s", view)
	}
}

func TestTreeExpandCollapse(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "alpha", Windows: 2},
		{Name: "beta", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "alpha"},
		{ID: "1", Label: "1:vim", Session: "alpha"},
		{ID: "0", Label: "0:zsh", Session: "beta"},
	}

	m := testTreeModel(sessions, windows, nil, false)
	h := NewHarness(m)

	// Initially cursor is on first session (alpha), collapsed.
	current := m.currentLevel()
	if current.Cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", current.Cursor)
	}
	if len(current.Items) != 2 {
		t.Fatalf("expected 2 items (collapsed), got %d", len(current.Items))
	}

	// Press right to expand alpha.
	h.Send(tea.KeyMsg{Type: tea.KeyRight})
	current = h.Model().currentLevel()
	if len(current.Items) != 4 {
		t.Fatalf("expected 4 items after expand (alpha + 2 windows + beta), got %d", len(current.Items))
	}

	// Cursor should move to first child (window 0:bash).
	if current.Cursor != 1 {
		t.Fatalf("expected cursor at 1 after expand, got %d", current.Cursor)
	}

	// Press left to collapse (should move to parent session first).
	h.Send(tea.KeyMsg{Type: tea.KeyLeft})
	current = h.Model().currentLevel()
	// Window is not expandable, left should move to parent session.
	if current.Cursor != 0 {
		t.Fatalf("expected cursor at 0 (parent session), got %d", current.Cursor)
	}

	// Press left again to collapse alpha.
	h.Send(tea.KeyMsg{Type: tea.KeyLeft})
	current = h.Model().currentLevel()
	if len(current.Items) != 2 {
		t.Fatalf("expected 2 items after collapse, got %d", len(current.Items))
	}
}

func TestTreeEnterAction(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "alpha", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "alpha"},
	}

	m := testTreeModel(sessions, windows, nil, false)
	current := m.currentLevel()

	// Cursor on session alpha.
	if current.Items[0].ID != "tree:s:alpha" {
		t.Fatalf("expected first item to be tree:s:alpha, got %s", current.Items[0].ID)
	}
}

func TestBuildTreeDFSOrder(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "a", Windows: 1},
		{Name: "b", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "a"},
		{ID: "0", Label: "0:zsh", Session: "b"},
	}
	panes := []menu.PaneEntry{
		{ID: "%0", Label: "%0", Session: "a", Window: "0"},
	}

	ts := menu.NewTreeState(true) // all expanded
	items := ts.BuildTreeItems(sessions, windows, panes)

	// Expected DFS order: a, 0:bash, %0, b, 0:zsh
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}
	expectedIDs := []string{
		"tree:s:a",
		"tree:w:a:0",
		"tree:p:a:0:%0",
		"tree:s:b",
		"tree:w:b:0",
	}
	for i, expected := range expectedIDs {
		if items[i].ID != expected {
			t.Fatalf("item %d: expected ID %q, got %q", i, expected, items[i].ID)
		}
	}
}
