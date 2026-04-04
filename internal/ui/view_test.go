package ui

import (
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/charmbracelet/x/ansi"
)

func TestViewDisplaysPreviewBlock(t *testing.T) {
	m := NewModel(ModelConfig{})
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m.stack = []*level{lvl}
	m.preview["session:switch"] = &previewData{
		target: "dev",
		label:  "Dev Session",
		lines:  []string{"pane-1", "pane-2"},
		seq:    1,
	}
	view := m.View().Content
	if !strings.Contains(view, "Preview: Dev Session") {
		t.Fatalf("expected preview title in view, got:\n%s", view)
	}
	if !strings.Contains(view, "pane-2") {
		t.Fatalf("expected preview body in view, got:\n%s", view)
	}
}

func TestViewOverlaysCompletionAbovePrompt(t *testing.T) {
	m := NewModel(ModelConfig{Width: 60, Height: 12})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}
	lvl := newLevel("command", "command", []menu.Item{
		{ID: "kill-session", Label: "kill-session [-aC] [-t target-session]"},
	}, node)
	lvl.SetFilter("kill-session -t ", len([]rune("kill-session -t ")))
	m.stack = []*level{lvl}
	m.completion = newCompletionState([]string{"main", "work"}, "target-session", "target-session", 18)

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "main") {
		t.Fatalf("expected completion overlay in view, got:\n%s", view)
	}
	if !strings.Contains(view, "» kill-session -t ") {
		t.Fatalf("expected prompt to remain visible, got:\n%s", view)
	}
}
