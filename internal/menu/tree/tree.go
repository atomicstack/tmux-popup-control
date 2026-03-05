package tree

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// Item ID prefixes.
const (
	PrefixSession = "tree:s:"
	PrefixWindow  = "tree:w:"
	PrefixPane    = "tree:p:"
)

// State tracks expand/collapse for each tree node.
type State struct {
	expanded   map[string]bool
	allDefault bool // default expand state for nodes not in the map
}

// NewState creates a new tree state. If allExpanded is true, all nodes
// start expanded; otherwise all start collapsed.
func NewState(allExpanded bool) *State {
	return &State{
		expanded:   make(map[string]bool),
		allDefault: allExpanded,
	}
}

// IsExpanded returns whether the node with the given item ID is expanded.
func (s *State) IsExpanded(id string) bool {
	if v, ok := s.expanded[id]; ok {
		return v
	}
	return s.allDefault
}

// SetExpanded sets the expand state for the given item ID.
func (s *State) SetExpanded(id string, expanded bool) {
	s.expanded[id] = expanded
}

// Toggle flips the expand state for the given item ID.
func (s *State) Toggle(id string) {
	s.expanded[id] = !s.IsExpanded(id)
}

// IsExpandable returns true if the item ID represents a session or window
// (nodes that can have children).
func IsExpandable(id string) bool {
	return strings.HasPrefix(id, PrefixSession) || strings.HasPrefix(id, PrefixWindow)
}

// ItemKind returns "session", "window", or "pane" for an item ID.
func ItemKind(id string) string {
	switch {
	case strings.HasPrefix(id, PrefixSession):
		return "session"
	case strings.HasPrefix(id, PrefixWindow):
		return "window"
	case strings.HasPrefix(id, PrefixPane):
		return "pane"
	default:
		return ""
	}
}

// SessionID formats a session item ID.
func SessionID(name string) string {
	return PrefixSession + name
}

// WindowID formats a window item ID.
func WindowID(sessionName, windowID string) string {
	return fmt.Sprintf("%s%s:%s", PrefixWindow, sessionName, windowID)
}

// PaneID formats a pane item ID.
func PaneID(sessionName, windowID, paneID string) string {
	return fmt.Sprintf("%s%s:%s:%s", PrefixPane, sessionName, windowID, paneID)
}

// BuildItems produces the flat item list based on current expand state.
// Sessions are listed in the order provided. Windows and panes appear
// under their parent only when the parent is expanded.
func (s *State) BuildItems(sessions []menu.SessionEntry, windows []menu.WindowEntry, panes []menu.PaneEntry) []menu.Item {
	winBySession := make(map[string][]menu.WindowEntry)
	for _, w := range windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWindow := make(map[string][]menu.PaneEntry)
	for _, p := range panes {
		paneByWindow[p.Window] = append(paneByWindow[p.Window], p)
	}

	var items []menu.Item
	for _, sess := range sessions {
		sid := SessionID(sess.Name)
		items = append(items, menu.Item{ID: sid, Label: sess.Name})

		if !s.IsExpanded(sid) {
			continue
		}
		for _, win := range winBySession[sess.Name] {
			wid := WindowID(sess.Name, win.ID)
			items = append(items, menu.Item{ID: wid, Label: win.Label})

			if !s.IsExpanded(wid) {
				continue
			}
			for _, pane := range paneByWindow[win.ID] {
				pid := PaneID(sess.Name, win.ID, pane.ID)
				items = append(items, menu.Item{ID: pid, Label: pane.Label})
			}
		}
	}
	return items
}

// FilterItems produces a flat item list filtered by query.
// Matching is case-insensitive substring on labels and IDs.
// Matched items keep their ancestor chain visible. All children of a
// matched session or window are included. When query is empty, falls
// back to BuildItems with current expand state.
func (s *State) FilterItems(sessions []menu.SessionEntry, windows []menu.WindowEntry, panes []menu.PaneEntry, query string) []menu.Item {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return s.BuildItems(sessions, windows, panes)
	}

	lower := strings.ToLower(trimmed)

	winBySession := make(map[string][]menu.WindowEntry)
	for _, w := range windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWindow := make(map[string][]menu.PaneEntry)
	for _, p := range panes {
		paneByWindow[p.Window] = append(paneByWindow[p.Window], p)
	}

	var items []menu.Item
	for _, sess := range sessions {
		sid := SessionID(sess.Name)
		sessionMatches := containsFold(sess.Name, lower)

		var sessionChildren []menu.Item
		for _, win := range winBySession[sess.Name] {
			wid := WindowID(sess.Name, win.ID)
			windowMatches := containsFold(win.Label, lower) || containsFold(win.ID, lower)

			var windowChildren []menu.Item
			for _, pane := range paneByWindow[win.ID] {
				pid := PaneID(sess.Name, win.ID, pane.ID)
				paneMatches := containsFold(pane.Label, lower) || containsFold(pane.ID, lower)
				if paneMatches || windowMatches || sessionMatches {
					windowChildren = append(windowChildren, menu.Item{ID: pid, Label: pane.Label})
				}
			}

			if sessionMatches || windowMatches || len(windowChildren) > 0 {
				sessionChildren = append(sessionChildren, menu.Item{ID: wid, Label: win.Label})
				sessionChildren = append(sessionChildren, windowChildren...)
			}
		}

		if sessionMatches || len(sessionChildren) > 0 {
			items = append(items, menu.Item{ID: sid, Label: sess.Name})
			items = append(items, sessionChildren...)
		}
	}
	return items
}

func containsFold(s, lowerSubstr string) bool {
	return strings.Contains(strings.ToLower(s), lowerSubstr)
}
