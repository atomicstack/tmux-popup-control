package resurrect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// makeSessions returns a small session snapshot for tests.
func makeSessions(names ...string) tmux.SessionSnapshot {
	sessions := make([]tmux.Session, len(names))
	for i, n := range names {
		sessions[i] = tmux.Session{Name: n, Attached: false}
	}
	return tmux.SessionSnapshot{Sessions: sessions}
}

// makeWindows returns a window snapshot with one window per (session, index) pair.
func makeWindows(sessionName string, indices ...int) tmux.WindowSnapshot {
	windows := make([]tmux.Window, len(indices))
	for i, idx := range indices {
		windows[i] = tmux.Window{
			ID:      fmt.Sprintf("%s:%d", sessionName, idx),
			Session: sessionName,
			Index:   idx,
			Name:    fmt.Sprintf("win%d", idx),
			Active:  idx == 0,
			Layout:  "even-horizontal",
		}
	}
	return tmux.WindowSnapshot{Windows: windows}
}

// makePanes returns a pane snapshot with one pane per window index.
func makePanes(sessionName string, windowIndices ...int) tmux.PaneSnapshot {
	panes := make([]tmux.Pane, len(windowIndices))
	for i, wIdx := range windowIndices {
		panes[i] = tmux.Pane{
			ID:        fmt.Sprintf("%s:%d.0", sessionName, wIdx),
			PaneID:    fmt.Sprintf("%%pane%d", i),
			Session:   sessionName,
			WindowIdx: wIdx,
			Index:     0,
			Command:   "bash",
			Path:      "/home/user",
			Width:     80,
			Height:    24,
			Active:    wIdx == 0,
		}
	}
	return tmux.PaneSnapshot{Panes: panes}
}

// collectEvents drains the channel returned by Save and returns all events.
func collectEvents(ch <-chan ProgressEvent) []ProgressEvent {
	var events []ProgressEvent
	for ev := range ch {
		events = append(events, ev)
		if ev.Done {
			break
		}
	}
	return events
}

// TestSave: happy path with two sessions, two windows each, pane contents enabled.
func TestSave(t *testing.T) {
	dir := t.TempDir()

	sessions := makeSessions("alpha", "beta")
	windows := tmux.WindowSnapshot{Windows: append(
		makeWindows("alpha", 0, 1).Windows,
		makeWindows("beta", 0, 1).Windows...,
	)}
	panes := tmux.PaneSnapshot{Panes: append(
		makePanes("alpha", 0, 1).Panes,
		makePanes("beta", 0, 1).Panes...,
	)}

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return sessions, nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return windows, nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return panes, nil })
	defer restoreFetchPanes()
	restoreCapture := withCapturePaneContentsFn(func(_, target string) (string, error) {
		return "pane content for " + target, nil
	})
	defer restoreCapture()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (clientSession, clientLastSession string) {
		return "alpha", ""
	})
	defer restoreClientInfo()

	cfg := Config{
		SaveDir:             dir,
		CapturePaneContents: true,
	}

	ch := Save(cfg)
	events := collectEvents(ch)

	if len(events) == 0 {
		t.Fatal("expected events, got none")
	}

	// first event: discovery
	first := events[0]
	if first.Kind != "info" {
		t.Errorf("first event kind: got %q, want %q", first.Kind, "info")
	}
	if first.Step != 0 {
		t.Errorf("first event step: got %d, want 0", first.Step)
	}
	if first.Total == 0 {
		t.Error("first event total should be > 0")
	}

	// last event: done
	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should have Done=true")
	}
	if last.Err != nil {
		t.Errorf("last event should have no error, got: %v", last.Err)
	}

	// verify session events (1 per session)
	sessionEvents := filterByKind(events, "session")
	if len(sessionEvents) != 2 {
		t.Errorf("session events: got %d, want 2", len(sessionEvents))
	}

	// verify window events (1 batch per session with windows = 2)
	windowEvents := filterByKind(events, "window")
	if len(windowEvents) != 2 {
		t.Errorf("window events: got %d, want 2", len(windowEvents))
	}

	// verify pane events (1 batch per session with panes = 2, contents enabled)
	paneEvents := filterByKind(events, "pane")
	if len(paneEvents) != 2 {
		t.Errorf("pane events: got %d, want 2", len(paneEvents))
	}

	// verify the save file was written
	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 save file, got %d", len(entries))
	}
	if entries[0].SessionCount != 2 {
		t.Errorf("session count: got %d, want 2", entries[0].SessionCount)
	}
	if !entries[0].HasPaneContents {
		t.Error("save file should have pane contents flag set")
	}

	// verify pane archive was written
	archivePath := paneArchivePath(entries[0].Path)
	if _, err := os.Stat(archivePath); err != nil {
		t.Errorf("pane archive not written: %v", err)
	}

	// verify last symlink was created (not a named snapshot)
	lastPath := filepath.Join(dir, "last")
	if _, err := os.Lstat(lastPath); err != nil {
		t.Errorf("last symlink not created: %v", err)
	}

	// verify total step count
	// 2 sessions + 4 windows + 4 panes + 1 (write json) + 1 (write archive) + 1 (symlink) = 13
	expectedTotal := 2 + 4 + 4 + 1 + 1 + 1
	if first.Total != expectedTotal {
		t.Errorf("total: got %d, want %d", first.Total, expectedTotal)
	}
}

