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

// Tree item IDs wrap tmux ids ($N, @N, %N) rather than names: session and
// window names may contain ':' and '.' on tmux next-3.8, so a name-based id
// would be ambiguous and, worse, unusable as a tmux target. Entries that
// predate the id fields (or test fixtures without them) fall back to the
// display form via the *Key helpers.

// TreeSessionID formats a session tree item ID from a session key.
func TreeSessionID(key string) string {
	return TreePrefixSession + key
}

// TreeWindowID formats a window tree item ID from a window key.
func TreeWindowID(key string) string {
	return TreePrefixWindow + key
}

// TreePaneID formats a pane tree item ID from a pane key.
func TreePaneID(key string) string {
	return TreePrefixPane + key
}

// TreeSessionKey is the tmux session id when known, else the name.
func TreeSessionKey(s SessionEntry) string {
	if s.ID != "" {
		return s.ID
	}
	return s.Name
}

// TreeWindowKey is the tmux window id when known, else "session:index".
func TreeWindowKey(w WindowEntry) string {
	if w.InternalID != "" {
		return w.InternalID
	}
	if w.Session != "" {
		return fmt.Sprintf("%s:%d", w.Session, w.Index)
	}
	return w.ID
}

// TreePaneKey is the tmux pane id when known, else the display id.
func TreePaneKey(p PaneEntry) string {
	if p.PaneID != "" {
		return p.PaneID
	}
	return p.ID
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
		sid := TreeSessionID(TreeSessionKey(sess))
		items = append(items, Item{ID: sid, Label: sess.Name})

		if !s.IsExpanded(sid) {
			continue
		}
		for _, win := range winBySession[sess.Name] {
			wid := TreeWindowID(TreeWindowKey(win))
			items = append(items, Item{ID: wid, Label: TreeWindowLabel(win)})

			if !s.IsExpanded(wid) {
				continue
			}
			for _, pane := range paneByWin[paneKey(sess.Name, win.Index)] {
				pid := TreePaneID(TreePaneKey(pane))
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
		sid := TreeSessionID(TreeSessionKey(sess))
		sessionMatches := treeAllWordsMatch(sess.Name, words)

		// Collect children that independently match (per-item matching).
		// Match against short structured fields (Name, Title, Command)
		// rather than the full label which includes long metadata (paths,
		// dimensions, history) that causes fuzzy false positives.
		var sessionChildren []Item
		for _, win := range winBySession[sess.Name] {
			wid := TreeWindowID(TreeWindowKey(win))
			windowMatches := treeAllWordsMatch(win.Name, words)

			var windowChildren []Item
			for _, pane := range paneByWin[paneKey(sess.Name, win.Index)] {
				pid := TreePaneID(TreePaneKey(pane))
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
