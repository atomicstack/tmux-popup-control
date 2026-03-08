package ui

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2/tree"
)

// isTreeLevel returns true if the given level ID is the session tree.
func isTreeLevel(id string) bool {
	return id == "session:tree"
}

// treeExpandIndicator returns ▼ or ▶ based on expand state, with a trailing space.
func treeExpandIndicator(state *menu.TreeState, id string) string {
	if state != nil && state.IsExpanded(id) {
		return "▼ "
	}
	return "▶ "
}

// buildTree constructs a lipgloss tree from the data model.
// The tree's DFS traversal order matches the flat item list from
// BuildTreeItems, so a counter-based ItemStyleFunc can map cursor
// position to the correct visual node.
func buildTree(
	sessions []menu.SessionEntry,
	windows []menu.WindowEntry,
	panes []menu.PaneEntry,
	state *menu.TreeState,
) *tree.Tree {
	winBySession := make(map[string][]menu.WindowEntry)
	for _, w := range windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWindow := make(map[string][]menu.PaneEntry)
	for _, p := range panes {
		key := fmt.Sprintf("%s\x00%d", p.Session, p.WindowIdx)
		paneByWindow[key] = append(paneByWindow[key], p)
	}

	root := tree.New()
	for _, sess := range sessions {
		sid := menu.TreeSessionID(sess.Name)
		indicator := treeExpandIndicator(state, sid)
		wc := len(winBySession[sess.Name])
		label := fmt.Sprintf("%s%s (%d windows)", indicator, sess.Name, wc)

		sessionNode := tree.Root(label)

		if state != nil && state.IsExpanded(sid) {
			for _, win := range winBySession[sess.Name] {
				wid := menu.TreeWindowID(sess.Name, win.Index)
				wIndicator := treeExpandIndicator(state, wid)
				wLabel := fmt.Sprintf("%s%s", wIndicator, win.Label)

				windowNode := tree.Root(wLabel)

				if state.IsExpanded(wid) {
					for _, pane := range paneByWindow[fmt.Sprintf("%s\x00%d", sess.Name, win.Index)] {
						windowNode.Child(pane.Label)
					}
				}
				sessionNode.Child(windowNode)
			}
		}
		root.Child(sessionNode)
	}

	root.Enumerator(tree.RoundedEnumerator)

	return root
}

// treeCollapse collapses the current node (or moves to parent and collapses it).
func (m *Model) treeCollapse(current *level, ts *menu.TreeState) tea.Cmd {
	if current.Cursor < 0 || current.Cursor >= len(current.Items) {
		return nil
	}
	item := current.Items[current.Cursor]
	if menu.TreeIsExpandable(item.ID) && ts.IsExpanded(item.ID) {
		ts.SetExpanded(item.ID, false)
		m.rebuildTreeItems(current, ts)
		return nil
	}
	// Move cursor to parent node and collapse it.
	kind := menu.TreeItemKind(item.ID)
	switch kind {
	case "pane":
		// Parent is the window — find it above and collapse.
		for i := current.Cursor - 1; i >= 0; i-- {
			if menu.TreeItemKind(current.Items[i].ID) == "window" {
				ts.SetExpanded(current.Items[i].ID, false)
				current.Cursor = i
				m.rebuildTreeItems(current, ts)
				return nil
			}
		}
	case "window":
		// Parent is the session — find it above and collapse.
		for i := current.Cursor - 1; i >= 0; i-- {
			if menu.TreeItemKind(current.Items[i].ID) == "session" {
				ts.SetExpanded(current.Items[i].ID, false)
				current.Cursor = i
				m.rebuildTreeItems(current, ts)
				return nil
			}
		}
	}
	return nil
}

// treeExpand expands the current node (cursor stays on it) or moves to
// first child if already expanded.
func (m *Model) treeExpand(current *level, ts *menu.TreeState) tea.Cmd {
	if current.Cursor < 0 || current.Cursor >= len(current.Items) {
		return nil
	}
	item := current.Items[current.Cursor]
	if !menu.TreeIsExpandable(item.ID) {
		return nil
	}
	if !ts.IsExpanded(item.ID) {
		ts.SetExpanded(item.ID, true)
		m.rebuildTreeItems(current, ts)
		// Cursor stays on the expanded item.
		return nil
	}
	// Already expanded — move to first child.
	if current.Cursor+1 < len(current.Items) {
		current.Cursor++
		m.syncViewport(current)
	}
	return nil
}