// TestSaveNoSessions: empty tmux server produces a valid but empty save file.
func TestSaveNoSessions(t *testing.T) {
	dir := t.TempDir()

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{}, nil
	})
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) {
		return tmux.WindowSnapshot{}, nil
	})
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) {
		return tmux.PaneSnapshot{}, nil
	})
	defer restoreFetchPanes()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (clientSession, clientLastSession string) {
		return "", ""
	})
	defer restoreClientInfo()

	cfg := Config{SaveDir: dir, CapturePaneContents: false}
	ch := Save(cfg)
	events := collectEvents(ch)

	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event must be done")
	}
	if last.Err != nil {
		t.Errorf("unexpected error: %v", last.Err)
	}

	// save file should exist with 0 sessions
	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 save file, got %d", len(entries))
	}
	if entries[0].SessionCount != 0 {
		t.Errorf("expected 0 sessions, got %d", entries[0].SessionCount)
	}
}

// TestSaveNoPaneContents: pane contents disabled → no archive, fewer events.
func TestSaveNoPaneContents(t *testing.T) {
	dir := t.TempDir()

	sessions := makeSessions("main")
	windows := makeWindows("main", 0)
	panes := makePanes("main", 0)

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return sessions, nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return windows, nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return panes, nil })
	defer restoreFetchPanes()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (clientSession, clientLastSession string) {
		return "main", ""
	})
	defer restoreClientInfo()

	cfg := Config{SaveDir: dir, CapturePaneContents: false}
	ch := Save(cfg)
	events := collectEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("unexpected done event: done=%v err=%v", last.Done, last.Err)
	}

	// no pane events
	if n := len(filterByKind(events, "pane")); n != 0 {
		t.Errorf("expected no pane events, got %d", n)
	}

	// verify save file has HasPaneContents=false
	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 save file, got %d", len(entries))
	}
	if entries[0].HasPaneContents {
		t.Error("HasPaneContents should be false when contents disabled")
	}

	// no archive file
	archivePath := paneArchivePath(entries[0].Path)
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Error("pane archive should not exist when contents disabled")
	}

	// symlink updated (not a named snapshot)
	lastPath := filepath.Join(dir, "last")
	if _, err := os.Lstat(lastPath); err != nil {
		t.Errorf("last symlink should be created: %v", err)
	}

	// total: 1 session + 1 window + 0 panes + 1 (write json) + 0 (archive) + 1 (symlink) = 4
	first := events[0]
	expectedTotal := 1 + 1 + 1 + 1
	if first.Total != expectedTotal {
		t.Errorf("total: got %d, want %d", first.Total, expectedTotal)
	}
}

