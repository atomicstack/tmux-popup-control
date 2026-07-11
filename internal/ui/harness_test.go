package ui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
)

type harnessUpdateTestMsg struct{}

func TestHarnessUpdateReturnsCommandWithoutExecutingIt(t *testing.T) {
	m := NewModel(ModelConfig{})
	executed := false
	m.handlers[reflect.TypeFor[harnessUpdateTestMsg]()] = func(tea.Msg) tea.Cmd {
		return func() tea.Msg {
			executed = true
			return nil
		}
	}
	h := NewHarness(m)

	cmd := h.Update(harnessUpdateTestMsg{})

	if cmd == nil {
		t.Fatal("expected update to return the model command")
	}
	if executed {
		t.Fatal("expected update to leave the command unexecuted")
	}
}
