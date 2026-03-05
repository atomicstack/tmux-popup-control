package ui

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
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
	cursorIdx int,
	width int,
) *tree.Tree {
	winBySession := make(map[string][]menu.WindowEntry)
	for _, w := range windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWindow := make(map[string][]menu.PaneEntry)
	for _, p := range panes {
		paneByWindow[p.Window] = append(paneByWindow[p.Window], p)
	}

	// DFS counter — incremented each time ItemStyleFunc is called.
	// Since we never set a custom renderer on child trees, the root
	// renderer (and its ItemStyleFunc) is used for all levels. The
	// lipgloss tree renderer calls ItemStyleFunc exactly once per
	// visible node, in DFS order — matching our flat item list.
	counter := 0

	root := tree.New()
	for _, sess := range sessions {
		sid := menu.TreeSessionID(sess.Name)
		indicator := treeExpandIndicator(state, sid)
		wc := len(winBySession[sess.Name])
		label := fmt.Sprintf("%s%s (%d windows)", indicator, sess.Name, wc)

		sessionNode := tree.Root(label)

		if state != nil && state.IsExpanded(sid) {
			for _, win := range winBySession[sess.Name] {
				wid := menu.TreeWindowID(sess.Name, win.ID)
				wIndicator := treeExpandIndicator(state, wid)
				wLabel := fmt.Sprintf("%s%s", wIndicator, win.Label)

				windowNode := tree.Root(wLabel)

				if state.IsExpanded(wid) {
					for _, pane := range paneByWindow[win.ID] {
						windowNode.Child(pane.Label)
						_ = menu.TreePaneID(sess.Name, win.ID, pane.ID) // ensure ID is used for consistency
					}
				}
				sessionNode.Child(windowNode)
			}
		}
		root.Child(sessionNode)
	}

	selectedStyle := lipgloss.NewStyle()
	normalStyle := lipgloss.NewStyle()
	if styles.SelectedItem != nil {
		selectedStyle = *styles.SelectedItem
	}
	if styles.Item != nil {
		normalStyle = *styles.Item
	}

	root.ItemStyleFunc(func(_ tree.Children, _ int) lipgloss.Style {
		idx := counter
		counter++
		if idx == cursorIdx {
			return selectedStyle
		}
		return normalStyle
	})

	enumStyle := lipgloss.NewStyle()
	if styles.ItemIndicator != nil {
		enumStyle = *styles.ItemIndicator
	}
	root.EnumeratorStyle(enumStyle)

	if width > 0 {
		// Reserve 1 col for potential scrollbar/edge — the tree renderer
		// pads item lines to this width.
		root.Enumerator(tree.RoundedEnumerator)
	} else {
		root.Enumerator(tree.RoundedEnumerator)
	}

	return root
}

// treeCollapse collapses the current node (or moves to parent if already collapsed/leaf).
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
	// Move cursor to parent node.
	kind := menu.TreeItemKind(item.ID)
	switch kind {
	case "pane":
		// Parent is the window — find it above.
		for i := current.Cursor - 1; i >= 0; i-- {
			if menu.TreeItemKind(current.Items[i].ID) == "window" {
				current.Cursor = i
				m.syncViewport(current)
				return nil
			}
		}
	case "window":
		// Parent is the session — find it above.
		for i := current.Cursor - 1; i >= 0; i-- {
			if menu.TreeItemKind(current.Items[i].ID) == "session" {
				current.Cursor = i
				m.syncViewport(current)
				return nil
			}
		}
	}
	return nil
}

// treeExpand expands the current node (or moves to first child if already expanded).
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
		// Move cursor to first child.
		if current.Cursor+1 < len(current.Items) {
			current.Cursor++
			m.syncViewport(current)
		}
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
func (m *Model) renderTreeView(items []menu.Item, state *menu.TreeState, cursorIdx int, width int) []styledLine {
	if len(items) == 0 {
		return nil
	}

	t := buildTree(
		m.treeSessions,
		m.treeWindows,
		m.treePanes,
		state,
		cursorIdx,
		width,
	)

	rendered := t.String()
	if rendered == "" {
		return nil
	}

	rawLines := strings.Split(rendered, "\n")
	result := make([]styledLine, 0, len(rawLines))
	for _, line := range rawLines {
		if width > 0 {
			w := lipgloss.Width(line)
			if w < width {
				line = line + strings.Repeat(" ", width-w)
			}
		}
		result = append(result, styledLine{
			text: line,
			raw:  true,
		})
	}
	return result
}
