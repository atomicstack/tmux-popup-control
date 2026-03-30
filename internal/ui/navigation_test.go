package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestHandleEscapeKeyFromRootQuits(t *testing.T) {
	m := NewModel(ModelConfig{})
	cmd := m.handleEscapeKey()
	if cmd == nil {
		t.Fatalf("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestHandleEscapeKeyPopsLevelAndClearsSwapState(t *testing.T) {
	m := NewModel(ModelConfig{})
	parent := m.currentLevel()
	parent.Items = []menu.Item{{ID: "one"}, {ID: "two"}, {ID: "window:swap-target"}}
	parent.Cursor = 1
	parent.LastCursor = 2

	swap := newLevel("window:swap-target", "Swap", []menu.Item{{ID: "a", Label: "A"}}, nil)
	m.stack = append(m.stack, swap)
	m.pendingWindowSwap = &menu.Item{ID: "a", Label: "A"}
	m.errMsg = "previous error"

	cmd := m.handleEscapeKey()
	if cmd != nil {
		t.Fatalf("expected no command when popping a level")
	}
	if len(m.stack) != 1 {
		t.Fatalf("expected stack to shrink to 1, got %d", len(m.stack))
	}
	if parent.Cursor != 2 {
		t.Fatalf("expected parent cursor restored to 2, got %d", parent.Cursor)
	}
	if parent.LastCursor != -1 {
		t.Fatalf("expected parent LastCursor reset, got %d", parent.LastCursor)
	}
	if m.pendingWindowSwap != nil {
		t.Fatalf("expected pending window swap cleared")
	}
	if m.errMsg != "" {
		t.Fatalf("expected error message cleared, got %q", m.errMsg)
	}
}

func TestLayoutPreviewRevertsOnEscape(t *testing.T) {
	var applied []string
	old := layoutPreviewFn
	layoutPreviewFn = func(_, layout string) error {
		applied = append(applied, layout)
		return nil
	}
	defer func() { layoutPreviewFn = old }()

	m := NewModel(ModelConfig{SocketPath: "test.sock", Width: 80, Height: 24})
	// Need a parent level so escape doesn't quit
	root := m.stack[0]
	_ = root

	items := []menu.Item{
		{ID: "even-horizontal", Label: "Even Horizontal"},
		{ID: "tiled", Label: "Tiled"},
		{ID: "original-layout-string", Label: "current layout"},
	}
	lvl := newLevel("window:layout", "Layout", items, nil)
	lvl.Data = "original-layout-string"
	m.stack = append(m.stack, lvl)

	h := NewHarness(m)
	applied = nil // reset
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	if len(applied) == 0 {
		t.Fatal("expected revert on escape")
	}
	if applied[0] != "original-layout-string" {
		t.Fatalf("expected revert to original-layout-string, got %q", applied[0])
	}
	// Should have popped back to root
	if len(m.stack) != 1 {
		t.Fatalf("expected stack length 1 after escape, got %d", len(m.stack))
	}
}

func TestRootMenuLeafActionDeferredUntilPaneData(t *testing.T) {
	// When --root-menu specifies a leaf action (like pane:capture), the
	// action must be deferred until the backend provides pane data.
	// Otherwise ctx.CurrentPaneID is empty and the action fails with
	// "no current pane".
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "pane:capture"})
	if m.deferredAction == nil {
		t.Fatal("expected deferredAction to be set for leaf action root menu")
	}
	if m.rootMenuID != "pane:capture" {
		t.Fatalf("rootMenuID = %q, want pane:capture", m.rootMenuID)
	}

	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Send a session event first — this must NOT trigger the deferred
	// action because pane data hasn't arrived yet.
	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}},
		Current:  "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})
	if h.Model().deferredAction == nil {
		t.Fatal("deferredAction should still be pending after session event")
	}
	if h.Model().mode != ModeMenu {
		t.Fatalf("mode = %v, want ModeMenu (action should not have fired yet)", h.Model().mode)
	}

	// Send a window event — still no pane data, action must remain deferred.
	winSnap := tmux.WindowSnapshot{
		Windows:        []tmux.Window{{ID: "main:0", Session: "main", Name: "vim", Current: true}},
		CurrentID:      "main:0",
		CurrentSession: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: winSnap}})
	if h.Model().deferredAction == nil {
		t.Fatal("deferredAction should still be pending after window event")
	}

	// Now send a pane event — the deferred action should fire.
	paneSnap := tmux.PaneSnapshot{
		Panes: []tmux.Pane{
			{ID: "s:0.0", PaneID: "%1", Session: "main", Current: true},
		},
		CurrentID:      "s:0.0",
		CurrentLabel:   "s:0.0: test",
		IncludeCurrent: true,
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindPanes, Data: paneSnap}})

	if h.Model().deferredAction != nil {
		t.Fatal("deferredAction should be nil after pane event")
	}
	if h.Model().mode != ModePaneCaptureForm {
		t.Fatalf("mode = %v, want ModePaneCaptureForm", h.Model().mode)
	}
}

