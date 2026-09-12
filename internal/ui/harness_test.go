package ui

import (
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"testing"
)

func TestHarnessUpdateReturnsCommandWithoutExecutingIt(t *testing.T) {
	m := NewModel(ModelConfig{})
	m.stack = []*level{newLevel("pane:switch", "panes", []menu.Item{{ID: "dev:0.0"}}, nil)}
	executed := false
	previous := panePreviewFn
	panePreviewFn = func(string, string) (tmux.PanePreviewData, error) {
		executed = true
		return tmux.PanePreviewData{}, nil
	}
	t.Cleanup(func() { panePreviewFn = previous })
	h := NewHarness(m)
	cmd := h.Update(previewTickMsg{})
	if cmd == nil {
		t.Fatal("expected update to return the model command")
	}
	if executed {
		t.Fatal("expected update to leave the command unexecuted")
	}
}
