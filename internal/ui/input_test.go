package ui

import (
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleTextInputAppendsRunes(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "")
	current := m.currentLevel()
	current.UpdateItems([]menu.Item{{ID: "one"}})
	handled := m.handleTextInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abc")})
	if !handled {
		t.Fatalf("expected key press to be handled")
	}
	if current.Filter != "abc" {
		t.Fatalf("expected filter 'abc', got %q", current.Filter)
	}
	if pos := current.FilterCursorPos(); pos != 3 {
		t.Fatalf("expected cursor at end, got %d", pos)
	}
}

func TestHandleTextInputCursorMovement(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "")
	current := m.currentLevel()
	current.UpdateItems([]menu.Item{{ID: "one"}})
	current.SetFilter("abc", 3)

	if !m.handleTextInput(tea.KeyMsg{Type: tea.KeyLeft}) {
		t.Fatalf("expected left arrow to be handled")
	}
	if pos := current.FilterCursorPos(); pos != 2 {
		t.Fatalf("expected cursor at 2 after left, got %d", pos)
	}

	if !m.handleTextInput(tea.KeyMsg{Type: tea.KeyRight}) {
		t.Fatalf("expected right arrow to be handled")
	}
	if pos := current.FilterCursorPos(); pos != 3 {
		t.Fatalf("expected cursor back at 3, got %d", pos)
	}
}

func TestFilterPromptPlaceholder(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "")
	current := m.currentLevel()
	current.SetFilter("", 0)
	prompt, _ := m.filterPrompt()
	if prompt == "" {
		t.Fatalf("expected non-empty prompt")
	}
	if !strings.Contains(prompt, "type to search") {
		t.Fatalf("expected placeholder in prompt, got %q", prompt)
	}
}
