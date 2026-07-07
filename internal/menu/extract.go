package menu

import (
	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// extractCaptureFn is swappable in tests.
var extractCaptureFn = tmux.CaptureVisible

// loadExtractMenu captures the originating pane's visible screen and returns
// the extracted tokens for ctx.ExtractCategory as selectable items. Each
// item's ID and Label are the raw token text.
func loadExtractMenu(ctx Context) ([]Item, error) {
	target := tmux.OriginPaneID()
	text, err := extractCaptureFn(ctx.SocketPath, target)
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
