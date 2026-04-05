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

func TestTriggerCompletionPreservesSynopsisFlagOrder(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}

	items := []menu.Item{
		{ID: "attach-session", Label: "attach-session [-dErx] [-c working-directory] [-f flags] [-t target-session]"},
	}
	m.handleCommandPreloadMsg(commandPreloadMsg{items: items})

	current := newLevel("command", "command", items, node)
	current.SetFilter("attach-session ", len([]rune("attach-session ")))
	m.stack = []*level{current}

	m.triggerCompletion()
	if m.completion == nil {
		t.Fatal("expected completion state")
	}

	var got []string
	for _, item := range m.completion.filtered {
		got = append(got, item.Value)
	}
	want := []string{"-d", "-E", "-r", "-x", "-c", "-f", "-t"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("flag order = %v, want %v", got, want)
	}
}

func TestTriggerCompletionKeepsRepeatableFlagAfterUse(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}

	items := []menu.Item{
		{ID: "new-window", Label: "new-window [-abdkPS] [-c start-directory] [-e environment] [-F format] [-n window-name] [-t target-window] [shell-command [argument ...]]"},
	}
	m.handleCommandPreloadMsg(commandPreloadMsg{items: items})

	current := newLevel("command", "command", items, node)
	current.SetFilter("new-window -a -b -d -k -P -S -c dir -e FOO=bar -F fmt -n name -t work:1 ", len([]rune("new-window -a -b -d -k -P -S -c dir -e FOO=bar -F fmt -n name -t work:1 ")))
	m.stack = []*level{current}

	m.triggerCompletion()
	if m.completion == nil {
		t.Fatal("expected completion state")
	}
	if got := m.completion.filtered[0].Value; got != "-e" {
		t.Fatalf("expected repeatable flag -e to remain available, got %q", got)
	}
}

func TestAcceptCompletionDoesNotAppendTrailingSpace(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}

	items := []menu.Item{
		{ID: "kill-session", Label: "kill-session [-aC] [-t target-session]"},
	}
	m.handleCommandPreloadMsg(commandPreloadMsg{items: items})
	m.sessions.SetEntries([]menu.SessionEntry{{Name: "main"}, {Name: "work"}})

	current := newLevel("command", "command", items, node)
	current.SetFilter("kill-session -t ", len([]rune("kill-session -t ")))
	m.stack = []*level{current}

	m.triggerCompletion()
	if m.completion == nil {
		t.Fatal("expected completion state")
	}

	_ = m.acceptCompletion()

	if current.Filter != "kill-session -t main" {
		t.Fatalf("expected completion without trailing space, got %q", current.Filter)
	}
	if m.completionVisible() {
		t.Fatal("expected completion to stay closed after accepting an item")
	}
}

func TestMoveWindowRenumberTargetsSessionCompletion(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}

	items := []menu.Item{
		{ID: "move-window", Label: "move-window [-adpr] [-s src-window] [-t dst-window]"},
	}
	m.handleCommandPreloadMsg(commandPreloadMsg{items: items})
	m.sessions.SetEntries([]menu.SessionEntry{
		{Name: "renumber-target"},
		{Name: "work"},
	})
	m.windows.SetEntries([]menu.WindowEntry{
		{Name: "0", Session: "renumber-target"},
		{Name: "2", Session: "renumber-target"},
	})

	current := newLevel("command", "command", items, node)
	current.SetFilter("move-window -r -t ", len([]rune("move-window -r -t ")))
	m.stack = []*level{current}

	m.triggerCompletion()
	if m.completion == nil || len(m.completion.filtered) == 0 {
		t.Fatal("expected completion state")
	}
	if got := m.completion.filtered[0].Value; got != "renumber-target" {
		t.Fatalf("expected session completion for move-window -r, got %q", got)
	}
}

func TestExactMatchValueCompletionDismissesDropdown(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}

	items := []menu.Item{
		{ID: "kill-session", Label: "kill-session [-aC] [-t target-session]"},
	}
	m.handleCommandPreloadMsg(commandPreloadMsg{items: items})
	m.sessions.SetEntries([]menu.SessionEntry{
		{Name: "main"},
		{Name: "work"},
	})

	current := newLevel("command", "command", items, node)
	current.SetFilter("kill-session -t main", len([]rune("kill-session -t main")))
	m.stack = []*level{current}

	m.triggerCompletion()
	if m.completionVisible() {
		t.Fatal("expected exact match value completion to dismiss the dropdown")
	}
}

func TestExactMatchFlagCompletionStaysVisibleUntilCommitted(t *testing.T) {
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
	current.SetFilter("move-window -r", len([]rune("move-window -r")))
	m.stack = []*level{current}

	m.triggerCompletion()
	if !m.completionVisible() {
		t.Fatal("expected exact match flag completion to stay visible")
	}
	if got := m.completion.selected(); got != "-r" {
		t.Fatalf("expected selected flag -r, got %q", got)
	}
}
