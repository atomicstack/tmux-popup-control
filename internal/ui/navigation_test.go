package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestHandleEscapeKeyFromRootQuits(t *testing.T) {
	m := NewModel("", 0, 0, false, false, nil, "", "", "", "")
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
	m := NewModel("", 0, 0, false, false, nil, "", "", "", "")
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

	m := NewModel("test.sock", 80, 24, false, false, nil, "", "", "", "")
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

func TestRootMenuLeafActionDeferredUntilBackendData(t *testing.T) {
	// When --root-menu specifies a leaf action (like pane:capture), the
	// action must be deferred until the backend provides data. Otherwise
	// ctx.CurrentPaneID is empty and the action fails with "no current pane".
	m := NewModel("", 80, 24, false, false, nil, "pane:capture", "", "", "")
	if m.deferredAction == nil {
		t.Fatal("expected deferredAction to be set for leaf action root menu")
	}
	if m.rootMenuID != "pane:capture" {
		t.Fatalf("rootMenuID = %q, want pane:capture", m.rootMenuID)
	}

	// Simulate a backend pane event with a current pane.
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	paneSnap := tmux.PaneSnapshot{
		Panes: []tmux.Pane{
			{ID: "s:0.0", PaneID: "%1", Session: "main", Current: true},
		},
		CurrentID:      "s:0.0",
		CurrentLabel:   "s:0.0: test",
		IncludeCurrent: true,
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindPanes, Data: paneSnap}})

	// After the backend event, the deferred action should have fired and
	// produced a PaneCapturePrompt, switching us to capture form mode.
	if h.Model().deferredAction != nil {
		t.Fatal("deferredAction should be nil after backend event")
	}
	if h.Model().mode != ModePaneCaptureForm {
		t.Fatalf("mode = %v, want ModePaneCaptureForm", h.Model().mode)
	}
}

func TestRootMenuLeafActionFailsWithoutBackendData(t *testing.T) {
	// Verify that without the deferred mechanism, a leaf action launched
	// directly would see an empty CurrentPaneID.
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
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

	m := NewModel("test.sock", 80, 24, false, false, nil, "", "", "", "")
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