// TestSaveNamedSnapshot: named snapshot does NOT update the last symlink.
func TestSaveNamedSnapshot(t *testing.T) {
	dir := t.TempDir()

	sessions := makeSessions("work")
	windows := makeWindows("work", 0)
	panes := makePanes("work", 0)

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return sessions, nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return windows, nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return panes, nil })
	defer restoreFetchPanes()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (clientSession, clientLastSession string) {
		return "work", ""
	})
	defer restoreClientInfo()

	cfg := Config{SaveDir: dir, CapturePaneContents: false, Name: "mysnap"}
	ch := Save(cfg)
	events := collectEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("unexpected done event: done=%v err=%v", last.Done, last.Err)
	}

	// no last symlink for named snapshots
	lastPath := filepath.Join(dir, "last")
	if _, err := os.Lstat(lastPath); err == nil {
		t.Error("last symlink should NOT be created for named snapshots")
	}

	// verify named file exists (now includes timestamp: mysnap_TIMESTAMP.json)
	matches, _ := filepath.Glob(filepath.Join(dir, "mysnap_*.json"))
	if len(matches) != 1 {
		t.Errorf("expected 1 named save file matching mysnap_*.json, found %d", len(matches))
	}

	// total: 1 session + 1 window + 0 panes + 1 (write json) + 0 (archive) + 0 (symlink, named) = 3
	first := events[0]
	expectedTotal := 1 + 1 + 1
	if first.Total != expectedTotal {
		t.Errorf("total: got %d, want %d", first.Total, expectedTotal)
	}
}

// TestSaveCaptureError: error during capture stops with error event.
func TestSaveCaptureError(t *testing.T) {
	dir := t.TempDir()

	sessions := makeSessions("dev")
	windows := makeWindows("dev", 0, 1)
	panes := makePanes("dev", 0, 1)
	captureErr := errors.New("capture failed")

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return sessions, nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return windows, nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return panes, nil })
	defer restoreFetchPanes()
	restoreCapture := withCapturePaneContentsFn(func(_, _ string) (string, error) {
		return "", captureErr
	})
	defer restoreCapture()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (clientSession, clientLastSession string) {
		return "dev", ""
	})
	defer restoreClientInfo()

	cfg := Config{SaveDir: dir, CapturePaneContents: true}
	ch := Save(cfg)
	events := collectEvents(ch)

	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should be done")
	}
	if last.Err == nil {
		t.Error("expected an error event")
	}
	if !errors.Is(last.Err, captureErr) {
		t.Errorf("expected captureErr, got: %v", last.Err)
	}
}

// TestSaveFetchSessionsError: error fetching sessions produces an error event.
func TestSaveFetchSessionsError(t *testing.T) {
	dir := t.TempDir()
	fetchErr := errors.New("tmux unavailable")

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{}, fetchErr
	})
	defer restoreFetchSessions()

	cfg := Config{SaveDir: dir}
	ch := Save(cfg)
	events := collectEvents(ch)

	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should be done")
	}
	if !errors.Is(last.Err, fetchErr) {
		t.Errorf("expected fetchErr, got: %v", last.Err)
	}
}

// TestSaveTimestamp: saved file has a recent timestamp.
func TestSaveTimestamp(t *testing.T) {
	dir := t.TempDir()
	before := time.Now().Add(-time.Second)

	sessions := makeSessions("ts")
	windows := makeWindows("ts", 0)
	panes := makePanes("ts", 0)

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return sessions, nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return windows, nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return panes, nil })
	defer restoreFetchPanes()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (clientSession, clientLastSession string) {
		return "ts", ""
	})
	defer restoreClientInfo()

	cfg := Config{SaveDir: dir, CapturePaneContents: false}
	ch := Save(cfg)
	events := collectEvents(ch)

	last := events[len(events)-1]
	if last.Err != nil {
		t.Fatalf("unexpected error: %v", last.Err)
	}

	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	ts := entries[0].Timestamp
	if ts.Before(before) {
		t.Errorf("timestamp %v is before test start %v", ts, before)
	}
}

// filterByKind returns only events matching the given Kind.
func filterByKind(events []ProgressEvent, kind string) []ProgressEvent {
	var out []ProgressEvent
	for _, ev := range events {
		if ev.Kind == kind {
			out = append(out, ev)
		}
	}
	return out
}
