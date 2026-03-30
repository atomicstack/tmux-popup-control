package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func testTreeModel(sessions []menu.SessionEntry, windows []menu.WindowEntry, panes []menu.PaneEntry, allExpanded bool) *Model {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
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

func testTreeModelWithSize(sessions []menu.SessionEntry, windows []menu.WindowEntry, panes []menu.PaneEntry, allExpanded bool, width int, height int) *Model {
	m := NewModel(ModelConfig{Width: width, Height: height})
	ts := menu.NewTreeState(allExpanded)
	items := ts.BuildTreeItems(sessions, windows, panes)
	node, _ := m.registry.Find("session:tree")
	lvl := newLevel("session:tree", "tree", items, node)
	lvl.Data = ts
	lvl.Cursor = 0
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
	view := m.View().Content

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
		{ID: "0", Label: "0:bash", Session: "alpha", Index: 0},
		{ID: "1", Label: "1:vim", Session: "alpha", Index: 1},
	}
	panes := []menu.PaneEntry{
		{ID: "%0", Label: "%0", Session: "alpha", WindowIdx: 0},
	}

	m := testTreeModel(sessions, windows, panes, true)
	view := m.View().Content

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
		{ID: "0", Label: "0:bash", Session: "alpha", Index: 0},
		{ID: "1", Label: "1:vim", Session: "alpha", Index: 1},
		{ID: "0", Label: "0:zsh", Session: "beta", Index: 0},
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

	// Press right to expand alpha — cursor stays on alpha.
	h.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	current = h.Model().currentLevel()
	if len(current.Items) != 4 {
		t.Fatalf("expected 4 items after expand (alpha + 2 windows + beta), got %d", len(current.Items))
	}
	if current.Cursor != 0 {
		t.Fatalf("expected cursor to stay at 0 (alpha) after expand, got %d", current.Cursor)
	}

	// Press right again on already-expanded alpha — moves to first child.
	h.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	current = h.Model().currentLevel()
	if current.Cursor != 1 {
		t.Fatalf("expected cursor at 1 (first child) on second right, got %d", current.Cursor)
	}

	// Press left on window — should move to parent session AND collapse it.
	h.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	current = h.Model().currentLevel()
	if current.Cursor != 0 {
		t.Fatalf("expected cursor at 0 (parent session), got %d", current.Cursor)
	}
	if len(current.Items) != 2 {
		t.Fatalf("expected 2 items after left collapses parent, got %d", len(current.Items))
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

func TestTreeFilter(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "alpha", Windows: 1},
		{Name: "beta", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Name: "bash", Session: "alpha"},
		{ID: "0", Label: "0:vim", Name: "vim", Session: "beta"},
	}

	m := testTreeModel(sessions, windows, nil, true)
	h := NewHarness(m)

	// Type "vim" to filter.
	h.Send(tea.KeyPressMsg{Text: "vim"})

	current := h.Model().currentLevel()
	// Should find "beta" session (ancestor) + "0:vim" window.
	found := false
	for _, item := range current.Items {
		if strings.Contains(item.Label, "vim") {
			found = true
		}
	}
	if !found {
		labels := make([]string, len(current.Items))
		for i, it := range current.Items {
			labels[i] = it.Label
		}
		t.Fatalf("expected filter to find 'vim', got items: %v", labels)
	}

	// Ancestor "beta" should be preserved.
	hasBeta := false
	for _, item := range current.Items {
		if item.ID == "tree:s:beta" {
			hasBeta = true
		}
	}
	if !hasBeta {
		t.Fatal("expected ancestor 'beta' session to be preserved in filter results")
	}
}

