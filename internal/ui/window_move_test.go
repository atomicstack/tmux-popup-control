package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func seedWindowMoveModel(t *testing.T) *Model {
	t.Helper()
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	sessions := []menu.SessionEntry{{Name: "work", ID: "$0", Current: true}, {Name: "zed", ID: "$1"}}
	windows := []menu.WindowEntry{
		{ID: "work:0", Label: "work:0: editor", Name: "editor", Session: "work", Index: 0, InternalID: "@0"},
		{ID: "work:1", Label: "work:1: runner", Name: "runner", Session: "work", Index: 1, InternalID: "@1", Current: true},
		{ID: "zed:0", Label: "zed:0: build", Name: "build", Session: "zed", Index: 0, InternalID: "@2"},
	}
	seedSessionTreeStores(m, sessions, windows, nil, "work", "work:1", "work:1: runner", "work", "", "")
	return m
}

func pinnedLabel(m *Model) string {
	current := m.currentLevel()
	return current.Items[current.Cursor].Label
}

func expectQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a quit command, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected tea.QuitMsg")
	}
}

func TestWindowMovePinsCurrentWindowAndSwallowsTyping(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride(windowMoveLevelID)

	if got := pinnedLabel(m); got != menu.WindowMoveMarker+"1: runner" {
		t.Fatalf("pinned label = %q", got)
	}
	m.handleKeyMsg(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if f := m.currentLevel().Filter; f != "" {
		t.Fatalf("filter = %q, typing must be ignored on the move level", f)
	}
	if !strings.Contains(m.View().Content, windowMoveHint) {
		t.Fatal("expected the key hint in the prompt row")
	}

	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	// Next slot is the top of zed: inserted before zed:0.
	if got := pinnedLabel(m); got != menu.WindowMoveMarker+"0: runner" {
		t.Fatalf("pinned label after down = %q", got)
	}
	if id := m.currentLevel().Items[m.currentLevel().Cursor-1].ID; id != menu.TreeSessionID("$1") {
		t.Fatalf("row above the pinned window = %q, want the zed session", id)
	}
}

func TestWindowMoveEnterWithoutMovingQuits(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride(windowMoveLevelID)
	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	expectQuit(t, m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if m.pendingID != "" {
		t.Fatalf("pendingID = %q, an unmoved window must not run a move", m.pendingID)
	}
}

func TestWindowMoveEscapeQuitsWhenInvokedDirectly(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride(windowMoveLevelID)
	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	expectQuit(t, m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEscape}))
	if m.pendingID != "" {
		t.Fatalf("pendingID = %q, escape must not run a move", m.pendingID)
	}
}

func TestWindowMoveEnterAfterMovingRunsMove(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride(windowMoveLevelID)
	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyUp})
	cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !m.loading || m.pendingID != windowMoveLevelID {
		t.Fatalf("expected a pending move, got loading=%v pendingID=%q", m.loading, m.pendingID)
	}
	if m.pendingLabel != "runner → work:0" {
		t.Fatalf("pendingLabel = %q", m.pendingLabel)
	}
}

func TestWindowMoveFromSubmenuEscapeReturnsToWindowMenu(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride("window")
	items, _ := menu.ActionLoaders()[windowMoveLevelID](m.menuContext())
	m.pendingID = windowMoveLevelID
	m.handleCategoryLoadedMsg(categoryLoadedMsg{id: windowMoveLevelID, title: "move", items: items})
	if got := m.currentLevel().ID; got != windowMoveLevelID {
		t.Fatalf("current level = %q", got)
	}
	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.currentLevel().ID; got != "window" {
		t.Fatalf("enter without moving should return to the window menu, at %q", got)
	}
}

func TestWindowMoveInitialisesWhenBackendDataArrives(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	m.applyRootMenuOverride(windowMoveLevelID)
	if m.currentLevel().Data != nil {
		t.Fatal("expected no move state before backend data")
	}
	seeded := seedWindowMoveModel(t)
	m.sessions, m.windows = seeded.sessions, seeded.windows
	m.refreshWindowMove()
	if got := pinnedLabel(m); got != menu.WindowMoveMarker+"1: runner" {
		t.Fatalf("pinned label = %q", got)
	}
}

func TestWindowMoveHintIsNotTruncatedByStyling(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride(windowMoveLevelID)
	view := m.View().Content
	for line := range strings.SplitSeq(view, "\n") {
		if strings.Contains(line, windowMoveHint) && strings.Contains(line, "…") {
			t.Fatalf("prompt row was truncated although it fits in 80 columns: %q", line)
		}
	}
}

func TestWindowMoveRefreshesFromBackendData(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride(windowMoveLevelID)
	m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyDown}) // top of zed

	windows := append(m.windows.Entries(), menu.WindowEntry{
		ID: "zed:1", Label: "zed:1: fresh", Name: "fresh", Session: "zed", Index: 1, InternalID: "@3",
	})
	m.windows.SetEntries(windows)
	m.refreshWindowMove()
	current := m.currentLevel()
	if got := current.Items[len(current.Items)-1].Label; got != "2: fresh" {
		t.Fatalf("last row = %q, want the new window (shuffled to 2)", got)
	}
	if got := pinnedLabel(m); got != menu.WindowMoveMarker+"0: runner" {
		t.Fatalf("pinned label = %q, want it still at the top of zed", got)
	}
}

func TestWindowMoveReportsVanishedWindow(t *testing.T) {
	m := seedWindowMoveModel(t)
	m.applyRootMenuOverride(windowMoveLevelID)
	var remaining []menu.WindowEntry
	for _, w := range m.windows.Entries() {
		if w.InternalID != "@1" {
			remaining = append(remaining, w)
		}
	}
	m.windows.SetEntries(remaining)
	m.refreshWindowMove()
	if len(m.currentLevel().Items) != 0 || !strings.Contains(m.errMsg, "runner no longer exists") {
		t.Fatalf("items = %d err = %q", len(m.currentLevel().Items), m.errMsg)
	}
	if cmd := m.handleKeyMsg(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("enter must do nothing once the window is gone")
	}
}