func TestRootMenuLeafActionHeaderUsesParentSegment(t *testing.T) {
	// When --root-menu launches a leaf action like pane:capture, the root
	// title should be the parent segment ("pane"), not the full colon-
	// separated ID ("pane:capture"). Otherwise the breadcrumb shows
	// "pane:capture→capture to file" instead of "pane→capture to file".
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "pane:capture"})
	if m.rootTitle != "pane" {
		t.Fatalf("rootTitle = %q, want %q", m.rootTitle, "pane")
	}
}

func TestRootMenuLeafActionHeaderSessionSave(t *testing.T) {
	// session:save is also a leaf action — root title should be "session".
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "session:save"})
	if m.rootTitle != "session" {
		t.Fatalf("rootTitle = %q, want %q", m.rootTitle, "session")
	}
}

func TestRootMenuLoaderHeaderUsesSegment(t *testing.T) {
	// Loader-based root menus (like session:tree) should also produce
	// a clean header segment without the colon prefix.
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "session:tree"})
	if m.rootTitle != "tree" {
		t.Fatalf("rootTitle = %q, want %q", m.rootTitle, "tree")
	}
}

func TestRootMenuLeafActionContextIsEmptyBeforeBackend(t *testing.T) {
	// Verify that context has empty CurrentPaneID before backend data.
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	ctx := m.menuContext()
	if ctx.CurrentPaneID != "" {
		t.Fatalf("expected empty CurrentPaneID before backend data, got %q", ctx.CurrentPaneID)
	}
}

func TestLayoutPreviewNoRevertWhenDataEmpty(t *testing.T) {
	var applied []string
	old := layoutPreviewFn
	layoutPreviewFn = func(_, layout string) error {
		applied = append(applied, layout)
		return nil
	}
	defer func() { layoutPreviewFn = old }()

	m := NewModel(ModelConfig{SocketPath: "test.sock", Width: 80, Height: 24})
	items := []menu.Item{
		{ID: "even-horizontal", Label: "Even Horizontal"},
	}
	lvl := newLevel("window:layout", "Layout", items, nil)
	lvl.Data = "" // empty string = no original layout known
	m.stack = append(m.stack, lvl)

	h := NewHarness(m)
	applied = nil
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	if len(applied) != 0 {
		t.Fatalf("expected no revert when Data is empty, got %v", applied)
	}
}

func TestRootMenuSessionRenameDeferredWithMenuArgs(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "session:rename", MenuArgs: "mysession"})
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set when session:rename has menuArgs")
	}
	if m.deferredRename.ID != "session:rename" {
		t.Fatalf("deferredRename.ID = %q, want session:rename", m.deferredRename.ID)
	}
	if !m.loading {
		t.Fatal("expected loading=true while deferred rename is pending")
	}
	if m.rootMenuID != "session:rename" {
		t.Fatalf("rootMenuID = %q, want session:rename", m.rootMenuID)
	}
	if m.rootTitle != "session" {
		t.Fatalf("rootTitle = %q, want session", m.rootTitle)
	}
}

func TestRootMenuWindowRenameDeferredWithMenuArgs(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "window:rename", MenuArgs: "main:0"})
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set when window:rename has menuArgs")
	}
	if m.deferredRename.ID != "window:rename" {
		t.Fatalf("deferredRename.ID = %q, want window:rename", m.deferredRename.ID)
	}
	if !m.loading {
		t.Fatal("expected loading=true while deferred rename is pending")
	}
}

