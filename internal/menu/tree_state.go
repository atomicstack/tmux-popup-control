package menu

import (
	"fmt"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
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

// TreeWindowID formats a window tree item ID using the integer window index
// to avoid colon conflicts with display-format IDs like "session:index".
func TreeWindowID(sessionName string, windowIndex int) string {
	return fmt.Sprintf("%s%s:%d", TreePrefixWindow, sessionName, windowIndex)
}

// TreePaneID formats a pane tree item ID. windowIndex is numeric; paneID is
// the pane's display ID (may contain colons) and is always the last component
// so SplitN(…, 3) captures it correctly.
func TreePaneID(sessionName string, windowIndex int, paneID string) string {
	return fmt.Sprintf("%s%s:%d:%s", TreePrefixPane, sessionName, windowIndex, paneID)
}

// TreeItemsInput carries the data sources used to build a flat tree view.
type TreeItemsInput struct {
	Sessions []SessionEntry
	Windows  []WindowEntry
	Panes    []PaneEntry
}

// BuildTreeItems produces the flat item list based on current expand state.
// paneKey creates a composite key for pane lookup by session and window index.
func paneKey(session string, windowIdx int) string {
	return fmt.Sprintf("%s\x00%d", session, windowIdx)
}

// treeWindowLabel produces a compact label for a window in the tree.
// Since windows are nested under their session, the "session:" prefix is
// stripped, leaving just "index: rest".
func TreeWindowLabel(win WindowEntry) string {
	prefix := fmt.Sprintf("%s:%d: ", win.Session, win.Index)
	if strings.HasPrefix(win.Label, prefix) {
		return fmt.Sprintf("%d: %s", win.Index, win.Label[len(prefix):])
	}
	return win.Label
}

// treePaneLabel produces a compact label for a pane in the tree.
// The "session:window." prefix is stripped, leaving just "paneIndex: rest",
// and the [name:title] block is moved after the command.
func TreePaneLabel(pane PaneEntry) string {
	prefix := fmt.Sprintf("%s:%d.%d: ", pane.Session, pane.WindowIdx, pane.Index)
	rest := strings.TrimPrefix(pane.Label, prefix)
	rest = swapLeadingBracketBlock(rest)
	return fmt.Sprintf("%d: %s", pane.Index, rest)
}

// swapLeadingBracketBlock rearranges "[name:title] command ..." to
// "command [name:title] ...". If the string doesn't start with a bracket
// block, it is returned unchanged.
func swapLeadingBracketBlock(s string) string {
	if !strings.HasPrefix(s, "[") {
		return s
	}
	close := strings.Index(s, "] ")
	if close < 0 {
		return s
	}
	bracket := s[:close+1]
	after := strings.TrimLeft(s[close+1:], " ")
	cmdEnd := strings.Index(after, "  ")
	if cmdEnd < 0 {
		return after + " " + bracket
	}
	command := after[:cmdEnd]
	remaining := after[cmdEnd:]
	return command + " " + bracket + remaining
}

func (s *TreeState) BuildTreeItems(input TreeItemsInput) []Item {
	winBySession := make(map[string][]WindowEntry)
	for _, w := range input.Windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWin := make(map[string][]PaneEntry)
	for _, p := range input.Panes {
		pk := paneKey(p.Session, p.WindowIdx)
		paneByWin[pk] = append(paneByWin[pk], p)
	}

	var items []Item
	for _, sess := range input.Sessions {
		sid := TreeSessionID(sess.Name)
		items = append(items, Item{ID: sid, Label: sess.Name})

		if !s.IsExpanded(sid) {
			continue
		}
		for _, win := range winBySession[sess.Name] {
			wid := TreeWindowID(sess.Name, win.Index)
			items = append(items, Item{ID: wid, Label: TreeWindowLabel(win)})

			if !s.IsExpanded(wid) {
				continue
			}
			for _, pane := range paneByWin[paneKey(sess.Name, win.Index)] {
				pid := TreePaneID(sess.Name, win.Index, pane.ID)
				items = append(items, Item{ID: pid, Label: TreePaneLabel(pane)})
			}
		}
	}
	return items
}

// FilterTreeItems produces a flat item list filtered by query.
// Matched items keep their ancestor chain visible. When query is empty,
// falls back to BuildTreeItems with current expand state.
func (s *TreeState) FilterTreeItems(input TreeItemsInput, query string) []Item {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return s.BuildTreeItems(input)
	}

	words := strings.Fields(trimmed)

	winBySession := make(map[string][]WindowEntry)
	for _, w := range input.Windows {
		winBySession[w.Session] = append(winBySession[w.Session], w)
	}
	paneByWin := make(map[string][]PaneEntry)
	for _, p := range input.Panes {
		pk := paneKey(p.Session, p.WindowIdx)
		paneByWin[pk] = append(paneByWin[pk], p)
	}

	var items []Item
	for _, sess := range input.Sessions {
		sid := TreeSessionID(sess.Name)
		sessionMatches := treeAllWordsMatch(sess.Name, words)

		// Collect children that independently match (per-item matching).
		// Match against short structured fields (Name, Title, Command)
		// rather than the full label which includes long metadata (paths,
		// dimensions, history) that causes fuzzy false positives.
		var sessionChildren []Item
		for _, win := range winBySession[sess.Name] {
			wid := TreeWindowID(sess.Name, win.Index)
			windowMatches := treeAllWordsMatch(win.Name, words)

			var windowChildren []Item
			for _, pane := range paneByWin[paneKey(sess.Name, win.Index)] {
				pid := TreePaneID(sess.Name, win.Index, pane.ID)
				paneContext := pane.Title + " " + pane.Command
				if treeAllWordsMatch(paneContext, words) {
					windowChildren = append(windowChildren, Item{ID: pid, Label: TreePaneLabel(pane)})
				}
			}

			if windowMatches || len(windowChildren) > 0 {
				sessionChildren = append(sessionChildren, Item{ID: wid, Label: TreeWindowLabel(win)})
				sessionChildren = append(sessionChildren, windowChildren...)
			}
		}

		// Show session if it matches or if any descendant matches.
		// Children are only included if they independently match.
		if sessionMatches || len(sessionChildren) > 0 {
			items = append(items, Item{ID: sid, Label: sess.Name})
			items = append(items, sessionChildren...)
		}
	}
	return items
}

// treeAllWordsMatch returns true if every word fuzzy-matches somewhere in context.
func treeAllWordsMatch(context string, words []string) bool {
	for _, w := range words {
		if !fuzzy.MatchNormalizedFold(w, context) {
			return false
		}
	}
	return true
}
