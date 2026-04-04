package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestHandleActionResultMsgShowsCommandOutput(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 12})
	m.pendingLabel = "list-keys"
	cmd := m.handleActionResultMsg(menu.ActionResult{
		Info:   "Ran: list-keys",
		Output: "first line\nsecond line",
	})

	if cmd != nil {
		t.Fatalf("expected no quit command when showing output, got %v", cmd)
	}
	if m.mode != ModeCommandOutput {
		t.Fatalf("mode = %v, want ModeCommandOutput", m.mode)
	}
	if m.commandOutputTitle != "list-keys" {
		t.Fatalf("title = %q, want list-keys", m.commandOutputTitle)
	}
	if len(m.commandOutputLines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(m.commandOutputLines))
	}
}

func TestCommandOutputEscapeReturnsToMenu(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 8})
	m.mode = ModeCommandOutput
	m.commandOutputTitle = "show-options -s"
	m.commandOutputLines = []string{"status on"}

	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	if h.Model().mode != ModeMenu {
		t.Fatalf("mode = %v, want ModeMenu", h.Model().mode)
	}
	if h.Model().commandOutputTitle != "" {
		t.Fatalf("expected output title cleared, got %q", h.Model().commandOutputTitle)
	}
}

func TestCommandOutputKeyScrolls(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 5})
	m.mode = ModeCommandOutput
	m.commandOutputLines = []string{"one", "two", "three", "four", "five", "six"}

	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	if h.Model().commandOutputOffset != 1 {
		t.Fatalf("offset = %d, want 1", h.Model().commandOutputOffset)
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeyEnd})
	if h.Model().commandOutputOffset == 0 {
		t.Fatalf("expected end to scroll to bottom")
	}
}
