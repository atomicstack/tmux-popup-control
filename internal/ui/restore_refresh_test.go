package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

func TestHandleCategoryLoadedMsgStartsRestoreRefresh(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", dir)

	restoreSchedule := restoreRefreshScheduleFn
	scheduleCalls := 0
	restoreRefreshScheduleFn = func() tea.Cmd {
		scheduleCalls++
		return nil
	}
	defer func() { restoreRefreshScheduleFn = restoreSchedule }()

	m := NewModel(ModelConfig{Width: 80, Height: 24})
	m.loading = true
	m.pendingID = "session:restore-from"

	cmd := m.handleCategoryLoadedMsg(categoryLoadedMsg{
		id:    "session:restore-from",
		title: "restore-from",
		items: []menu.Item{
			{Label: "name  type", Header: true},
			{ID: "/tmp/save.json", Label: "save"},
		},
	})

	if cmd != nil {
		t.Fatalf("expected no command when only restore refresh scheduling is active, got %T", cmd)
	}
	if scheduleCalls != 1 {
		t.Fatalf("expected one restore refresh schedule call, got %d", scheduleCalls)
	}
	if m.restoreRefresh == nil {
		t.Fatal("expected restore refresh state to be initialized")
	}
	if m.restoreRefresh.dir != dir {
		t.Fatalf("restore refresh dir = %q, want %q", m.restoreRefresh.dir, dir)
	}
	if m.restoreRefresh.lastModTime.IsZero() {
		t.Fatal("expected restore refresh mod time to be recorded")
	}
}

func TestHandleRestoreRefreshTickMsgSkipsReloadWhenModTimeUnchanged(t *testing.T) {
	ts := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)

	restoreStat := restoreRefreshStatFn
	restoreRefreshStatFn = func(string) (time.Time, error) { return ts, nil }
	defer func() { restoreRefreshStatFn = restoreStat }()

	restoreSchedule := restoreRefreshScheduleFn
	scheduleCalls := 0
	restoreRefreshScheduleFn = func() tea.Cmd {
		scheduleCalls++
		return nil
	}
	defer func() { restoreRefreshScheduleFn = restoreSchedule }()

	m := NewModel(ModelConfig{Width: 80, Height: 24})
	lvl := newLevel("session:restore-from", "restore-from", []menu.Item{
		{Label: "name  type", Header: true},
		{ID: "keep", Label: "keep"},
	}, nil)
	m.stack = []*level{lvl}
	m.restoreRefresh = &restoreRefreshState{dir: "/tmp/saves", lastModTime: ts}

	cmd := m.handleRestoreRefreshTickMsg(restoreRefreshTickMsg{})

	if cmd != nil {
		t.Fatalf("expected no reload command when mod time is unchanged, got %T", cmd)
	}
	if scheduleCalls != 1 {
		t.Fatalf("expected next restore refresh tick to be scheduled once, got %d", scheduleCalls)
	}
	if len(lvl.Items) != 2 || lvl.Items[1].ID != "keep" {
		t.Fatalf("expected restore list items to remain unchanged, got %#v", lvl.Items)
	}
}

func TestHandleRestoreRefreshTickMsgLoadsWhenModTimeChanges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", dir)

	path := filepath.Join(dir, "auto-2026-04-06T10-01-00_20260406T100100.json")
	if err := resurrect.WriteSaveFile(path, &resurrect.SaveFile{
		Version:   2,
		Timestamp: time.Date(2026, 4, 6, 10, 1, 0, 0, time.UTC),
		Name:      "auto-2026-04-06T10-01-00",
		Kind:      resurrect.SaveKindAuto,
		Sessions:  []resurrect.Session{{Name: "main"}},
	}); err != nil {
		t.Fatalf("WriteSaveFile: %v", err)
	}

	oldTime := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 4, 6, 10, 2, 0, 0, time.UTC)

	restoreStat := restoreRefreshStatFn
	restoreRefreshStatFn = func(string) (time.Time, error) { return newTime, nil }
	defer func() { restoreRefreshStatFn = restoreStat }()

	restoreSchedule := restoreRefreshScheduleFn
	restoreRefreshScheduleFn = func() tea.Cmd { return nil }
	defer func() { restoreRefreshScheduleFn = restoreSchedule }()

	m := NewModel(ModelConfig{Width: 80, Height: 24})
	lvl := newLevel("session:restore-from", "restore-from", nil, nil)
	m.stack = []*level{lvl}
	m.restoreRefresh = &restoreRefreshState{dir: dir, lastModTime: oldTime}

	cmd := m.handleRestoreRefreshTickMsg(restoreRefreshTickMsg{})
	if cmd == nil {
		t.Fatal("expected reload command when mod time changes")
	}

	msg := cmd()
	loaded, ok := msg.(restoreRefreshLoadedMsg)
	if !ok {
		t.Fatalf("expected restoreRefreshLoadedMsg, got %T", msg)
	}
	if loaded.dir != dir {
		t.Fatalf("loaded dir = %q, want %q", loaded.dir, dir)
	}
	if !loaded.modTime.Equal(newTime) {
		t.Fatalf("loaded mod time = %s, want %s", loaded.modTime, newTime)
	}
	if len(loaded.items) < 2 {
		t.Fatalf("expected header + save entry from reload, got %#v", loaded.items)
	}
}

