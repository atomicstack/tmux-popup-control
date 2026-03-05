package menu

import (
	"fmt"
	"strings"
)

// Tree item ID prefixes.
const (
	TreePrefixSession = "tree:s:"
	TreePrefixWindow  = "tree:w:"
	TreePrefixPane    = "tree:p:"
)

// TreeState tracks expand/collapse for each tree node.
type TreeState struct {
	expanded   map[string]bool
	allDefault bool // default expand state for nodes not in the map
}

// NewTreeState creates a new tree state. If allExpanded is true, all nodes
// start expanded; otherwise all start collapsed.
func NewTreeState(allExpanded bool) *TreeState {
	return &TreeState{
		expanded:   make(map[string]bool),
		allDefault: allExpanded,
	}
}

// IsExpanded returns whether the node with the given item ID is expanded.
func (s *TreeState) IsExpanded(id string) bool {
	if v, ok := s.expanded[id]; ok {
		return v
	}
	return s.allDefault
}

// SetExpanded sets the expand state for the given item ID.
func (s *TreeState) SetExpanded(id string, expanded bool) {
	s.expanded[id] = expanded
}

// Toggle flips the expand state for the given item ID.
func (s *TreeState) Toggle(id string) {
	s.expanded[id] = !s.IsExpanded(id)
}

// TreeIsExpandable returns true if the item ID represents a session or window
// (nodes that can have children).
func TreeIsExpandable(id string) bool {
	return strings.HasPrefix(id, TreePrefixSession) || strings.HasPrefix(id, TreePrefixWindow)
}

// TreeItemKind returns "session", "window", or "pane" for a tree item ID.
func TreeItemKind(id string) string {
	switch {
	case strings.HasPrefix(id, TreePrefixSession):
		return "session"
	case strings.HasPrefix(id, TreePrefixWindow):
		return "window"
	case strings.HasPrefix(id, TreePrefixPane):
		return "pane"
	default:
		return ""
	}
}

// TreeSessionID formats a session tree item ID.
func TreeSessionID(name string) string {
	return TreePrefixSession + name
}

// TreeWindowID formats a window tree item ID.
func TreeWindowID(sessionName, windowID string) string {
	return fmt.Sprintf("%s%s:%s", TreePrefixWindow, sessionName, windowID)
}

// TreePaneID formats a pane tree item ID.
func TreePaneID(sessionName, windowID, paneID string) string {
	return fmt.Sprintf("%s%s:%s:%s", TreePrefixPane, sessionName, windowID, paneID)
}

// BuildTreeItems produces the flat item list based on current expand state.
func (s *TreeState) BuildTreeItems(sessions []SessionEntry, windows []WindowEntry, panes []PaneEntry) []Item {
	winBySession := make(map[string][]WindowEntry)
	for _, w := range windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWindow := make(map[string][]PaneEntry)
	for _, p := range panes {
		paneByWindow[p.Window] = append(paneByWindow[p.Window], p)
	}

	var items []Item
	for _, sess := range sessions {
		sid := TreeSessionID(sess.Name)
		items = append(items, Item{ID: sid, Label: sess.Name})

		if !s.IsExpanded(sid) {
			continue
		}
		for _, win := range winBySession[sess.Name] {
			wid := TreeWindowID(sess.Name, win.ID)
			items = append(items, Item{ID: wid, Label: win.Label})

			if !s.IsExpanded(wid) {
				continue
			}
			for _, pane := range paneByWindow[win.ID] {
				pid := TreePaneID(sess.Name, win.ID, pane.ID)
				items = append(items, Item{ID: pid, Label: pane.Label})
			}
		}
	}
	return items
}

// FilterTreeItems produces a flat item list filtered by query.
// Matched items keep their ancestor chain visible. When query is empty,
// falls back to BuildTreeItems with current expand state.
func (s *TreeState) FilterTreeItems(sessions []SessionEntry, windows []WindowEntry, panes []PaneEntry, query string) []Item {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return s.BuildTreeItems(sessions, windows, panes)
	}

	lower := strings.ToLower(trimmed)

	winBySession := make(map[string][]WindowEntry)
	for _, w := range windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWindow := make(map[string][]PaneEntry)
	for _, p := range panes {
		paneByWindow[p.Window] = append(paneByWindow[p.Window], p)
	}

	var items []Item
	for _, sess := range sessions {
		sid := TreeSessionID(sess.Name)
		sessionMatches := treeContainsFold(sess.Name, lower)

		var sessionChildren []Item
		for _, win := range winBySession[sess.Name] {
			wid := TreeWindowID(sess.Name, win.ID)
			windowMatches := treeContainsFold(win.Label, lower) || treeContainsFold(win.ID, lower)

			var windowChildren []Item
			for _, pane := range paneByWindow[win.ID] {
				pid := TreePaneID(sess.Name, win.ID, pane.ID)
				paneMatches := treeContainsFold(pane.Label, lower) || treeContainsFold(pane.ID, lower)
				if paneMatches || windowMatches || sessionMatches {
					windowChildren = append(windowChildren, Item{ID: pid, Label: pane.Label})
				}
			}

			if sessionMatches || windowMatches || len(windowChildren) > 0 {
				sessionChildren = append(sessionChildren, Item{ID: wid, Label: win.Label})
				sessionChildren = append(sessionChildren, windowChildren...)
			}
		}

		if sessionMatches || len(sessionChildren) > 0 {
			items = append(items, Item{ID: sid, Label: sess.Name})
			items = append(items, sessionChildren...)
		}
	}
	return items
}

func treeContainsFold(s, lowerSubstr string) bool {
	return strings.Contains(strings.ToLower(s), lowerSubstr)
}
