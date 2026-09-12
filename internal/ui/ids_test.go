package ui

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// Previews capture panes over control mode; the capture target must be the
// pane's %N id, never the "session:index.pane" display id, because tmux
// next-3.8 allows ':' and '.' inside session and window names.

func weirdPreviewModel(levelID string, items []menu.Item) *Model {
	lvl := newLevel(levelID, "level", items, nil)
	m := NewModel(ModelConfig{})
	m.stack = []*level{lvl}
	m.preview = make(map[string]*previewData)
	m.sessions.SetEntries([]menu.SessionEntry{{Name: "we:ird", ID: "$3", Label: "we:ird"}})
	m.windows.SetEntries([]menu.WindowEntry{
		{ID: "we:ird:1", Session: "we:ird", SessionID: "$3", Index: 1, Name: "do.tted", InternalID: "@7", Current: true},
	})
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "we:ird:1.0", PaneID: "%9", SessionID: "$3", WindowID: "@7", Session: "we:ird", Window: "do.tted", WindowIdx: 1, Index: 0, Current: true},
		{ID: "we:ird:1.1", PaneID: "%10", SessionID: "$3", WindowID: "@7", Session: "we:ird", Window: "do.tted", WindowIdx: 1, Index: 1},
	})
	return m
}

func stubPreviewCapture(t *testing.T, topology tmux.PreviewTopology) *string {
	t.Helper()
	captured := new(string)
	oldPreview := panePreviewFn
	oldTopology := fetchPreviewTopologyFn
	panePreviewFn = func(_, pane string) (tmux.PanePreviewData, error) {
		*captured = pane
		return tmux.PanePreviewData{Lines: []string{"x"}}, nil
	}
	fetchPreviewTopologyFn = func(string) (tmux.PreviewTopology, error) { return topology, nil }
	t.Cleanup(func() {
		panePreviewFn = oldPreview
		fetchPreviewTopologyFn = oldTopology
	})
	return captured
}

func runPreview(t *testing.T, m *Model) {
	t.Helper()
	cmd := m.ensurePreviewForLevel(m.currentLevel())
	if cmd == nil {
		t.Fatal("expected preview command")
	}
	_ = cmd()
}

func TestPaneSwitchPreviewCapturesByPaneID(t *testing.T) {
	m := weirdPreviewModel("pane:switch", []menu.Item{{ID: "we:ird:1.1", Label: "vim"}})
	captured := stubPreviewCapture(t, tmux.PreviewTopology{})
	runPreview(t, m)
	if *captured != "%10" {
		t.Fatalf("expected capture target %%10, got %q", *captured)
	}
}

func TestSessionSwitchPreviewFallbackCapturesByPaneID(t *testing.T) {
	m := weirdPreviewModel("session:switch", []menu.Item{{ID: "we:ird", Label: "we:ird"}})
	captured := stubPreviewCapture(t, tmux.PreviewTopology{})
	runPreview(t, m)
	if *captured != "%9" {
		t.Fatalf("expected current pane %%9 as capture target, got %q", *captured)
	}
}

func TestWindowSwitchPreviewFallbackCapturesByPaneID(t *testing.T) {
	m := weirdPreviewModel("window:switch", []menu.Item{{ID: "we:ird:1", Label: "do.tted"}})
	captured := stubPreviewCapture(t, tmux.PreviewTopology{})
	runPreview(t, m)
	if *captured != "%9" {
		t.Fatalf("expected current pane %%9 as capture target, got %q", *captured)
	}
}

func TestTreePanePreviewCapturesByPaneID(t *testing.T) {
	m := weirdPreviewModel("session:tree", []menu.Item{{ID: menu.TreePaneID("%10"), Label: "1: vim"}})
	captured := stubPreviewCapture(t, tmux.PreviewTopology{})
	runPreview(t, m)
	if *captured != "%10" {
		t.Fatalf("expected capture target %%10, got %q", *captured)
	}
}

func TestTreeWindowPreviewResolvesInternalID(t *testing.T) {
	m := weirdPreviewModel("session:tree", []menu.Item{{ID: menu.TreeWindowID("@7"), Label: "1: do.tted"}})
	captured := stubPreviewCapture(t, tmux.PreviewTopology{
		WindowActivePaneIDs: map[string]string{"we:ird:1": "%10"},
	})
	runPreview(t, m)
	if *captured != "%10" {
		t.Fatalf("expected topology-resolved pane %%10, got %q", *captured)
	}
}

func TestTreeSessionPreviewResolvesSessionID(t *testing.T) {
	m := weirdPreviewModel("session:tree", []menu.Item{{ID: menu.TreeSessionID("$3"), Label: "we:ird"}})
	captured := stubPreviewCapture(t, tmux.PreviewTopology{
		SessionActivePaneIDs: map[string]string{"we:ird": "%10"},
	})
	runPreview(t, m)
	if *captured != "%10" {
		t.Fatalf("expected topology-resolved pane %%10, got %q", *captured)
	}
}

func TestInitialSessionTreeCursorUsesInternalIDs(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24, MenuArgs: "expanded"})
	sessions := []menu.SessionEntry{{Name: "we:ird", ID: "$3", Label: "we:ird", Current: true}}
	windows := []menu.WindowEntry{
		{ID: "we:ird:0", Session: "we:ird", SessionID: "$3", Index: 0, Name: "shell", InternalID: "@6"},
		{ID: "we:ird:1", Session: "we:ird", SessionID: "$3", Index: 1, Name: "do.tted", InternalID: "@7", Current: true},
	}
	panes := []menu.PaneEntry{
		{ID: "we:ird:1.0", PaneID: "%9", SessionID: "$3", WindowID: "@7", Session: "we:ird", WindowIdx: 1, Index: 0},
		{ID: "we:ird:1.1", PaneID: "%10", SessionID: "$3", WindowID: "@7", Session: "we:ird", WindowIdx: 1, Index: 1, Current: true},
	}
	seedSessionTreeStores(m, sessions, windows, panes, "we:ird", "we:ird:1", "1: do.tted", "we:ird", "we:ird:1.1", "1: vim")
	items := buildSessionTreeItems(true, sessions, windows, panes)
	if got := m.initialSessionTreeCursor(items); got < 0 || items[got].ID != menu.TreePaneID("%10") {
		t.Fatalf("expected cursor on tree:p:%%10, got index %d (%v)", got, items)
	}
}
