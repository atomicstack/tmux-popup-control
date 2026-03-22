package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// Harness drives the UI model programmatically for integration tests.
type Harness struct {
	model *Model
}

// NewHarness creates a harness for the provided model.
func NewHarness(model *Model) *Harness {
	return &Harness{model: model}
}

// Send routes a message through the model and executes any returned commands.
func (h *Harness) Send(msg tea.Msg) {
	if h.model == nil {
		return
	}
	mdl, cmd := h.model.Update(msg)
	if updated, ok := mdl.(*Model); ok {
		h.model = updated
	}
	h.processCmd(cmd)
}

func (h *Harness) processCmd(cmd tea.Cmd) {
	for cmd != nil {
		// run cmd with a timeout to avoid hanging on timer-based
		// commands (e.g. cursor blink ticks). the real tea.Program
		// runs these in goroutines; the harness is synchronous so it
		// must skip blocking cmds to avoid infinite loops.
		ch := make(chan tea.Msg, 1)
		go func() { ch <- cmd() }()

		var msg tea.Msg
		select {
		case msg = <-ch:
		case <-time.After(10 * time.Millisecond):
			return
		}
		if msg == nil {
			return
		}
		mdl, next := h.model.Update(msg)
		if updated, ok := mdl.(*Model); ok {
			h.model = updated
		}
		cmd = next
	}
}

// View returns the current view string.
func (h *Harness) View() string {
	if h.model == nil {
		return ""
	}
	return h.model.View().Content
}

// Model exposes the underlying model.
func (h *Harness) Model() *Model {
	return h.model
}
