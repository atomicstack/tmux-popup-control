package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// extractLevelID identifies the extract (extrakto-style token picker) level.
const extractLevelID = "extract"

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
// level in place, guarding against a stale reload landing after the user has
// already navigated away from the extract level.
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
		// Stale reply from an earlier ctrl-f, overtaken by a later one.
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
