package ui

import (
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestViewDisplaysPreviewBlock(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "", "")
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m.stack = []*level{lvl}
	m.preview["session:switch"] = &previewData{
		target: "dev",
		label:  "Dev Session",
		lines:  []string{"pane-1", "pane-2"},
		seq:    1,
	}
	view := m.View()
	if !strings.Contains(view, "Preview: Dev Session") {
		t.Fatalf("expected preview title in view, got:\n%s", view)
	}
	if !strings.Contains(view, "pane-2") {
		t.Fatalf("expected preview body in view, got:\n%s", view)
	}
}
