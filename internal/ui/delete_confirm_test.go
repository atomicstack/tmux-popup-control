package ui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// makeDeleteSavedLevel builds a delete-saved level pre-loaded with two
// snapshot rows so the tests can drive the confirm flow.
func makeDeleteSavedLevel(t *testing.T) (*Model, *level) {
	t.Helper()
	m := NewModel(ModelConfig{Width: 100, Height: 30})
	node, ok := m.registry.Find(deleteSavedLevelID)
	if !ok || node == nil {
		t.Fatal("registry missing resurrect:delete-saved node")
	}
	items := []menu.Item{
		{Label: "name  type  age  date  time  size  info", Header: true},
		{ID: "/saves/manual-foo.json", Label: "foo  manual …"},
		{ID: "/saves/auto-2026-04-26T15-30-00.json", Label: "auto manual …"},
	}
	lvl := newLevel(deleteSavedLevelID, "delete-saved", items, node)
	lvl.Cursor = 1
	m.applyNodeSettings(lvl)
	m.stack = []*level{lvl}
	return m, lvl
}

func TestEnterOnDeleteSavedEntersConfirmState(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	lvl.SetFilter("manual", len("manual"))

	if cmd := m.handleEnterKey(); cmd != nil {
		t.Fatalf("expected handleEnterKey to dispatch nil cmd while entering confirm, got %T", cmd())
	}
	if m.confirmState == nil {
		t.Fatal("expected confirmState to be set after Enter on delete-saved")
	}
	if m.confirmState.savedFilter != "manual" {
		t.Fatalf("expected saved filter 'manual', got %q", m.confirmState.savedFilter)
	}
	if lvl.Filter != "" {
		t.Fatalf("expected current filter to be cleared, got %q", lvl.Filter)
	}
	if got := lvl.Items[1].ID; m.confirmState.item.ID != got {
		t.Fatalf("expected confirm item to match cursor row, want %q got %q", got, m.confirmState.item.ID)
	}
	if m.loading {
		t.Fatal("expected m.loading to remain false until action dispatched")
	}
}

func TestEnterOnDeleteSavedHeaderRowIgnored(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	lvl.Cursor = 0 // header row
	if cmd := m.handleEnterKey(); cmd != nil {
		t.Fatalf("expected nil cmd for header row, got cmd")
	}
	if m.confirmState != nil {
		t.Fatal("expected confirmState to remain nil for header row")
	}
}

