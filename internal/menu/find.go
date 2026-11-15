package menu

import (
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/contentsearch"
	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// Menu ids of the find-by-content levels. Their items are search results, not
// a filtered list: the UI runs FindPanes / FindWindows with the typed text.
const (
	PaneFindID   = "pane:find-pane-by-content"
	WindowFindID = "window:find-window-by-content"
)

// findSnippetLead is how much text before a match a result label keeps.
const findSnippetLead = 24

// findCaptureVisibleFn / findCaptureHistoryFn capture one pane's viewport or
// full scrollback. Injectable so tests need no tmux server.
var (
	findCaptureVisibleFn = tmux.CaptureVisible
	findCaptureHistoryFn = tmux.CaptureScrollback
)

// FindResult is one find-by-content hit. Item is the selectable row (its ID is
// the pane or window display id, so the switch actions resolve it as usual);
// PaneID is the tmux id of the pane whose text matched.
type FindResult struct {
	Item   Item
	PaneID string
}

// loadFindMenu opens a find level empty: there are no results until the user
// types something to search for.
func loadFindMenu(Context) ([]Item, error) { return nil, nil }

// FindCapture returns the text a find searches for one pane: its visible
// viewport, or the whole scrollback when history is set.
func FindCapture(socketPath, paneID string, history bool) (string, error) {
	if history {
		return findCaptureHistoryFn(socketPath, paneID)
	}
	return findCaptureVisibleFn(socketPath, paneID)
}

type paneHit struct {
	pane PaneEntry
	line string
}

// findPaneHits captures every pane in ctx and keeps those containing query.
// A pane that fails to capture (it may have just closed) is skipped; the
// search only fails when no pane could be captured at all.
func findPaneHits(ctx Context, query string, history bool) ([]paneHit, error) {
	if contentsearch.Blank(query) {
		return nil, nil
	}
	var (
		hits     []paneHit
		firstErr error
		captured int
	)
	for _, pane := range ctx.Panes {
		target := strings.TrimSpace(pane.PaneID)
		if target == "" {
			continue
		}
		text, err := FindCapture(ctx.SocketPath, target, history)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		captured++
		if match, ok := contentsearch.First(text, query); ok {
			hits = append(hits, paneHit{pane: pane, line: match.Text})
		}
	}
	if captured == 0 && firstErr != nil {
		return nil, firstErr
	}
	return hits, nil
}

// FindPanes returns one result per pane whose text contains query, in pane
// list order. history widens the search from the viewport to all scrollback.
func FindPanes(ctx Context, query string, history bool) ([]FindResult, error) {
	hits, err := findPaneHits(ctx, query, history)
	if err != nil || len(hits) == 0 {
		return nil, err
	}
	rows := make([][]string, 0, len(hits))
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		rows = append(rows, []string{hit.pane.ID, contentsearch.Snippet(hit.line, query, findSnippetLead)})
		ids = append(ids, hit.pane.ID)
	}
	items := tableItems(rows, ids, []table.Alignment{table.AlignLeft, table.AlignLeft})
	results := make([]FindResult, len(items))
	for i, item := range items {
		results[i] = FindResult{Item: item, PaneID: hits[i].pane.PaneID}
	}
	return results, nil
}

// FindWindows returns one result per window that has a pane containing query,
// in the order their first matching panes appear. The result carries that
// first matching pane, which is what the preview shows.
func FindWindows(ctx Context, query string, history bool) ([]FindResult, error) {
	hits, err := findPaneHits(ctx, query, history)
	if err != nil || len(hits) == 0 {
		return nil, err
	}
	seen := make(map[string]struct{}, len(hits))
	var (
		rows    [][]string
		ids     []string
		paneIDs []string
	)
	for _, hit := range hits {
		win, ok := ctx.WindowEntryFor(hit.pane.WindowID)
		if !ok {
			continue
		}
		if _, dup := seen[win.ID]; dup {
			continue
		}
		seen[win.ID] = struct{}{}
		rows = append(rows, []string{win.ID, strings.TrimSpace(win.Name), contentsearch.Snippet(hit.line, query, findSnippetLead)})
		ids = append(ids, win.ID)
		paneIDs = append(paneIDs, hit.pane.PaneID)
	}
	items := tableItems(rows, ids, []table.Alignment{table.AlignLeft, table.AlignLeft, table.AlignLeft})
	results := make([]FindResult, len(items))
	for i, item := range items {
		results[i] = FindResult{Item: item, PaneID: paneIDs[i]}
	}
	return results, nil
}

// SetFindCaptureForTest swaps the viewport and history capture functions for
// the duration of a test and returns a func that restores the originals. A nil
// argument leaves that capture function as it is. Exported for internal/ui.
func SetFindCaptureForTest(visible, history func(string, string) (string, error)) func() {
	origVisible, origHistory := findCaptureVisibleFn, findCaptureHistoryFn
	if visible != nil {
		findCaptureVisibleFn = visible
	}
	if history != nil {
		findCaptureHistoryFn = history
	}
	return func() { findCaptureVisibleFn, findCaptureHistoryFn = origVisible, origHistory }
}
