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

// TestCompletionIncludesUserOptions asserts that live @-prefixed options
// fetched from tmux are merged into option-name completion alongside the
// static catalog. Without this wiring, a set-option prefix "@" yields no
// completions even though the user has defined options.
func TestCompletionIncludesUserOptions(t *testing.T) {
	h := setupCommandHarness(t)
	h.model.userOptionNames = []string{"@catppuccin_flavor", "@plugin"}

	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "@")

	if !h.model.completionVisible() {
		t.Fatal("expected completion visible for @ prefix")
	}
	seen := map[string]bool{}
	for _, item := range h.model.completion.filtered {
		seen[item.Value] = true
	}
	for _, want := range []string{"@catppuccin_flavor", "@plugin"} {
		if !seen[want] {
			t.Errorf("expected %q in completion candidates, got %v", want, completionValues(h))
		}
	}
	// User options should be tagged with the user scope so phase B can
	// colour them distinctly.
	for _, item := range h.model.completion.filtered {
		if item.Value == "@plugin" && item.Scope != ScopeUser {
			t.Errorf("expected @plugin tagged as user scope, got %q", item.Scope)
		}
	}
}

// TestFilterPromptColoursOptionNameBeingTyped asserts that when the user is
// in the middle of typing a tmux option name, the option token in the
// filter input picks up the scope colour (Phase C). The colour must appear
// both while the dropdown is visible and after it dismisses on exact match.
func TestFilterPromptColoursOptionNameBeingTyped(t *testing.T) {
	h := setupCommandHarness(t)

	// Partial prefix — dropdown visible, colour should come from the
	// highlighted candidate ("mouse"), which is a session-scope option.
	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "mou")
	if !h.model.completionVisible() {
		t.Fatal("expected completion visible for 'mou' prefix")
	}
	prompt, _ := h.model.filterPrompt()
	if !strings.Contains(prompt,"\x1b[38;5;39m") && !strings.Contains(prompt,"\x1b[38;5;170m") && !strings.Contains(prompt,"\x1b[38;5;84m") {
		t.Fatalf("expected a scope colour in filter prompt while typing option, got:\n%q", prompt)
	}

	// Continue typing to an exact match ("mouse") — the completion will
	// dismiss itself. The filter prompt must still colour the token via
	// the catalog fallback path.
	sendKeys(h, "se")
	if h.model.completionVisible() {
		t.Fatalf("expected completion dismissed after exact match, still visible: %v", completionValues(h))
	}
	prompt, _ = h.model.filterPrompt()
	if !strings.Contains(prompt,"\x1b[38;5;39m") {
		t.Fatalf("expected session scope colour on exact-match option token, got:\n%q", prompt)
	}
}

// TestFilterPromptColoursUserOption asserts that a live user option (@-prefixed)
// coloured in the filter prompt uses the user scope colour.
func TestFilterPromptColoursUserOption(t *testing.T) {
	h := setupCommandHarness(t)
	h.model.userOptionNames = []string{"@plugin"}

	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "@plugin")

	prompt, _ := h.model.filterPrompt()
	// User scope is colour 220.
	if !strings.Contains(prompt,"\x1b[38;5;220m") {
		t.Fatalf("expected user scope colour (220) in filter prompt, got:\n%q", prompt)
	}
}

// TestFilterPromptDoesNotColourNonOptionCommand asserts that no scope colour
// leaks into the filter prompt for commands that don't complete options.
func TestFilterPromptDoesNotColourNonOptionCommand(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "main")

	prompt, _ := h.model.filterPrompt()
	for _, seq := range []string{"\x1b[38;5;203m", "\x1b[38;5;39m", "\x1b[38;5;170m", "\x1b[38;5;84m", "\x1b[38;5;220m"} {
		if strings.Contains(prompt,seq) {
			t.Fatalf("did not expect scope colour %q in kill-session filter prompt, got:\n%q", seq, prompt)
		}
	}
}

// TestCompletionColourSwatchForBasicNames verifies Phase D: value-completion
// for a colour-typed option renders a coloured swatch block before each
// basic colour name. Extended X11 names fall through without a swatch but
// with padding so rows stay aligned.
func TestCompletionColourSwatchForBasicNames(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "status-fg")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	if !h.model.completionVisible() {
		t.Fatal("expected value completion visible for 'set-option status-fg '")
	}
	var redLabel, aliceLabel string
	for _, item := range h.model.completion.filtered {
		if item.Value == "red" {
			redLabel = item.Label
		}
		if item.Value == "AliceBlue" {
			aliceLabel = item.Label
		}
	}
	if redLabel == "" {
		t.Fatalf("expected red in value candidates")
	}
	if !strings.Contains(redLabel, "█") {
		t.Errorf("expected swatch block in red label, got %q", redLabel)
	}
	// Any ANSI escape sequence before the block implies colour was applied.
	if !strings.Contains(redLabel, "\x1b[") {
		t.Errorf("expected ANSI colour escape in red label, got %q", redLabel)
	}
	if aliceLabel == "" {
		t.Fatalf("expected AliceBlue in value candidates")
	}
	if strings.Contains(aliceLabel, "█") {
		t.Errorf("did not expect swatch block for extended name, got %q", aliceLabel)
	}
	if !strings.HasPrefix(aliceLabel, "  ") {
		t.Errorf("expected two-space padding prefix on extended name, got %q", aliceLabel)
	}
}

