package ui

import tea "github.com/charmbracelet/bubbletea"

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
		msg := cmd()
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
	return h.model.View()
}

// Model exposes the underlying model.
func (h *Harness) Model() *Model {
	return h.model
}