func TestTreeFilterUpdatesVisibleItems(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "shell", Windows: 1},
		{Name: "test00", Windows: 1},
		{Name: "test02", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "shell"},
		{ID: "0", Label: "0:bash", Session: "test00"},
		{ID: "0", Label: "0:bash", Session: "test02"},
	}

	m := testTreeModel(sessions, windows, nil, false)
	h := NewHarness(m)

	// Type "test02" to filter.
	h.Send(tea.KeyPressMsg{Text: "test02"})

	current := h.Model().currentLevel()

	// Items list should contain test02 but not shell.
	hasTest02 := false
	for _, item := range current.Items {
		if item.ID == "tree:s:test02" {
			hasTest02 = true
		}
		if item.ID == "tree:s:shell" {
			t.Fatal("expected shell to be filtered out of items")
		}
	}
	if !hasTest02 {
		t.Fatal("expected test02 in filtered items")
	}

	// View rendering should show test02 but not shell or test00.
	view := h.Model().View().Content
	if !strings.Contains(view, "test02") {
		t.Fatalf("expected view to contain 'test02', got:\n%s", view)
	}
	if strings.Contains(view, "shell") {
		t.Fatalf("expected 'shell' hidden in filtered view, got:\n%s", view)
	}
	if strings.Contains(view, "test00") {
		t.Fatalf("expected 'test00' hidden in filtered view, got:\n%s", view)
	}
}

func TestTreeFilterCursorNavigation(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "shell", Windows: 1},
		{Name: "test00", Windows: 1},
		{Name: "test02", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "shell"},
		{ID: "0", Label: "0:bash", Session: "test00"},
		{ID: "0", Label: "0:bash", Session: "test02"},
	}

	m := testTreeModel(sessions, windows, nil, false)
	h := NewHarness(m)

	// Type "test" to filter — should match test00 and test02 but not shell.
	h.Send(tea.KeyPressMsg{Text: "test"})

	current := h.Model().currentLevel()
	itemCount := len(current.Items)

	// At minimum test00 and test02 sessions should be present.
	if itemCount < 2 {
		t.Fatalf("expected at least 2 filtered items, got %d", itemCount)
	}
	for _, item := range current.Items {
		if item.ID == "tree:s:shell" {
			t.Fatal("expected shell to be filtered out")
		}
	}

	// Navigate down through all items — cursor should wrap back to 0.
	for i := 0; i < itemCount; i++ {
		h.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	current = h.Model().currentLevel()
	if current.Cursor != 0 {
		t.Fatalf("expected cursor to wrap to 0 after %d down presses, got cursor=%d", itemCount, current.Cursor)
	}
}

func TestTreeFilteredViewportShowsSelectedItem(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "sess00", Windows: 1},
		{Name: "sess01", Windows: 1},
		{Name: "sess02", Windows: 1},
		{Name: "sess03", Windows: 1},
		{Name: "sess04", Windows: 1},
		{Name: "sess05", Windows: 1},
		{Name: "sess06", Windows: 1},
		{Name: "sess07", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "sess00", Index: 0},
		{ID: "0", Label: "0:bash", Session: "sess01", Index: 0},
		{ID: "0", Label: "0:bash", Session: "sess02", Index: 0},
		{ID: "0", Label: "0:bash", Session: "sess03", Index: 0},
		{ID: "0", Label: "0:bash", Session: "sess04", Index: 0},
		{ID: "0", Label: "0:bash", Session: "sess05", Index: 0},
		{ID: "0", Label: "0:bash", Session: "sess06", Index: 0},
		{ID: "0", Label: "0:bash", Session: "sess07", Index: 0},
	}

	m := testTreeModelWithSize(sessions, windows, nil, false, 80, 8)
	h := NewHarness(m)

	h.Send(tea.KeyPressMsg{Text: "sess"})
	h.Send(tea.KeyPressMsg{Code: tea.KeyEnd})

	current := h.Model().currentLevel()
	if current.Filter != "sess" {
		t.Fatalf("expected active filter to be %q, got %q", "sess", current.Filter)
	}
	if current.Cursor != len(current.Items)-1 {
		t.Fatalf("expected cursor at end of filtered items, got %d of %d", current.Cursor, len(current.Items))
	}
	if current.ViewportOffset == 0 {
		t.Fatalf("expected viewport offset to move below the top, got %d", current.ViewportOffset)
	}

	view := h.Model().View().Content
	if !strings.Contains(view, "sess07") {
		t.Fatalf("expected view to contain selected filtered item 'sess07', got:\n%s", view)
	}
	if strings.Contains(view, "sess00") {
		t.Fatalf("expected top-of-list item 'sess00' to be scrolled out, got:\n%s", view)
	}
}

