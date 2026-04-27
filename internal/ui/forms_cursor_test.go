package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestPaneFormCursorVisibleWhenFocused(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 16})
	prompt := menu.PanePrompt{
		Context: menu.Context{},
		Target:  "dev:0.1",
		Initial: "old-name",
	}
	if cmd := m.startPaneForm(prompt); cmd != nil {
		// Focus cmd is a tea.Cmd; running it produces a focus msg the model
		// can ignore for our purpose. We don't need to drain it for the test.
		_ = cmd
	}
	v := m.View()
	if v.Cursor == nil {
		t.Fatalf("expected cursor visible when pane rename form is focused")
	}
	// Cursor lands at the end of the pre-populated value:
	// prompt prefix "» " (2 cells) + len("old-name") (8 cells) = 10.
	if got, want := v.Cursor.Position.X, 10; got != want {
		t.Fatalf("cursor X = %d; want %d", got, want)
	}
	if v.Cursor.Position.Y <= 0 {
		t.Fatalf("cursor Y = %d; want > 0 (input is below header/title)", v.Cursor.Position.Y)
	}
	// Sanity: shape/blink propagated from textinput styles via task 5.
	if v.Cursor.Shape != tea.CursorBlock {
		t.Fatalf("cursor shape = %v; want CursorBlock", v.Cursor.Shape)
	}
	if !v.Cursor.Blink {
		t.Fatalf("cursor should blink")
	}
}

func TestPaneFormCursorMovesOnBackspace(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 16})
	prompt := menu.PanePrompt{
		Context: menu.Context{},
		Target:  "dev:0.1",
		Initial: "abc",
	}
	_ = m.startPaneForm(prompt)
	beforeX := m.View().Cursor.Position.X
	// Send a backspace through the harness so the form's Update path runs.
	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyBackspace})
	afterX := h.Cursor().Position.X
	if afterX != beforeX-1 {
		t.Fatalf("cursor X did not decrement on backspace: before=%d after=%d", beforeX, afterX)
	}
}
