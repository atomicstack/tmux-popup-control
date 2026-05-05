package ui

import (
	"errors"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
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
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		capturedTarget = pane
		return tmux.PanePreviewData{
			Lines:         []string{"line1", "line2"},
			RawANSI:       true,
			CursorVisible: true,
			CursorX:       1,
			CursorY:       0,
		}, nil
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
	if !data.cursorVisible || data.cursorX != 1 || data.cursorY != 0 {
		t.Fatalf("expected preview cursor metadata, got %+v", data)
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
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		capturedTarget = pane
		return tmux.PanePreviewData{
			Lines:         []string{"vim output"},
			RawANSI:       true,
			CursorVisible: true,
			CursorX:       2,
			CursorY:       0,
		}, nil
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

func TestSessionPreviewUsesSessionActivePaneFromTopology(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.windows.SetEntries([]menu.WindowEntry{
		{ID: "dev:0", Session: "dev", Index: 0, Name: "main", Current: false},
		{ID: "dev:1", Session: "dev", Index: 1, Name: "logs", Current: true},
	})
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "dev:0.0", Session: "dev", Window: "main", WindowIdx: 0, Index: 0},
		{ID: "dev:1.0", Session: "dev", Window: "logs", WindowIdx: 1, Index: 0},
		{ID: "dev:1.1", Session: "dev", Window: "logs", WindowIdx: 1, Index: 1},
	})

	topologyFetchCalls := 0
	oldTopology := fetchPreviewTopologyFn
	fetchPreviewTopologyFn = func(string) (tmux.PreviewTopology, error) {
		topologyFetchCalls++
		return tmux.PreviewTopology{
			SessionActivePaneIDs: map[string]string{"dev": "dev:1.1"},
			WindowActivePaneIDs:  map[string]string{"dev:1": "dev:1.1"},
		}, nil
	}
	capturedTarget := ""
	old := panePreviewFn
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		capturedTarget = pane
		return tmux.PanePreviewData{Lines: []string{"topology"}}, nil
	}
	defer func() {
		panePreviewFn = old
		fetchPreviewTopologyFn = oldTopology
	}()

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	_ = cmd()

	if capturedTarget != "dev:1.1" {
		t.Fatalf("expected topology-resolved active pane dev:1.1, got %q", capturedTarget)
	}
	if topologyFetchCalls != 1 {
		t.Fatalf("expected topology fetch helper to be called once, got %d", topologyFetchCalls)
	}
}

func TestWindowPreviewUsesWindowActivePaneFromTopology(t *testing.T) {
	lvl := newLevel("window:switch", "Windows", []menu.Item{{ID: "dev:1", Label: "main"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "dev:1.0", Session: "dev", Window: "main", WindowIdx: 1, Index: 0},
		{ID: "dev:1.1", Session: "dev", Window: "main", WindowIdx: 1, Index: 1},
	})

	oldTopology := fetchPreviewTopologyFn
	fetchPreviewTopologyFn = func(string) (tmux.PreviewTopology, error) {
		return tmux.PreviewTopology{
			WindowActivePaneIDs: map[string]string{"dev:1": "dev:1.1"},
		}, nil
	}
	capturedTarget := ""
	old := panePreviewFn
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		capturedTarget = pane
		return tmux.PanePreviewData{Lines: []string{"topology"}}, nil
	}
	defer func() {
		panePreviewFn = old
		fetchPreviewTopologyFn = oldTopology
	}()

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	_ = cmd()

	if capturedTarget != "dev:1.1" {
		t.Fatalf("expected topology-resolved active pane dev:1.1, got %q", capturedTarget)
	}
}

