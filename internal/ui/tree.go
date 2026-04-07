package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// minimalEnumerator uses single-dash connectors with sharp corners.
func minimalEnumerator(children tree.Children, index int) string {
	if children.Length()-1 == index {
		return "└─"
	}
	return "├─"
}

// isTreeLevel returns true if the given level ID uses tree rendering.
func isTreeLevel(id string) bool {
	return id == "session:tree" || id == "window:pull-from-session"
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
	windowCounts map[string]int,
	paneCounts map[string]int,
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

	hasPanes := len(paneCounts) > 0
	root := tree.New()
	for _, sess := range sessions {
		sid := menu.TreeSessionID(sess.Name)
		indicator := treeExpandIndicator(state, sid)
		wc := windowCounts[sess.Name]
		label := fmt.Sprintf("%s%s (%d windows)", indicator, sess.Name, wc)

		sessionNode := tree.Root(label)

		if state != nil && state.IsExpanded(sid) {
			for _, win := range winBySession[sess.Name] {
				wid := menu.TreeWindowID(sess.Name, win.Index)
				wLabel := menu.TreeWindowLabel(win)
				if hasPanes {
					wIndicator := treeExpandIndicator(state, wid)
					pk := fmt.Sprintf("%s\x00%d", sess.Name, win.Index)
					pc := paneCounts[pk]
					wLabel = fmt.Sprintf("%s%s (%d panes)", wIndicator, wLabel, pc)

					windowNode := tree.Root(wLabel)

					if state.IsExpanded(wid) {
						for _, pane := range paneByWindow[fmt.Sprintf("%s\x00%d", sess.Name, win.Index)] {
							windowNode.Child(menu.TreePaneLabel(pane))
						}
					}
					sessionNode.Child(windowNode)
				} else {
					sessionNode.Child(wLabel)
				}
			}
		}
		root.Child(sessionNode)
	}

	root.Enumerator(minimalEnumerator)

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
	sessions, windows, panes := m.treeDataForLevel(current)
	var items []menu.Item
	if current.Filter != "" {
		items = ts.FilterTreeItems(sessions, windows, panes, current.Filter)
	} else {
		items = ts.BuildTreeItems(sessions, windows, panes)
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

// populatePullTreeData fills pullTreeSessions and pullTreeWindows
// from the model's stores, excluding the current session.
func (m *Model) populatePullTreeData() {
	session := m.sessionName
	if session == "" {
		session = m.sessions.Current()
	}
	allSessions := m.sessions.Entries()
	m.pullTreeSessions = make([]menu.SessionEntry, 0, len(allSessions))
	for _, s := range allSessions {
		if s.Name != session {
			m.pullTreeSessions = append(m.pullTreeSessions, s)
		}
	}
	allWindows := m.windows.Entries()
	m.pullTreeWindows = make([]menu.WindowEntry, 0, len(allWindows))
	for _, w := range allWindows {
		if w.Session != session {
			m.pullTreeWindows = append(m.pullTreeWindows, w)
		}
	}
}

// treeDataForLevel returns the source sessions, windows, and panes
// for the given tree level. pull-from-session uses its own filtered
// data (no panes, excluding the current session).
func (m *Model) treeDataForLevel(current *level) ([]menu.SessionEntry, []menu.WindowEntry, []menu.PaneEntry) {
	if current.ID == "window:pull-from-session" {
		return m.pullTreeSessions, m.pullTreeWindows, nil
	}
	return m.treeSessions, m.treeWindows, m.treePanes
}

type treeRenderOptions struct {
	LevelID        string
	Items          []menu.Item
	State          *menu.TreeState
	CursorIdx      int
	Width          int
	ViewportOffset int
	MaxVisible     int
}

// renderTreeView renders the tree as styled lines for display.
// The items parameter determines which nodes are visible — only
// sessions/windows/panes whose IDs appear in items are rendered.
func (m *Model) renderTreeView(opts treeRenderOptions) []styledLine {
	if len(opts.Items) == 0 {
		return nil
	}

	// Build a set of item IDs to determine which nodes are visible.
	idSet := make(map[string]bool, len(opts.Items))
	for _, it := range opts.Items {
		idSet[it.ID] = true
	}

	// Select source data based on tree level type.
	var allSessions []menu.SessionEntry
	var allWindows []menu.WindowEntry
	var allPanes []menu.PaneEntry
	if opts.LevelID == "window:pull-from-session" {
		allSessions = m.pullTreeSessions
		allWindows = m.pullTreeWindows
	} else {
		allSessions = m.treeSessions
		allWindows = m.treeWindows
		allPanes = m.treePanes
	}

	// Filter source data to only include entries present in items.
	sessions := filterTreeSessions(allSessions, idSet)
	windows := filterTreeWindows(allWindows, idSet)
	panes := filterTreePanes(allPanes, idSet)

	// When a filter is active, override expand state so all matching
	// nodes are visible (FilterTreeItems already computed the correct set).
	renderState := opts.State
	if current := m.currentLevel(); current != nil && current.Filter != "" {
		renderState = menu.NewTreeState(true)
	}

	// Build window/pane counts from the full (unfiltered) lists so
	// counts are always correct regardless of expand/collapse state.
	windowCounts := make(map[string]int, len(allSessions))
	for _, w := range allWindows {
		windowCounts[w.Session]++
	}
	paneCounts := make(map[string]int)
	for _, p := range allPanes {
		paneCounts[fmt.Sprintf("%s\x00%d", p.Session, p.WindowIdx)]++
	}

	t := buildTree(
		sessions,
		windows,
		panes,
		renderState,
		windowCounts,
		paneCounts,
	)

	rendered := t.String()
	if rendered == "" {
		return nil
	}

	const indicator = "▌"
	rawLines := strings.Split(rendered, "\n")
	start := opts.ViewportOffset
	if start < 0 {
		start = 0
	}
	if start > len(rawLines) {
		start = len(rawLines)
	}
	end := len(rawLines)
	if opts.MaxVisible > 0 && start+opts.MaxVisible < end {
		end = start + opts.MaxVisible
	}
	visibleLines := rawLines[start:end]
	result := make([]styledLine, 0, len(visibleLines))
	for i, line := range visibleLines {
		absoluteIdx := start + i
		fullText := indicator + " " + line
		if opts.Width > 0 {
			if pad := opts.Width - len([]rune(fullText)); pad > 0 {
				fullText += strings.Repeat(" ", pad)
			}
		}
		lineStyle := styles.Item
		indicatorStyle := styles.ItemIndicator
		if absoluteIdx == opts.CursorIdx {
			lineStyle = styles.SelectedItem
			indicatorStyle = styles.SelectedItemIndicator
		}
		result = append(result, styledLine{
			text:          fullText,
			style:         lineStyle,
			prefixStyle:   indicatorStyle,
			highlightFrom: 1,
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