func TestHandleRestoreRefreshLoadedMsgPreservesFilterAndCursor(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	lvl := newLevel("session:restore-from", "restore-from", []menu.Item{
		{Label: "name  type", Header: true},
		{ID: "alpha", Label: "keep alpha"},
		{ID: "beta", Label: "keep beta"},
	}, nil)
	lvl.Subtitle = "/tmp/old"
	lvl.SetFilter("keep", len([]rune("keep")))
	lvl.Cursor = lvl.IndexOf("beta")
	m.stack = []*level{lvl}
	m.restoreRefresh = &restoreRefreshState{
		dir:         "/tmp/new",
		lastModTime: time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
	}

	restoreSchedule := restoreRefreshScheduleFn
	scheduleCalls := 0
	restoreRefreshScheduleFn = func() tea.Cmd {
		scheduleCalls++
		return nil
	}
	defer func() { restoreRefreshScheduleFn = restoreSchedule }()

	cmd := m.handleRestoreRefreshLoadedMsg(restoreRefreshLoadedMsg{
		dir:      "/tmp/new",
		modTime:  time.Date(2026, 4, 6, 10, 5, 0, 0, time.UTC),
		subtitle: "/tmp/new",
		items: []menu.Item{
			{Label: "name  type", Header: true},
			{ID: "alpha", Label: "keep alpha updated"},
			{ID: "beta", Label: "keep beta updated"},
			{ID: "gamma", Label: "keep gamma"},
		},
	})

	if cmd != nil {
		t.Fatalf("expected no extra command after applying refresh, got %T", cmd)
	}
	if lvl.Filter != "keep" {
		t.Fatalf("filter = %q, want %q", lvl.Filter, "keep")
	}
	if lvl.FilterCursorPos() != len([]rune("keep")) {
		t.Fatalf("filter cursor = %d, want %d", lvl.FilterCursorPos(), len([]rune("keep")))
	}
	if lvl.Subtitle != "/tmp/new" {
		t.Fatalf("subtitle = %q, want %q", lvl.Subtitle, "/tmp/new")
	}
	if current := lvl.Items[lvl.Cursor].ID; current != "beta" {
		t.Fatalf("cursor now points to %q, want beta", current)
	}
	if !m.restoreRefresh.lastModTime.Equal(time.Date(2026, 4, 6, 10, 5, 0, 0, time.UTC)) {
		t.Fatalf("last mod time not updated: %s", m.restoreRefresh.lastModTime)
	}
	if scheduleCalls != 1 {
		t.Fatalf("expected one follow-up restore refresh schedule call, got %d", scheduleCalls)
	}
}

func TestHandleRestoreRefreshTickMsgStopsOutsideRestoreFrom(t *testing.T) {
	restoreStat := restoreRefreshStatFn
	statCalled := false
	restoreRefreshStatFn = func(string) (time.Time, error) {
		statCalled = true
		return time.Time{}, os.ErrNotExist
	}
	defer func() { restoreRefreshStatFn = restoreStat }()

	restoreSchedule := restoreRefreshScheduleFn
	scheduleCalls := 0
	restoreRefreshScheduleFn = func() tea.Cmd {
		scheduleCalls++
		return nil
	}
	defer func() { restoreRefreshScheduleFn = restoreSchedule }()

	m := NewModel(ModelConfig{Width: 80, Height: 24})
	m.restoreRefresh = &restoreRefreshState{dir: "/tmp/saves", lastModTime: time.Now()}

	cmd := m.handleRestoreRefreshTickMsg(restoreRefreshTickMsg{})

	if cmd != nil {
		t.Fatalf("expected no command once restore-from is no longer current, got %T", cmd)
	}
	if statCalled {
		t.Fatal("expected no stat call when restore-from is not current")
	}
	if scheduleCalls != 0 {
		t.Fatalf("expected no further scheduling, got %d calls", scheduleCalls)
	}
	if m.restoreRefresh != nil {
		t.Fatal("expected restore refresh state to be cleared")
	}
}