func TestTreeSessionPreviewUsesSessionActivePaneFromTopology(t *testing.T) {
	lvl := newLevel("session:tree", "tree", []menu.Item{{ID: menu.TreeSessionID("dev"), Label: "dev"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.windows.SetEntries([]menu.WindowEntry{
		{ID: "dev:0", Session: "dev", Index: 0, Name: "main", Current: false},
		{ID: "dev:1", Session: "dev", Index: 1, Name: "logs", Current: true},
	})
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "dev:0.0", Session: "dev", Window: "main", WindowIdx: 0, Index: 0},
		{ID: "dev:1.0", Session: "dev", Window: "logs", WindowIdx: 1, Index: 0},
		{ID: "dev:1.1", Session: "dev", Window: "logs", WindowIdx: 1, Index: 1},
	})

	oldTopology := fetchPreviewTopologyFn
	fetchPreviewTopologyFn = func(string) (tmux.PreviewTopology, error) {
		return tmux.PreviewTopology{
			SessionActivePaneIDs: map[string]string{"dev": "dev:1.1"},
		}, nil
	}
	capturedTarget := ""
	old := panePreviewFn
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		capturedTarget = pane
		return tmux.PanePreviewData{Lines: []string{"topology"}}, nil
	}
	defer func() {
		panePreviewFn = old
		fetchPreviewTopologyFn = oldTopology
	}()

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected preview command")
	}
	_ = cmd()

	if capturedTarget != "dev:1.1" {
		t.Fatalf("expected topology-resolved active pane dev:1.1, got %q", capturedTarget)
	}
}

func TestPreviewTopologyCacheReusedWhileLevelLoading(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.windows.SetEntries([]menu.WindowEntry{
		{ID: "dev:0", Session: "dev", Index: 0, Name: "main", Current: false},
		{ID: "dev:1", Session: "dev", Index: 1, Name: "logs", Current: true},
	})
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "dev:0.0", Session: "dev", Window: "main", WindowIdx: 0, Index: 0},
		{ID: "dev:1.0", Session: "dev", Window: "logs", WindowIdx: 1, Index: 0},
		{ID: "dev:1.1", Session: "dev", Window: "logs", WindowIdx: 1, Index: 1},
	})

	topologyCalls := 0
	oldTopology := fetchPreviewTopologyFn
	fetchPreviewTopologyFn = func(string) (tmux.PreviewTopology, error) {
		topologyCalls++
		return tmux.PreviewTopology{
			SessionActivePaneIDs: map[string]string{"dev": "dev:1.1"},
			WindowActivePaneIDs:  map[string]string{"dev:1": "dev:1.1"},
		}, nil
	}
	old := panePreviewFn
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		return tmux.PanePreviewData{Lines: []string{"topology"}}, nil
	}
	defer func() {
		panePreviewFn = old
		fetchPreviewTopologyFn = oldTopology
	}()

	if cmd := m.ensurePreviewForLevel(lvl); cmd == nil {
		t.Fatalf("expected first preview command")
	} else {
		_ = cmd()
	}
	if cmd := m.ensurePreviewForLevel(lvl); cmd != nil {
		t.Fatalf("expected second ensurePreviewForLevel call to reuse the in-flight preview and return nil")
	}

	if topologyCalls != 1 {
		t.Fatalf("expected topology fetch helper to be called once while the level is loading, got %d calls", topologyCalls)
	}
}

