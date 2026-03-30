package ui

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// TestEnsurePreviewForLevelFallsBackToWindowList covers the session preview
// path when no pane data is cached: it should fall back to a window list
// derived from the window store.
func TestEnsurePreviewForLevelFallsBackToWindowList(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.windows.SetEntries([]menu.WindowEntry{{Session: "dev", Index: 1, Name: "main"}})

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	msg := cmd()
	previewMsg, ok := msg.(previewLoadedMsg)
	if !ok {
		t.Fatalf("expected previewLoadedMsg, got %T", msg)
	}
	m.handlePreviewLoadedMsg(previewMsg)
	data := m.activePreview()
	if data == nil {
		t.Fatalf("expected preview data to be populated")
	}
	if len(data.lines) == 0 {
		t.Fatalf("expected preview lines, got %#v", data.lines)
	}
	if data.loading {
		t.Fatalf("expected loading to be false")
	}
}

// TestSessionPreviewUsesPaneCaptureWhenPanesAvailable checks that, when pane
// data is cached for the target session, ensurePreviewForLevel fires an async
// capture-pane command rather than the window-list fallback.
func TestSessionPreviewUsesPaneCaptureWhenPanesAvailable(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "dev:1.0", Session: "dev", Window: "main", WindowIdx: 1, Index: 0, Current: true},
	})

	capturedTarget := ""
	old := panePreviewFn
	panePreviewFn = func(_, pane string) ([]string, error) {
		capturedTarget = pane
		return []string{"line1", "line2"}, nil
	}
	defer func() { panePreviewFn = old }()

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	msg := cmd()
	previewMsg, ok := msg.(previewLoadedMsg)
	if !ok {
		t.Fatalf("expected previewLoadedMsg, got %T", msg)
	}
	if capturedTarget != "dev:1.0" {
		t.Fatalf("expected pane capture target dev:1.0, got %q", capturedTarget)
	}
	m.handlePreviewLoadedMsg(previewMsg)
	data := m.activePreview()
	if data == nil {
		t.Fatalf("expected preview data")
	}
	if len(data.lines) != 2 {
		t.Fatalf("expected 2 preview lines, got %d: %v", len(data.lines), data.lines)
	}
}

// TestWindowPreviewUsesPaneCaptureWhenPanesAvailable mirrors the session test
// for the window:switch level.
func TestWindowPreviewUsesPaneCaptureWhenPanesAvailable(t *testing.T) {
	lvl := newLevel("window:switch", "Windows", []menu.Item{{ID: "dev:1", Label: "main"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "dev:1.0", Session: "dev", Window: "main", WindowIdx: 1, Index: 0, Current: false},
		{ID: "dev:1.1", Session: "dev", Window: "main", WindowIdx: 1, Index: 1, Current: true},
	})

	capturedTarget := ""
	old := panePreviewFn
	panePreviewFn = func(_, pane string) ([]string, error) {
		capturedTarget = pane
		return []string{"vim output"}, nil
	}
	defer func() { panePreviewFn = old }()

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	msg := cmd()
	if _, ok := msg.(previewLoadedMsg); !ok {
		t.Fatalf("expected previewLoadedMsg, got %T", msg)
	}
	// Active pane (Current=true) should be preferred over the first pane.
	if capturedTarget != "dev:1.1" {
		t.Fatalf("expected pane capture target dev:1.1, got %q", capturedTarget)
	}
}

func TestHandlePreviewLoadedMsgIgnoresStaleResponses(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := &Model{
		stack: []*level{lvl},
		preview: map[string]*previewData{
			"session:switch": {target: "dev", seq: 2},
		},
	}
	msg := previewLoadedMsg{
		levelID: "session:switch",
		target:  "dev",
		seq:     1,
		lines:   []string{"old"},
	}
	m.handlePreviewLoadedMsg(msg)
	data := m.activePreview()
	if data.lines != nil {
		t.Fatalf("expected stale message to be ignored, got %+v", data)
	}
}