// rebuildTreeItems regenerates the flat item list from tree state.
// For tree levels, we bypass the generic filter (which would strip ancestor
// nodes) and use FilterTreeItems which preserves the ancestor chain.
func (m *Model) rebuildTreeItems(current *level, ts *menu.TreeState) {
	cursorID := ""
	if current.Cursor >= 0 && current.Cursor < len(current.Items) {
		cursorID = current.Items[current.Cursor].ID
	}
	var items []menu.Item
	if current.Filter != "" {
		items = ts.FilterTreeItems(m.treeSessions, m.treeWindows, m.treePanes, current.Filter)
	} else {
		items = ts.BuildTreeItems(m.treeSessions, m.treeWindows, m.treePanes)
	}
	// Set both Full and Items directly to bypass the generic FilterItems
	// which would strip ancestor nodes that don't match the filter query.
	current.Full = items
	current.Items = items
	// Restore cursor to the same item if possible.
	if cursorID != "" {
		for i, it := range current.Items {
			if it.ID == cursorID {
				current.Cursor = i
				break
			}
		}
	}
	if current.Cursor >= len(current.Items) {
		if len(current.Items) > 0 {
			current.Cursor = len(current.Items) - 1
		} else {
			current.Cursor = 0
		}
	}
	m.syncViewport(current)
}

// syncTreeFilter rebuilds tree items after filter text changes.
// Must be called after any filter modification on a tree level.
func (m *Model) syncTreeFilter(current *level) {
	if current == nil || !isTreeLevel(current.ID) {
		return
	}
	ts, ok := current.Data.(*menu.TreeState)
	if !ok || ts == nil {
		return
	}
	m.rebuildTreeItems(current, ts)
}

// renderTreeView renders the tree as styled lines for display.
// Each output line is a styledLine with raw=true since the tree
// renderer embeds ANSI escape codes via lipgloss styles.
// The items parameter determines which nodes are visible — only
// sessions/windows/panes whose IDs appear in items are rendered.
func (m *Model) renderTreeView(items []menu.Item, state *menu.TreeState, cursorIdx int, width int) []styledLine {
	if len(items) == 0 {
		return nil
	}

	// Build a set of item IDs to determine which nodes are visible.
	idSet := make(map[string]bool, len(items))
	for _, it := range items {
		idSet[it.ID] = true
	}

	// Filter source data to only include entries present in items.
	sessions := filterTreeSessions(m.treeSessions, idSet)
	windows := filterTreeWindows(m.treeWindows, idSet)
	panes := filterTreePanes(m.treePanes, idSet)

	// When a filter is active, override expand state so all matching
	// nodes are visible (FilterTreeItems already computed the correct set).
	renderState := state
	if current := m.currentLevel(); current != nil && current.Filter != "" {
		renderState = menu.NewTreeState(true)
	}

	t := buildTree(
		sessions,
		windows,
		panes,
		renderState,
	)

	rendered := t.String()
	if rendered == "" {
		return nil
	}

	rawLines := strings.Split(rendered, "\n")
	result := make([]styledLine, 0, len(rawLines))
	for i, line := range rawLines {
		if width > 0 {
			if pad := width - len([]rune(line)); pad > 0 {
				line = line + strings.Repeat(" ", pad)
			}
		}
		lineStyle := styles.Item
		if i == cursorIdx {
			lineStyle = styles.SelectedItem
		}
		result = append(result, styledLine{
			text:  line,
			style: lineStyle,
		})
	}
	return result
}

// filterTreeSessions returns only sessions whose tree IDs are in idSet.
func filterTreeSessions(sessions []menu.SessionEntry, idSet map[string]bool) []menu.SessionEntry {
	result := make([]menu.SessionEntry, 0, len(sessions))
	for _, s := range sessions {
		if idSet[menu.TreeSessionID(s.Name)] {
			result = append(result, s)
		}
	}
	return result
}

// filterTreeWindows returns only windows whose tree IDs are in idSet.
func filterTreeWindows(windows []menu.WindowEntry, idSet map[string]bool) []menu.WindowEntry {
	result := make([]menu.WindowEntry, 0, len(windows))
	for _, w := range windows {
		if idSet[menu.TreeWindowID(w.Session, w.Index)] {
			result = append(result, w)
		}
	}
	return result
}

// filterTreePanes returns only panes whose tree IDs are in idSet.
func filterTreePanes(panes []menu.PaneEntry, idSet map[string]bool) []menu.PaneEntry {
	result := make([]menu.PaneEntry, 0, len(panes))
	for _, p := range panes {
		if idSet[menu.TreePaneID(p.Session, p.WindowIdx, p.ID)] {
			result = append(result, p)
		}
	}
	return result
}