func TestPreviewTopologyFetchErrorRetriedForSameLevel(t *testing.T) {
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "dev:1.0", Session: "dev", Window: "logs", WindowIdx: 1, Index: 0, Current: true},
		{ID: "dev:1.1", Session: "dev", Window: "logs", WindowIdx: 1, Index: 1, Current: false},
	})

	topologyCalls := 0
	oldTopology := fetchPreviewTopologyFn
	fetchPreviewTopologyFn = func(string) (tmux.PreviewTopology, error) {
		topologyCalls++
		if topologyCalls == 1 {
			return tmux.PreviewTopology{}, errors.New("tmux unavailable")
		}
		return tmux.PreviewTopology{
			SessionActivePaneIDs: map[string]string{"dev": "dev:1.1"},
		}, nil
	}
	var capturedTargets []string
	oldPanePreview := panePreviewFn
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		capturedTargets = append(capturedTargets, pane)
		return tmux.PanePreviewData{Lines: []string{"preview"}}, nil
	}
	defer func() {
		fetchPreviewTopologyFn = oldTopology
		panePreviewFn = oldPanePreview
	}()

	cmd := m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected first preview command")
	}
	rawMsg := cmd()
	msg, ok := rawMsg.(previewLoadedMsg)
	if !ok {
		t.Fatalf("expected first previewLoadedMsg, got %T", rawMsg)
	}
	m.handlePreviewLoadedMsg(msg)

	cmd = m.ensurePreviewForLevel(lvl)
	if cmd == nil {
		t.Fatalf("expected second preview command")
	}
	rawMsg = cmd()
	msg, ok = rawMsg.(previewLoadedMsg)
	if !ok {
		t.Fatalf("expected second previewLoadedMsg, got %T", rawMsg)
	}
	m.handlePreviewLoadedMsg(msg)

	if topologyCalls != 2 {
		t.Fatalf("expected topology fetch to retry after error, got %d calls", topologyCalls)
	}
	if len(capturedTargets) != 2 {
		t.Fatalf("expected 2 pane captures, got %v", capturedTargets)
	}
	if capturedTargets[0] != "dev:1.0" {
		t.Fatalf("expected first capture to use pane-store fallback, got %q", capturedTargets[0])
	}
	if capturedTargets[1] != "dev:1.1" {
		t.Fatalf("expected second capture to use retried topology result, got %q", capturedTargets[1])
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

func TestHandlePreviewLoadedMsgAnchorsPaneScrollToLastContentRow(t *testing.T) {
	lvl := newLevel("pane:switch", "Panes", []menu.Item{{ID: "dev:1.0", Label: "Pane"}}, nil)
	m := &Model{
		stack: []*level{lvl},
		preview: map[string]*previewData{
			"pane:switch": {target: "dev:1.0", seq: 1},
		},
	}
	msg := previewLoadedMsg{
		levelID:       "pane:switch",
		kind:          previewKindPane,
		target:        "dev:1.0",
		seq:           1,
		lines:         []string{"only-line", ""},
		rawANSI:       true,
		cursorVisible: true,
		cursorX:       0,
		cursorY:       1,
	}

	m.handlePreviewLoadedMsg(msg)
	data := m.activePreview()
	if data == nil {
		t.Fatal("expected preview data")
	}
	if data.scrollOffset != 0 {
		t.Fatalf("expected pane preview scroll anchored to last content row, got %+v", data)
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

// TestMaxVisibleItemsRespectsNoPreview verifies that with noPreview=true the
// function does not reserve rows for a preview, since viewVertical will not
// render one. Without this guarantee the tree level shrinks to a tiny
// viewport and the cursor (auto-positioned at the active pane via the
// session-tree initial-selection feature) drags the viewport off the top of
// the tree, hiding earlier items from pane-capture-based assertions.
func TestMaxVisibleItemsRespectsNoPreview(t *testing.T) {
	m := NewModel(ModelConfig{Height: 24, NoPreview: true})
	lvl := newLevel("session:tree", "Tree", []menu.Item{
		{ID: "tree:s:a", Label: "a"},
		{ID: "tree:s:b", Label: "b"},
	}, nil)
	m.stack = []*level{lvl}

	// Inject a loaded preview to mimic the async preview-load racing the
	// initial render. With noPreview=true the model must ignore it.
	m.preview["session:tree"] = &previewData{
		target: "tree:s:a",
		lines:  []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
		seq:    1,
	}

	got := m.maxVisibleItems()

	// With Height=24, bottomBarRows=2, header=1, and no preview reserve, the
	// budget should be at least 20 rows. If the bug is present the loaded
	// preview shrinks the budget below 15.
	if got < 15 {
		t.Fatalf("expected maxVisibleItems >= 15 when noPreview=true, got %d", got)
	}
}
