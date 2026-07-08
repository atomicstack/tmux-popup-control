package menu

import (
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// extractCaptureFn is swappable in tests.
var extractCaptureFn = tmux.CaptureVisible

// extractScrollbackFn is swappable in tests.
var extractScrollbackFn = tmux.CaptureScrollback

// extractWindowPanesFn is swappable in tests.
var extractWindowPanesFn = tmux.WindowPaneIDs

// captureForArea captures pane/window text for ctx.ExtractGrabArea:
//   - Viewport: the originating pane's visible screen.
//   - PaneHistory: the originating pane's full scrollback.
//   - Window: the visible screen of every pane in the originating window,
//     joined in pane-index order.
//   - WindowHistory: the full scrollback of every pane in the originating
//     window, joined in pane-index order.
func captureForArea(ctx Context) (string, error) {
	target := tmux.OriginPaneID()
	switch ctx.ExtractGrabArea {
	case extract.PaneHistory:
		return extractScrollbackFn(ctx.SocketPath, target)
	case extract.Window:
		ids, err := extractWindowPanesFn(ctx.SocketPath, target)
		if err != nil {
			return "", err
		}
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			text, err := extractCaptureFn(ctx.SocketPath, id)
			if err != nil {
				return "", err
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, "\n"), nil
	case extract.WindowHistory:
		ids, err := extractWindowPanesFn(ctx.SocketPath, target)
		if err != nil {
			return "", err
		}
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			text, err := extractScrollbackFn(ctx.SocketPath, id)
			if err != nil {
				return "", err
			}
			parts = append(parts, text)
		}
		return strings.Join(parts, "\n"), nil
	default:
		// extract.Viewport, and any unrecognized value, treated as viewport.
		return extractCaptureFn(ctx.SocketPath, target)
	}
}

// loadExtractMenu captures pane/window text per ctx.ExtractGrabArea and
// returns the extracted tokens for ctx.ExtractCategory as selectable items.
// Each item's ID and Label are the raw token text.
func loadExtractMenu(ctx Context) ([]Item, error) {
	text, err := captureForArea(ctx)
	if err != nil {
		return nil, err
	}
	tokens := extract.Extract(text, ctx.ExtractCategory)
	items := make([]Item, 0, len(tokens))
	for _, tok := range tokens {
		items = append(items, Item{ID: tok.Text, Label: tok.Text})
	}
	return items, nil
}

// SetExtractCaptureForTest swaps extractCaptureFn for the duration of a test
// and returns a func that restores the original. Exported for use by tests in
// other packages (e.g. internal/ui).
func SetExtractCaptureForTest(fn func(string, string) (string, error)) func() {
	orig := extractCaptureFn
	extractCaptureFn = fn
	return func() { extractCaptureFn = orig }
}
