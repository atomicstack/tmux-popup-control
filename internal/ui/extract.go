package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// extractLevelID identifies the extract (extrakto-style token picker) level.
const extractLevelID = "extract"

// extractInsertFn and extractCopyFn are injectable seams over the tmux
// package so tests can stub the actual tmux paste-buffer / set-buffer calls.
var (
	extractInsertFn = tmux.InsertText
	extractCopyFn   = tmux.CopyText
)

// extractDoneMsg carries the result of an extractInsert/extractCopy action.
type extractDoneMsg struct{ err error }

// extractReloadMsg carries the result of an async re-extract triggered by
// ctrl-f cycling the active category. seq ties the reply back to the
// request that triggered it, so a stale reply from an earlier request
// (overtaken by a subsequent ctrl-f) can be detected and dropped.
type extractReloadMsg struct {
	items []menu.Item
	err   error
	seq   int
}

// extractCycleCmd advances the active category and re-captures+extracts
// asynchronously, updating the current level in place (no new stack level,
// filter query preserved). Bumps m.extractSeq and stamps the request with it
// so handleExtractReloadMsg can ignore out-of-order replies from a rapid
// double ctrl-f (see internal/ui/preview.go for the same seq pattern).
func (m *Model) extractCycleCmd() tea.Cmd {
	m.extractCategory = m.extractCategory.Next()
	m.extractSeq++
	seq := m.extractSeq
	ctx := m.menuContext() // now carries the advanced ExtractCategory
	loader := m.extractLoader()
	return func() tea.Msg {
		items, err := loader(ctx)
		return extractReloadMsg{items: items, err: err, seq: seq}
	}
}

// extractLoader resolves the registered extract loader from the menu
// registry.
func (m *Model) extractLoader() menu.Loader {
	if node, ok := m.registry.Find(extractLevelID); ok && node.Loader != nil {
		return node.Loader
	}
	return func(menu.Context) ([]menu.Item, error) { return nil, nil }
}

// handleExtractReloadMsg applies the async re-extract result to the current
// level in place, guarding against a stale reload landing after a later
// ctrl-f (or a re-entry into the extract level) has superseded it. Entry into
// the extract level (handleEnterKey / applyRootMenuOverride) bumps
// m.extractSeq, so a reload dispatched during an earlier visit can never
// match m.extractSeq after the user navigates away and back in — even if the
// level ID check below still matches extractLevelID.
func (m *Model) handleExtractReloadMsg(msg tea.Msg) tea.Cmd {
	reload, ok := msg.(extractReloadMsg)
	if !ok {
		return nil
	}
	current := m.currentLevel()
	if current == nil || current.ID != extractLevelID {
		return nil
	}
	if reload.seq != m.extractSeq {
		// Stale reply from an earlier ctrl-f (overtaken by a later one) or
		// from a prior visit to this level (invalidated by the entry-time
		// seq bump).
		return nil
	}
	if reload.err != nil {
		m.errMsg = reload.err.Error()
		return nil
	}
	m.errMsg = ""
	current.UpdateItems(reload.items)
	current.Subtitle = extractSubtitle(m.extractCategory)
	m.syncViewport(current)
	return nil
}

// extractCategoryOrder lists the categories in ctrl-f cycle order, used to
// render the category header.
var extractCategoryOrder = []extract.Category{
	extract.Word, extract.Path, extract.URL, extract.Quote,
	extract.SQuote, extract.Line, extract.All,
}

// extractSubtitle renders the category cycle header with the active category
// emphasized and a ctrl-f hint. Reuses existing theme styles: FilterPrompt
// (already used to mark the "active" filter prompt) for the active category,
// HeaderItem (already used for dim, non-selectable header rows) for inactive
// categories, and FilterPlaceholder (already used for dim hint/placeholder
// text) for the ctrl-f hint. The returned string embeds ANSI escapes from
// lipgloss Style.Render calls, so callers that place it in a styledLine must
// mark that line raw (see viewVertical's Subtitle handling).
func extractSubtitle(active extract.Category) string {
	parts := make([]string, 0, len(extractCategoryOrder))
	for _, c := range extractCategoryOrder {
		label := c.String()
		if c == active {
			label = styles.FilterPrompt.Render(label)
		} else {
			label = styles.HeaderItem.Render(label)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "  ") + "   " + styles.FilterPlaceholder.Render("‹ctrl-f›")
}

// extractSelectedText returns the text to act on for an insert/copy action:
// marked items if any are selected, else the item under the cursor. Item IDs
// are the raw token text (see internal/menu/extract.go loadExtractMenu).
// Tokens are joined with a newline for the All/Line categories (whole-line
// semantics) and a space otherwise. Returns ("", false) when there is
// nothing to act on (empty list or an out-of-range cursor).
func (m *Model) extractSelectedText() (string, bool) {
	current := m.currentLevel()
	if current == nil || len(current.Items) == 0 {
		return "", false
	}
	var toks []string
	if sel := current.SelectedItems(); len(sel) > 0 {
		for _, s := range sel {
			toks = append(toks, s.ID)
		}
	} else {
		if current.Cursor < 0 || current.Cursor >= len(current.Items) {
			return "", false
		}
		toks = append(toks, current.Items[current.Cursor].ID)
	}
	sep := " "
	if m.extractCategory == extract.All || m.extractCategory == extract.Line {
		sep = "\n"
	}
	return strings.Join(toks, sep), true
}

// extractInsert pastes the selected token(s) into the pane that launched the
// popup (tmux.OriginPaneID), then quits on success.
func (m *Model) extractInsert() tea.Cmd {
	text, ok := m.extractSelectedText()
	if !ok {
		return nil
	}
	sock := m.socketPath
	target := tmux.OriginPaneID()
	return func() tea.Msg { return extractDoneMsg{err: extractInsertFn(sock, target, text)} }
}

// extractCopy stores the selected token(s) in the tmux paste buffer, then
// quits on success.
func (m *Model) extractCopy() tea.Cmd {
	text, ok := m.extractSelectedText()
	if !ok {
		return nil
	}
	sock := m.socketPath
	return func() tea.Msg { return extractDoneMsg{err: extractCopyFn(sock, text)} }
}

// handleExtractDoneMsg reports a failed insert/copy via m.errMsg (no quit) or
// quits the app on success.
func (m *Model) handleExtractDoneMsg(msg tea.Msg) tea.Cmd {
	done, ok := msg.(extractDoneMsg)
	if !ok {
		return nil
	}
	if done.err != nil {
		m.errMsg = done.err.Error()
		return nil
	}
	return tea.Quit
}