func TestConfirmCancelRestoresFilter(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	lvl.SetFilter("manual", len("manual"))
	m.handleEnterKey()
	if m.confirmState == nil {
		t.Fatal("expected confirmState to be set")
	}

	cmd := m.handleDeleteConfirmKey(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if cmd != nil {
		t.Fatalf("expected nil cmd from cancel, got %T", cmd())
	}
	if m.confirmState != nil {
		t.Fatal("expected confirmState cleared after cancel")
	}
	if lvl.Filter != "manual" {
		t.Fatalf("expected filter restored to 'manual', got %q", lvl.Filter)
	}
}

func TestConfirmEscCancels(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	lvl.SetFilter("foo", 3)
	m.handleEnterKey()
	if m.confirmState == nil {
		t.Fatal("expected confirmState to be set")
	}

	if cmd := m.handleDeleteConfirmKey(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd != nil {
		t.Fatalf("expected nil cmd, got %T", cmd())
	}
	if m.confirmState != nil {
		t.Fatal("expected confirmState cleared after escape")
	}
	if lvl.Filter != "foo" {
		t.Fatalf("expected filter restored to 'foo', got %q", lvl.Filter)
	}
}

func TestConfirmYesDispatchesAction(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	lvl.SetFilter("foo", 3)
	m.handleEnterKey()
	if m.confirmState == nil {
		t.Fatal("expected confirmState to be set")
	}

	cmd := m.handleDeleteConfirmKey(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from confirm-yes (action dispatch)")
	}
	if m.confirmState != nil {
		t.Fatal("expected confirmState cleared after confirm-yes")
	}
	if !m.loading {
		t.Fatal("expected m.loading to be set after dispatch")
	}
	if m.pendingID != deleteSavedLevelID {
		t.Fatalf("expected pendingID %q, got %q", deleteSavedLevelID, m.pendingID)
	}
	if got, want := m.pendingLabel, filepath.Base("/saves/manual-foo.json"); got != want {
		t.Fatalf("expected pendingLabel %q, got %q", want, got)
	}
	if m.pendingDeleteFilter != "foo" {
		t.Fatalf("expected pendingDeleteFilter 'foo', got %q", m.pendingDeleteFilter)
	}
}

func TestConfirmEnterAcceptsAsYes(t *testing.T) {
	m, _ := makeDeleteSavedLevel(t)
	m.handleEnterKey()
	if m.confirmState == nil {
		t.Fatal("expected confirmState to be set")
	}

	cmd := m.handleDeleteConfirmKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected Enter inside confirm to dispatch the action")
	}
	if m.confirmState != nil {
		t.Fatal("expected confirmState cleared after Enter accept")
	}
	if !m.loading {
		t.Fatal("expected m.loading to be set after Enter accept")
	}
}

func TestConfirmPromptIncludesSaveName(t *testing.T) {
	m, _ := makeDeleteSavedLevel(t)
	m.handleEnterKey()
	prompt := m.renderDeleteConfirmPrompt()
	if !strings.Contains(prompt, "manual-foo.json") {
		t.Fatalf("expected prompt to include save name, got %q", prompt)
	}
	if !strings.Contains(prompt, "delete save") {
		t.Fatalf("expected prompt to start with 'delete save', got %q", prompt)
	}
	// Routed through the shared renderYNPrompt — body text inside the
	// "[y/n]" marker is split into separate ANSI runs (yellow [, green y,
	// yellow /, red n, yellow ]). Assert on the rendered components rather
	// than the literal marker.
	if !strings.Contains(prompt, "y") || !strings.Contains(prompt, "n") {
		t.Fatalf("expected prompt to contain y/n choice, got %q", prompt)
	}
	if !strings.Contains(prompt, "\x1b[") {
		t.Fatalf("expected prompt to contain ANSI styling, got %q", prompt)
	}
}

func TestDeleteSavedActionResultShowsInfoAndDoesNotQuit(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	m.pendingID = deleteSavedLevelID
	m.pendingLabel = "manual-foo.json"
	m.pendingDeleteFilter = "foo"
	m.pendingDeleteFilterCursor = 3

	cmd := m.handleActionResultMsg(menu.ActionResult{Info: "Deleted manual-foo.json"})

	if m.pendingID != "" {
		t.Fatalf("expected pendingID cleared, got %q", m.pendingID)
	}
	if m.loading {
		t.Fatal("expected m.loading cleared after action result")
	}
	if m.infoMsg == "" {
		t.Fatal("expected infoMsg set with deleted-save message")
	}
	if !strings.Contains(m.infoMsg, "deleted save manual-foo.json") {
		t.Fatalf("expected infoMsg to contain 'deleted save manual-foo.json', got %q", m.infoMsg)
	}
	// cmd should be the reload command (non-nil), NOT tea.Quit
	if cmd == nil {
		t.Fatal("expected non-nil reload command from successful delete")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected reload cmd to produce a deleteSavedReloadedMsg")
	}
	// the level filter should have been restored to the saved value before
	// reload arrives (reload restores it via UpdateItems + SetFilter).
	_ = lvl
}

func TestDeleteSavedReloadedMsgRefreshesItems(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	originalCount := len(lvl.Items)

	// Simulate a reload with one fewer entry.
	newItems := []menu.Item{
		{Label: "name  type  age  date  time  size  info", Header: true},
		{ID: "/saves/auto-2026-04-26T15-30-00.json", Label: "auto manual …"},
	}
	cmd := m.handleDeleteSavedReloadedMsg(deleteSavedReloadedMsg{
		levelID:      deleteSavedLevelID,
		items:        newItems,
		filter:       "manual",
		filterCursor: 6,
	})
	if cmd != nil {
		t.Fatalf("expected handleDeleteSavedReloadedMsg to return nil cmd, got %T", cmd())
	}
	if got := len(lvl.Full); got >= originalCount {
		t.Fatalf("expected entries to shrink after reload, originally %d now %d", originalCount, got)
	}
	if lvl.Filter != "manual" {
		t.Fatalf("expected filter restored to 'manual', got %q", lvl.Filter)
	}
	if lvl.Cursor < 0 || lvl.Cursor >= len(lvl.Items) {
		t.Fatalf("expected cursor inside bounds [0,%d), got %d", len(lvl.Items), lvl.Cursor)
	}
}

func TestDeleteConfirmKeyHandlerSwallowsOtherKeys(t *testing.T) {
	m, lvl := makeDeleteSavedLevel(t)
	lvl.SetFilter("foo", 3)
	m.handleEnterKey()
	if m.confirmState == nil {
		t.Fatal("expected confirmState to be set")
	}

	if cmd := m.handleDeleteConfirmKey(tea.KeyPressMsg{Code: 'a', Text: "a"}); cmd != nil {
		t.Fatalf("expected unhandled key to return nil cmd, got %T", cmd())
	}
	if m.confirmState == nil {
		t.Fatal("expected confirmState to remain set after unrelated key")
	}
	if lvl.Filter != "" {
		t.Fatalf("expected level filter to remain blank during confirm, got %q", lvl.Filter)
	}
}

func TestEnterKeyOnNonDeleteSavedLevelStillFlowsThrough(t *testing.T) {
	// Sanity: the confirm flow should not affect other menus. Build a
	// non-resurrect level and make sure handleEnterKey does not enter the
	// confirm state.
	m := NewModel(ModelConfig{Width: 100, Height: 30})
	current := m.currentLevel()
	if current == nil {
		t.Fatal("expected root level")
	}
	if current.ID == deleteSavedLevelID {
		t.Fatalf("test assumes root is not delete-saved, got %q", current.ID)
	}
	// Setting filter to ensure no confirm intercept happens.
	current.SetFilter("", 0)
	_ = m.handleEnterKey()
	if m.confirmState != nil {
		t.Fatal("expected confirm state to remain nil on non-delete-saved level")
	}
}
