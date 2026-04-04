package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/charmbracelet/x/ansi"
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

func TestTabReplacesCurrentCommandToken(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}
	current := newLevel("command", "command", []menu.Item{
		{ID: "move-window", Label: "move-window [-dr] [-s src-window] [-t dst-window]"},
		{ID: "move-pane", Label: "move-pane [-bdhv] [-s source-pane] [-t target-pane]"},
	}, node)
	current.SetFilter("move-wnd -r", len([]rune("move-wnd")))
	m.stack = []*level{current}

	_ = m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyTab})

	if current.Filter != "move-window -r" {
		t.Fatalf("expected tab to replace current command token, got %q", current.Filter)
	}
	if current.FilterCursorPos() != len([]rune("move-window")) {
		t.Fatalf("expected cursor after replaced token, got %d", current.FilterCursorPos())
	}
}

func TestCurrentCommandSummaryUsesResolvedCommand(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}
	current := newLevel("command", "command", []menu.Item{
		{ID: "move-window", Label: "move-window [-adpr] [-s src-window] [-t dst-window]"},
	}, node)
	current.SetFilter("move-window -t ", len([]rune("move-window -t ")))
	m.stack = []*level{current}

	if got := m.currentCommandSummary(); got == "" {
		t.Fatal("expected summary for move-window")
	}
}

func TestTriggerCompletionIncludesFlagDescriptions(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}

	items := []menu.Item{
		{ID: "move-window", Label: "move-window [-adpr] [-s src-window] [-t dst-window]"},
	}
	m.handleCommandPreloadMsg(commandPreloadMsg{items: items})

	current := newLevel("command", "command", items, node)
	current.SetFilter("move-window ", len([]rune("move-window ")))
	m.stack = []*level{current}

	m.triggerCompletion()
	if m.completion == nil {
		t.Fatal("expected completion state")
	}

	view := ansi.Strip(m.completion.view(80, 10))
	if !strings.Contains(view, "-t <dst-window>") {
		t.Fatalf("expected described flag label in view, got:\n%s", view)
	}
	if !strings.Contains(view, "destination window") {
		t.Fatalf("expected flag description in view, got:\n%s", view)
	}
}