func TestTreeFilterAllItemsSelectable(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "alpha", Windows: 2},
		{Name: "beta", Windows: 1},
		{Name: "gamma", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Name: "bash", Session: "alpha"},
		{ID: "1", Label: "1:vim", Name: "vim", Session: "alpha"},
		{ID: "0", Label: "0:zsh", Name: "zsh", Session: "beta"},
		{ID: "0", Label: "0:top", Name: "top", Session: "gamma"},
	}

	m := testTreeModel(sessions, windows, nil, true) // all expanded
	h := NewHarness(m)

	// Type "vim" — should show alpha (ancestor) + 1:vim window.
	h.Send(tea.KeyPressMsg{Text: "vim"})

	current := h.Model().currentLevel()
	itemCount := len(current.Items)
	if itemCount < 2 {
		labels := make([]string, len(current.Items))
		for i, it := range current.Items {
			labels[i] = it.ID
		}
		t.Fatalf("expected at least 2 items (alpha + 1:vim), got %d: %v", itemCount, labels)
	}

	// Verify every item is reachable by cursor navigation.
	reachable := make(map[string]bool)
	for i := 0; i < itemCount; i++ {
		current = h.Model().currentLevel()
		if current.Cursor >= 0 && current.Cursor < len(current.Items) {
			reachable[current.Items[current.Cursor].ID] = true
		}
		h.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	for _, item := range h.Model().currentLevel().Items {
		if !reachable[item.ID] {
			t.Fatalf("item %q not reachable by cursor navigation", item.ID)
		}
	}

	// View should show vim-related items, not gamma.
	view := h.Model().View().Content
	if !strings.Contains(view, "vim") {
		t.Fatalf("expected view to contain 'vim', got:\n%s", view)
	}
	if strings.Contains(view, "gamma") {
		t.Fatalf("expected 'gamma' to be hidden, got:\n%s", view)
	}
}

func TestTreeRendersCompactLabels(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "dev", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "dev:0: bash", Name: "bash", Session: "dev", Index: 0},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "dev:0.0: [bash:~] vim  [120x40]", Session: "dev", WindowIdx: 0, Index: 0},
	}

	m := testTreeModel(sessions, windows, panes, true)
	view := m.View().Content

	// Window label should use compact form (no session prefix).
	if strings.Contains(view, "dev:0:") {
		t.Fatalf("expected compact window label without session prefix, got:\n%s", view)
	}
	if !strings.Contains(view, "0: bash") {
		t.Fatalf("expected compact window label '0: bash', got:\n%s", view)
	}

	// Pane label should strip prefix and swap [name:cwd] after command.
	if strings.Contains(view, "dev:0.0:") {
		t.Fatalf("expected compact pane label without session prefix, got:\n%s", view)
	}
	if strings.Contains(view, "[bash:~] vim") {
		t.Fatalf("expected [name:cwd] to be swapped after command, got:\n%s", view)
	}
	if !strings.Contains(view, "vim [bash:~]") {
		t.Fatalf("expected 'vim [bash:~]' (command before name/cwd), got:\n%s", view)
	}
}

func TestBuildTreeDFSOrder(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "a", Windows: 1},
		{Name: "b", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "a", Index: 0},
		{ID: "0", Label: "0:zsh", Session: "b", Index: 0},
	}
	panes := []menu.PaneEntry{
		{ID: "%0", Label: "%0", Session: "a", WindowIdx: 0},
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
