package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestPaneCapturePromptSwitchesToForm(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	h := NewHarness(m)
	h.Send(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1", SocketPath: "sock"},
		Template: "test.log",
	})
	if h.Model().mode != ModePaneCaptureForm {
		t.Fatalf("mode = %v, want ModePaneCaptureForm", h.Model().mode)
	}
	if h.Model().paneCaptureForm == nil {
		t.Fatal("paneCaptureForm should not be nil")
	}
}

func TestPaneCapturePreviewMsgUpdatesPreview(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	m.paneCaptureForm = menu.NewPaneCaptureForm(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	m.mode = ModePaneCaptureForm
	seq := m.paneCaptureForm.Seq()
	h := NewHarness(m)
	h.Send(menu.PaneCapturePreviewMsg{Path: "/resolved/path.log", Seq: seq})
	if h.Model().paneCaptureForm.Preview() != "/resolved/path.log" {
		t.Errorf("preview = %q, want %q", h.Model().paneCaptureForm.Preview(), "/resolved/path.log")
	}
}

func TestPaneCapturePreviewMsgStaleDiscarded(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	m.paneCaptureForm = menu.NewPaneCaptureForm(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	m.mode = ModePaneCaptureForm
	m.paneCaptureForm.SetPreview("original", "")
	seq := m.paneCaptureForm.Seq()
	h := NewHarness(m)
	h.Send(menu.PaneCapturePreviewMsg{Path: "/stale/path.log", Seq: seq - 1})
	if h.Model().paneCaptureForm.Preview() != "original" {
		t.Errorf("stale preview was applied: %q", h.Model().paneCaptureForm.Preview())
	}
}

func TestPaneCaptureFormEscReturnsToMenu(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	m.paneCaptureForm = menu.NewPaneCaptureForm(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	m.mode = ModePaneCaptureForm
	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if h.Model().mode != ModeMenu {
		t.Fatalf("mode = %v, want ModeMenu after esc", h.Model().mode)
	}
	if h.Model().paneCaptureForm != nil {
		t.Fatal("paneCaptureForm should be nil after esc")
	}
}
