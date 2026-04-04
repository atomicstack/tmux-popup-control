package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestHandleCommandPreloadBuildsSchemas(t *testing.T) {
	m := NewModel(ModelConfig{})
	items := []menu.Item{
		{ID: "kill-session", Label: "kill-session [-aC] [-t target-session]"},
		{ID: "swap-window", Label: "swap-window (swapw) [-d] [-s src-window] [-t target-window]"},
	}

	m.handleCommandPreloadMsg(commandPreloadMsg{items: items})

	if len(m.commandItemsCache) != len(items) {
		t.Fatalf("expected %d cached items, got %d", len(items), len(m.commandItemsCache))
	}
	if m.commandSchemas == nil {
		t.Fatal("expected command schemas to be built")
	}
	if _, ok := m.commandSchemas["kill-session"]; !ok {
		t.Fatal("expected kill-session schema")
	}
	if _, ok := m.commandSchemas["swapw"]; !ok {
		t.Fatal("expected alias schema for swapw")
	}
}

func TestCompletionTriggersOnFlagValue(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	if !h.model.completionVisible() {
		t.Fatal("expected completion dropdown to be visible after '-t '")
	}
	if got := h.model.completion.selected(); got != "main" {
		t.Fatalf("expected first completion candidate to be main, got %q", got)
	}
}

func TestCompletionArrowNavigation(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	first := h.model.completion.selected()
	h.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	second := h.model.completion.selected()
	if first == second {
		t.Fatal("expected cursor to move to different item")
	}
	h.Send(tea.KeyPressMsg{Code: tea.KeyUp})
	back := h.model.completion.selected()
	if back != first {
		t.Fatalf("expected to return to %q, got %q", first, back)
	}
}

func TestCompletionTabAccepts(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	selected := h.model.completion.selected()
	h.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	current := h.model.currentLevel()
	if current == nil {
		t.Fatal("expected current level")
	}
	if !strings.Contains(current.Filter, selected) {
		t.Fatalf("expected filter to contain %q, got %q", selected, current.Filter)
	}
}

func TestCompletionEscapeDismisses(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	if !h.model.completionVisible() {
		t.Fatal("expected dropdown visible")
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if h.model.completionVisible() {
		t.Fatal("expected dropdown dismissed after escape")
	}
}

func TestCompletionTypeToFilter(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "ma")

	if !h.model.completionVisible() {
		t.Fatal("expected dropdown still visible while filtering")
	}
	if len(h.model.completion.filtered) != 1 {
		t.Fatalf("expected 1 filtered candidate, got %d: %v", len(h.model.completion.filtered), h.model.completion.filtered)
	}
	if h.model.completion.filtered[0].Value != "main" {
		t.Fatalf("expected 'main', got %q", h.model.completion.filtered[0].Value)
	}
}

func TestGhostHintShowsCompletionValue(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	ghost := h.model.autoCompleteGhost()
	if ghost == "" {
		t.Fatal("expected non-empty ghost hint after '-t '")
	}
}

func TestCompletionDismissesOnResize(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	if !h.model.completionVisible() {
		t.Fatal("expected dropdown visible")
	}

	h.Send(tea.WindowSizeMsg{Width: 100, Height: 30})
	if h.model.completionVisible() {
		t.Fatal("expected dropdown dismissed after resize")
	}
}

func TestCompletionSelectionPersistsAcrossBackendRefresh(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	h.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	before := h.model.completion.selected()
	if before != "work" {
		t.Fatalf("expected selected candidate 'work', got %q", before)
	}

	h.Send(backendEventMsg{event: backend.Event{
		Kind: backend.KindSessions,
		Data: tmux.SessionSnapshot{
			Sessions: []tmux.Session{
				{Name: "main"},
				{Name: "work"},
				{Name: "scratch"},
			},
			Current:        "main",
			IncludeCurrent: true,
		},
	}})

	after := h.model.completion.selected()
	if after != before {
		t.Fatalf("expected selection to remain %q after backend refresh, got %q", before, after)
	}
}

func TestCompletionDismissedByEscapeStaysDismissedUntilInputChanges(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	if !h.model.completionVisible() {
		t.Fatal("expected dropdown visible")
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if h.model.completionVisible() {
		t.Fatal("expected dropdown dismissed after escape")
	}

	h.Send(backendEventMsg{event: backend.Event{
		Kind: backend.KindSessions,
		Data: tmux.SessionSnapshot{
			Sessions: []tmux.Session{
				{Name: "main"},
				{Name: "work"},
				{Name: "scratch"},
			},
			Current:        "main",
			IncludeCurrent: true,
		},
	}})
	if h.model.completionVisible() {
		t.Fatal("expected dropdown to stay dismissed across backend refresh")
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	h.Send(tea.KeyPressMsg{Code: tea.KeyRight})
	if h.model.completionVisible() {
		t.Fatal("expected cursor movement not to re-trigger dismissed completion")
	}

	sendKeys(h, "m")
	if !h.model.completionVisible() {
		t.Fatal("expected input modification to re-trigger completion")
	}
}

func setupCommandHarness(t *testing.T) *Harness {
	t.Helper()

	model := NewModel(ModelConfig{Width: 80, Height: 20})
	model.sessions.SetEntries([]menu.SessionEntry{
		{Name: "main"},
		{Name: "work"},
		{Name: "scratch"},
	})
	model.windows.SetEntries([]menu.WindowEntry{
		{Name: "0", Session: "main"},
		{Name: "1", Session: "main"},
		{Name: "0", Session: "work"},
	})
	model.panes.SetEntries([]menu.PaneEntry{
		{PaneID: "%1"},
		{PaneID: "%2"},
	})

	commandLines := []string{
		"kill-session [-aC] [-t target-session]",
		"swap-window (swapw) [-d] [-s src-window] [-t target-window]",
		"bind-key (bind) [-nr] [-T key-table] [-N note] key [command [argument ...]]",
	}
	model.commandSchemas = cmdparse.BuildRegistry(commandLines)
	model.commandItemsCache = []menu.Item{
		{ID: "kill-session", Label: commandLines[0]},
		{ID: "swap-window", Label: commandLines[1]},
		{ID: "bind-key", Label: commandLines[2]},
	}

	node, ok := model.registry.Find("command")
	if !ok {
		t.Fatal("expected command node in registry")
	}
	lvl := newLevel("command", "command", model.commandItemsCache, node)
	model.applyNodeSettings(lvl)
	model.stack = []*level{lvl}

	return NewHarness(model)
}

func sendKeys(h *Harness, text string) {
	for _, r := range text {
		h.Send(tea.KeyPressMsg{Text: string(r)})
	}
}