// TestColourSpecForName covers the colour-name resolver for all the
// accepted forms and a few non-resolvable cases.
func TestColourSpecForName(t *testing.T) {
	cases := []struct {
		name    string
		want    string
		wantOk  bool
	}{
		{"red", "1", true},
		{"BrightCyan", "14", true},
		{"colour42", "42", true},
		{"color0", "0", true},
		{"#ff8800", "#ff8800", true},
		{"AliceBlue", "", false},
		{"default", "", false},
		{"terminal", "", false},
		{"colour999", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := colourSpecForName(tc.name)
		if ok != tc.wantOk || got != tc.want {
			t.Errorf("colourSpecForName(%q) = (%q, %v); want (%q, %v)", tc.name, got, ok, tc.want, tc.wantOk)
		}
	}
}

// TestCompletionKnownOptionsHaveScopes verifies that catalog options get
// their primary scope threaded through to the completionItem, so Phase B's
// colour pass has something to read.
func TestCompletionKnownOptionsHaveScopes(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "mou")

	if !h.model.completionVisible() {
		t.Fatal("expected completion visible for mou prefix")
	}
	var found *completionItem
	for i := range h.model.completion.filtered {
		if h.model.completion.filtered[i].Value == "mouse" {
			found = &h.model.completion.filtered[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected mouse in filtered candidates, got %v", completionValues(h))
	}
	if found.Scope == "" {
		t.Errorf("expected non-empty scope on mouse option")
	}
}

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

// TestCompletionPageNavigationScopesToDropdown guards that pgup/pgdown while
// the completion popup is focused move the completion cursor only and never
// leak through to the background menu level.
func TestCompletionPageNavigationScopesToDropdown(t *testing.T) {
	h := setupCommandHarness(t)

	// Open option-name completion for set-option; the static catalog yields
	// enough rows that a page step is visibly larger than one.
	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "m")

	if !h.model.completionVisible() {
		t.Fatal("expected option-name completion visible")
	}
	if len(h.model.completion.filtered) < 3 {
		t.Fatalf("need at least 3 candidates to exercise paging, got %d", len(h.model.completion.filtered))
	}

	bgLevel := h.model.currentLevel()
	if bgLevel == nil {
		t.Fatal("expected current level")
	}
	bgCursorBefore := bgLevel.Cursor

	completionCursorBefore := h.model.completion.cursor
	h.Send(tea.KeyPressMsg{Code: tea.KeyPgDown})
	if got := bgLevel.Cursor; got != bgCursorBefore {
		t.Errorf("background cursor moved on pgdown while completion focused: before=%d after=%d", bgCursorBefore, got)
	}
	if h.model.completion.cursor == completionCursorBefore {
		t.Error("expected completion cursor to advance on pgdown")
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeyPgUp})
	if got := bgLevel.Cursor; got != bgCursorBefore {
		t.Errorf("background cursor moved on pgup while completion focused: before=%d after=%d", bgCursorBefore, got)
	}
	if h.model.completion.cursor != completionCursorBefore {
		t.Errorf("expected pgup to return completion cursor to %d, got %d", completionCursorBefore, h.model.completion.cursor)
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
	if current.Filter != "kill-session -t "+selected {
		t.Fatalf("expected filter to contain accepted completion without trailing space, got %q", current.Filter)
	}
	if h.model.completionVisible() {
		t.Fatal("expected dropdown to stay closed after tab acceptance")
	}
}

func TestCompletionEnterExecutesInsteadOfAccepting(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "move-window")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-r")

	if !h.model.completionVisible() {
		t.Fatal("expected exact-match flag completion to be visible")
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	current := h.model.currentLevel()
	if current == nil {
		t.Fatal("expected current level")
	}
	if current.Filter != "" {
		t.Fatalf("expected enter to submit and clear the filter, got %q", current.Filter)
	}
	if !h.model.loading {
		t.Fatal("expected enter to start command execution")
	}
	if h.model.pendingID != "command" {
		t.Fatalf("expected pendingID command, got %q", h.model.pendingID)
	}
	if h.model.pendingLabel != "move-window -r" {
		t.Fatalf("expected pendingLabel to preserve typed command, got %q", h.model.pendingLabel)
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

func TestCompletionDismissedByTabStaysDismissedUntilInputChanges(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "move-window")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-r")
	if !h.model.completionVisible() {
		t.Fatal("expected dropdown visible for exact-match flag completion")
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	if h.model.completionVisible() {
		t.Fatal("expected dropdown dismissed after tab acceptance")
	}

	current := h.model.currentLevel()
	if current == nil {
		t.Fatal("expected current level")
	}
	if current.Filter != "move-window -r" {
		t.Fatalf("expected exact-match tab acceptance to leave filter unchanged, got %q", current.Filter)
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
		t.Fatal("expected dropdown to stay dismissed across backend refresh after tab acceptance")
	}

	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	if !h.model.completionVisible() {
		t.Fatal("expected input modification to re-trigger completion after tab dismissal")
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
		"move-window (movew) [-abdkr] [-s src-window] [-t dst-window]",
		"set-option (set) [-aFgopqsuUw] [-t target-pane] option [value]",
		"set-window-option (setw) [-aFgoqu] [-t target-window] option [value]",
		"set-hook [-agpRuw] [-t target-pane] hook [command]",
		"show-options (show) [-AgHpqsvw] [-t target-pane] [option]",
	}
	model.commandSchemas = cmdparse.BuildRegistry(commandLines)
	model.commandItemsCache = []menu.Item{
		{ID: "kill-session", Label: commandLines[0]},
		{ID: "swap-window", Label: commandLines[1]},
		{ID: "bind-key", Label: commandLines[2]},
		{ID: "move-window", Label: commandLines[3]},
		{ID: "set-option", Label: commandLines[4]},
		{ID: "set-window-option", Label: commandLines[5]},
		{ID: "set-hook", Label: commandLines[6]},
		{ID: "show-options", Label: commandLines[7]},
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