func TestLayoutPreviewAppliesOnCursorMove(t *testing.T) {
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
		{ID: "even-vertical", Label: "Even Vertical"},
		{ID: "tiled", Label: "Tiled"},
		{ID: "bb62,159x48", Label: "current layout"},
	}
	lvl := newLevel("window:layout", "Layout", items, nil)
	lvl.Cursor = 0 // simulate user moving cursor to first item
	m.stack = append(m.stack, lvl)

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatal("expected preview command")
	}
	msg := cmd()
	if _, ok := msg.(layoutAppliedMsg); !ok {
		t.Fatalf("expected layoutAppliedMsg, got %T", msg)
	}
	if len(applied) == 0 || applied[0] != "even-horizontal" {
		t.Fatalf("expected even-horizontal applied, got %v", applied)
	}
}

func TestLayoutPreviewSavesOriginalLayout(t *testing.T) {
	old := layoutPreviewFn
	layoutPreviewFn = func(_, _ string) error { return nil }
	defer func() { layoutPreviewFn = old }()

	m := NewModel(ModelConfig{SocketPath: "test.sock", Width: 80, Height: 24})
	items := []menu.Item{
		{ID: "even-horizontal", Label: "Even Horizontal"},
		{ID: "bb62,159x48", Label: "current layout"},
	}
	lvl := newLevel("window:layout", "Layout", items, nil)
	lvl.Cursor = 0
	m.stack = append(m.stack, lvl)

	m.ensurePreviewForLevel(lvl)

	original, ok := lvl.Data.(string)
	if !ok {
		t.Fatalf("expected level.Data to be string, got %T", lvl.Data)
	}
	if original != "bb62,159x48" {
		t.Fatalf("expected original layout bb62,159x48, got %q", original)
	}
}

func TestLayoutPreviewSkipsDuplicateApply(t *testing.T) {
	var count int
	old := layoutPreviewFn
	layoutPreviewFn = func(_, _ string) error { count++; return nil }
	defer func() { layoutPreviewFn = old }()

	m := NewModel(ModelConfig{SocketPath: "test.sock", Width: 80, Height: 24})
	items := []menu.Item{
		{ID: "even-horizontal", Label: "Even Horizontal"},
	}
	lvl := newLevel("window:layout", "Layout", items, nil)
	lvl.Cursor = 0
	m.stack = append(m.stack, lvl)

	cmd1 := m.ensurePreviewForLevel(lvl)
	if cmd1 != nil {
		msg := cmd1()
		m.handleLayoutAppliedMsg(msg)
	}

	// Second call with same cursor — should still issue command (loading is false now)
	// but the target check prevents re-issue if loading is still true
	cmd2 := m.ensurePreviewForLevel(lvl)
	if cmd2 != nil {
		cmd2()
	}
	// Both should fire since loading was cleared between calls
	if count != 2 {
		t.Fatalf("expected 2 calls, got %d", count)
	}
}

func TestLayoutPreviewNoSidePanelRendered(t *testing.T) {
	m := NewModel(ModelConfig{SocketPath: "test.sock", Width: 120, Height: 24})
	items := []menu.Item{
		{ID: "even-horizontal", Label: "Even Horizontal"},
	}
	lvl := newLevel("window:layout", "Layout", items, nil)
	m.stack = []*level{lvl}

	if m.hasSidePreview() {
		t.Fatal("expected no side preview for window:layout")
	}
}

// TestMaxVisibleItemsAccountsForPreview verifies that the item viewport
// shrinks to make room for an active preview block.
func TestMaxVisibleItemsAccountsForPreview(t *testing.T) {
	m := NewModel(ModelConfig{Height: 20})
	lvl := newLevel("session:switch", "Sessions", []menu.Item{
		{ID: "s1", Label: "s1"},
		{ID: "s2", Label: "s2"},
	}, nil)
	m.stack = []*level{lvl}

	// Without any preview loaded the function should reserve 3 lines (blank +
	// title + loading placeholder) for the incoming preview.
	without := m.maxVisibleItems()

	// Inject a loaded preview with 5 content lines.
	m.preview["session:switch"] = &previewData{
		target: "s1",
		lines:  []string{"a", "b", "c", "d", "e"},
		seq:    1,
	}
	with := m.maxVisibleItems()

	// The loaded preview (blank + title + 5 lines = 7) is taller than the
	// loading reservation (3 lines), so the item budget must shrink.
	if with >= without {
		t.Fatalf("expected item count to shrink when preview is loaded: without=%d with=%d", without, with)
	}
}