func TestDeferredSessionRenameFiresOnSessionsUpdated(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "session:rename", MenuArgs: "main"})
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set")
	}

	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{
			{Name: "main", Label: "main: 2 windows"},
			{Name: "other", Label: "other: 1 window"},
		},
		Current: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})

	if h.Model().deferredRename != nil {
		t.Fatal("expected deferredRename cleared after SessionsUpdated")
	}
	if h.Model().mode != ModeSessionForm {
		t.Fatalf("mode = %v, want ModeSessionForm", h.Model().mode)
	}
	if h.Model().sessionForm == nil {
		t.Fatal("expected sessionForm to be set")
	}
	if h.Model().sessionForm.Target() != "main" {
		t.Fatalf("sessionForm target = %q, want main", h.Model().sessionForm.Target())
	}
	if h.Model().loading {
		t.Fatal("expected loading=false after form opens")
	}
}

func TestDeferredWindowRenameFiresOnWindowsUpdated(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "window:rename", MenuArgs: "main:0"})
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set")
	}

	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Send sessions first — should NOT trigger the deferred window rename.
	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}},
		Current:  "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})
	if h.Model().deferredRename == nil {
		t.Fatal("deferredRename should still be pending after session event")
	}

	// Now send windows — should trigger the deferred window rename.
	winSnap := tmux.WindowSnapshot{
		Windows: []tmux.Window{
			{ID: "main:0", Session: "main", Name: "vim", Label: "0: vim", Current: true},
			{ID: "main:1", Session: "main", Name: "zsh", Label: "1: zsh"},
		},
		CurrentID:      "main:0",
		CurrentSession: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: winSnap}})

	if h.Model().deferredRename != nil {
		t.Fatal("expected deferredRename cleared after WindowsUpdated")
	}
	if h.Model().mode != ModeWindowForm {
		t.Fatalf("mode = %v, want ModeWindowForm", h.Model().mode)
	}
	if h.Model().windowForm == nil {
		t.Fatal("expected windowForm to be set")
	}
	if h.Model().windowForm.Target() != "main:0" {
		t.Fatalf("windowForm target = %q, want main:0", h.Model().windowForm.Target())
	}
	if h.Model().loading {
		t.Fatal("expected loading=false after form opens")
	}
}

func TestDeferredWindowRenameResolvesWindowName(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "window:rename", MenuArgs: "main:0"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Provide sessions first.
	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}},
		Current:  "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})

	// Provide windows with a name for main:0.
	winSnap := tmux.WindowSnapshot{
		Windows: []tmux.Window{
			{ID: "main:0", Session: "main", Name: "editor", Label: "0: editor", Current: true},
		},
		CurrentID:      "main:0",
		CurrentSession: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: winSnap}})

	if h.Model().windowForm == nil {
		t.Fatal("expected windowForm to be set")
	}
	// The form's initial value should be the resolved window name, not the ID.
	got := h.Model().windowForm.Value()
	if got != "editor" {
		t.Fatalf("windowForm initial value = %q, want %q", got, "editor")
	}
}

func TestDeferredSessionRenameEscapeQuits(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "session:rename", MenuArgs: "main"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}},
		Current:  "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})
	if h.Model().mode != ModeSessionForm {
		t.Fatalf("mode = %v, want ModeSessionForm", h.Model().mode)
	}

	// Pressing escape in a directly-invoked rename form should quit,
	// not return to the root menu.
	_, cmd := h.Model().handleSessionForm(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected quit command on escape from directly-invoked session rename")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestDeferredWindowRenameEscapeQuits(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "window:rename", MenuArgs: "main:0"})
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}},
		Current:  "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})

	winSnap := tmux.WindowSnapshot{
		Windows: []tmux.Window{
			{ID: "main:0", Session: "main", Name: "vim", Label: "0: vim", Current: true},
		},
		CurrentID:      "main:0",
		CurrentSession: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: winSnap}})
	if h.Model().mode != ModeWindowForm {
		t.Fatalf("mode = %v, want ModeWindowForm", h.Model().mode)
	}

	_, cmd := h.Model().handleWindowForm(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected quit command on escape from directly-invoked window rename")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestRootMenuSessionRenameWithoutMenuArgsFallsThrough(t *testing.T) {
	// When menuArgs is empty, session:rename should load the picker list
	// via the standard loader path, not defer.
	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "session:rename"})
	if m.deferredRename != nil {
		t.Fatal("expected deferredRename to be nil when menuArgs is empty")
	}
}
