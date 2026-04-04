package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestHandleTextInputAppendsRunes(t *testing.T) {
	m := NewModel(ModelConfig{})
	current := m.currentLevel()
	current.UpdateItems([]menu.Item{{ID: "one"}})
	handled, _ := m.handleTextInput(tea.KeyPressMsg{Text: "abc"})
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
	m := NewModel(ModelConfig{})
	current := m.currentLevel()
	current.UpdateItems([]menu.Item{{ID: "one"}})
	current.SetFilter("abc", 3)

	if handled, _ := m.handleTextInput(tea.KeyPressMsg{Code: tea.KeyLeft}); !handled {
		t.Fatalf("expected left arrow to be handled")
	}
	if pos := current.FilterCursorPos(); pos != 2 {
		t.Fatalf("expected cursor at 2 after left, got %d", pos)
	}

	if handled, _ := m.handleTextInput(tea.KeyPressMsg{Code: tea.KeyRight}); !handled {
		t.Fatalf("expected right arrow to be handled")
	}
	if pos := current.FilterCursorPos(); pos != 3 {
		t.Fatalf("expected cursor back at 3, got %d", pos)
	}
}

func TestFilterPromptPlaceholder(t *testing.T) {
	m := NewModel(ModelConfig{})
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

func TestAutoCompleteGhostStillUsesCommandNameForFirstToken(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}
	current := newLevel("command", "command", []menu.Item{
		{ID: "kill-session", Label: "kill-session [-aC] [-t target-session]"},
	}, node)
	current.Cursor = 0
	current.SetFilter("kill-s", len([]rune("kill-s")))
	m.stack = []*level{current}

	if ghost := m.autoCompleteGhost(); ghost != "ession" {
		t.Fatalf("expected command ghost 'ession', got %q", ghost)
	}
}
