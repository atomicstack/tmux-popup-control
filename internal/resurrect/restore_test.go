package resurrect

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// buildSaveFile constructs a minimal SaveFile with the given sessions for testing.
func buildSaveFile(sessions ...Session) *SaveFile {
	return &SaveFile{
		Version:       currentVersion,
		Timestamp:     time.Now(),
		ClientSession: "alpha",
		Sessions:      sessions,
	}
}

// writeSaveFile writes sf as JSON to dir/name.json and returns the path.
func writeSaveFile(t *testing.T, dir string, name string, sf *SaveFile) string {
	t.Helper()
	path := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		t.Fatalf("marshal save file: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write save file: %v", err)
	}
	return path
}

// stubNoOp returns a function that accepts any args and returns nil.
func noopSession(_, _, _, _ string) error                    { return nil }
func noopWindow(_, _ string, _ int, _, _, _ string) error    { return nil }
func noopRename(_, _, _ string) error                        { return nil }
func noopSplit(_, _, _, _ string) error                      { return nil }
func noopLayout(_, _, _ string) error                        { return nil }
func noopPane(_, _ string) error                             { return nil }
func noopSelectWindow(_, _ string) error                     { return nil }
func noopSwitch(_, _, _ string) error                        { return nil }
func noopDefaultCommand(_ string) string                     { return "/bin/bash" }

// collectRestoreEvents drains the channel returned by Restore.
func collectRestoreEvents(ch <-chan ProgressEvent) []ProgressEvent {
	var events []ProgressEvent
	for ev := range ch {
		events = append(events, ev)
		if ev.Done {
			break
		}
	}
	return events
}

// withStatefulSessionOptionFns installs paired session option stubs that share
// state — values set via setSessionOptionFn are returned by sessionOptionFn.
// Optionally, pre-populate the map to simulate pre-existing markers.
func withStatefulSessionOptionFns(initial map[string]string) (func(), func()) {
	store := make(map[string]string)
	for k, v := range initial {
		store[k] = v
	}
	r1 := withSessionOptionFn(func(_, _, option string) string {
		return store[option]
	})
	r2 := withSetSessionOptionFn(func(_, _, option, value string) error {
		store[option] = value
		return nil
	})
	return r1, r2
}

// installNoopRestoreFns installs all no-op injectable functions for restore and
// returns a cleanup function that restores the originals.
func installNoopRestoreFns(t *testing.T) func() {
	t.Helper()
	r1 := withCreateSessionFn(noopSession)
	r2 := withCreateWindowFn(noopWindow)
	r3 := withRenameWindowFn(noopRename)
	r4 := withSplitPaneFn(noopSplit)
	r5 := withSelectLayoutTargetFn(noopLayout)
	r6 := withSelectPaneFn(noopPane)
	r7 := withSelectWindowFn(noopSelectWindow)
	r8 := withSwitchClientFn(noopSwitch)
	r9 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{}, nil
	})
	r10 := withDefaultCommandFn(noopDefaultCommand)
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{}, nil
	})
	r12, r13 := withStatefulSessionOptionFns(nil)
	return func() {
		r1()
		r2()
		r3()
		r4()
		r5()
		r6()
		r7()
		r8()
		r9()
		r10()
		r11()
		r12()
		r13()
	}
}

// ── TestRestoreHappyPath ────────────────────────────────────────────────────

// TestRestoreHappyPath exercises a full restore of two sessions, two windows
// each with two panes, no pane archive.
func TestRestoreHappyPath(t *testing.T) {
	dir := t.TempDir()
	defer installNoopRestoreFns(t)()

	sess1 := Session{
		Name: "alpha",
		Windows: []Window{
			{Index: 0, Name: "editor", Layout: "even-horizontal", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/home/user", Active: true}, {Index: 1, WorkingDir: "/tmp"}}},
			{Index: 1, Name: "server", Layout: "tiled",
				Panes: []Pane{{Index: 0, WorkingDir: "/srv", Active: true}}},
		},
	}
	sess2 := Session{
		Name: "beta",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "even-vertical", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/opt", Active: true}}},
		},
	}
	sf := buildSaveFile(sess1, sess2)
	sf.ClientSession = "alpha"

	path := writeSaveFile(t, dir, "test", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	if len(events) == 0 {
		t.Fatal("expected events, got none")
	}

	// first event is discovery
	first := events[0]
	if first.Kind != "info" {
		t.Errorf("first event kind: got %q, want %q", first.Kind, "info")
	}
	if first.Step != 0 {
		t.Errorf("first event step: got %d, want 0", first.Step)
	}
	if first.Total == 0 {
		t.Error("first event total must be > 0")
	}

	// last event is done, no error
	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should have Done=true")
	}
	if last.Err != nil {
		t.Errorf("unexpected error: %v", last.Err)
	}

	// step numbers are monotonically increasing
	for i := 1; i < len(events); i++ {
		if events[i].Step < events[i-1].Step {
			t.Errorf("step went backwards at index %d: %d < %d", i, events[i].Step, events[i-1].Step)
		}
	}

	// final step equals total
	if last.Step != last.Total {
		t.Errorf("done event: Step=%d Total=%d; want equal", last.Step, last.Total)
	}
}

// ── TestRestoreNoPaneArchive ─────────────────────────────────────────────────

// TestRestoreNoPaneArchive verifies that when no .panes.tar.gz exists the
// restore still completes and no pane-contents step is counted.
func TestRestoreNoPaneArchive(t *testing.T) {
	dir := t.TempDir()
	defer installNoopRestoreFns(t)()

	sf := buildSaveFile(Session{
		Name: "solo",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/home", Active: true}}},
		},
	})
	sf.HasPaneContents = false
	path := writeSaveFile(t, dir, "nopane", sf)
	// deliberately do not create the .panes.tar.gz

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("unexpected done state: done=%v err=%v", last.Done, last.Err)
	}

	// total should not include pane contents steps
	// session(1) + window(1) + layout(1) + active-pane(1) + active-window(1) + switch(1) + cleanup(1) = 7
	expected := computeRestoreTotal(sf)
	if events[0].Total != expected {
		t.Errorf("total: got %d, want %d", events[0].Total, expected)
	}
}

// ── TestRestoreSessionMerge ──────────────────────────────────────────────────

// TestRestoreSessionMerge verifies that when a session already exists, windows
// are merged into it (appended after the highest existing index) rather than
// the session being skipped.
func TestRestoreSessionMerge(t *testing.T) {
	dir := t.TempDir()

	createSessionCalled := false
	var createdWindowIndices []int
	r1 := withCreateSessionFn(func(_, _, _, _ string) error {
		createSessionCalled = true
		return nil
	})
	defer r1()
	r2 := withCreateWindowFn(func(_, _ string, index int, _, _, _ string) error {
		createdWindowIndices = append(createdWindowIndices, index)
		return nil
	})
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(noopSplit)
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withDefaultCommandFn(noopDefaultCommand)
	defer r9()

	// pretend "existing" session already exists with window at index 0
	r10 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Sessions: []tmux.Session{{Name: "existing"}},
		}, nil
	})
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{0: true}, nil
	})
	defer r11()
	r12, r13 := withStatefulSessionOptionFns(nil)
	defer r12()
	defer r13()

	sf := buildSaveFile(Session{
		Name: "existing",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/", Active: true}}},
		},
	})
	path := writeSaveFile(t, dir, "merge", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	// session must NOT be created (it already exists)
	if createSessionCalled {
		t.Error("createSessionFn should not be called for existing session")
	}

	// window must be created at remapped index 1 (after existing 0)
	if len(createdWindowIndices) != 1 {
		t.Fatalf("expected 1 createWindow call, got %d", len(createdWindowIndices))
	}
	if createdWindowIndices[0] != 1 {
		t.Errorf("expected window created at index 1, got %d", createdWindowIndices[0])
	}

	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should be done")
	}
	if last.Err != nil {
		t.Errorf("unexpected error: %v", last.Err)
	}
	if last.Step != last.Total {
		t.Errorf("done: Step=%d Total=%d; want equal", last.Step, last.Total)
	}

	expected := computeRestoreTotal(sf)
	if events[0].Total != expected {
		t.Errorf("total: got %d, want %d", events[0].Total, expected)
	}
}

// ── TestRestoreNamedSnapshot ─────────────────────────────────────────────────

// TestRestoreNamedSnapshot verifies that a named snapshot file is read and
// restored correctly.
func TestRestoreNamedSnapshot(t *testing.T) {
	dir := t.TempDir()
	defer installNoopRestoreFns(t)()

	sf := buildSaveFile(Session{
		Name: "work",
		Windows: []Window{
			{Index: 0, Name: "editor", Layout: "even-horizontal", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/code", Active: true}}},
		},
	})
	sf.Name = "mysnap"
	path := writeSaveFile(t, dir, "mysnap", sf)

	cfg := Config{SaveDir: dir, Name: "mysnap"}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}
	if last.Step != last.Total {
		t.Errorf("done: Step=%d Total=%d; want equal", last.Step, last.Total)
	}
}

// ── TestRestoreSessionCreationError ─────────────────────────────────────────

// TestRestoreSessionCreationError verifies that an error during session
// creation produces an error event and terminates the restore.
func TestRestoreSessionCreationError(t *testing.T) {
	dir := t.TempDir()

	createErr := errors.New("new-session failed")
	r1 := withCreateSessionFn(func(_, _, _, _ string) error { return createErr })
	defer r1()
	r2 := withCreateWindowFn(noopWindow)
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(noopSplit)
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{}, nil
	})
	defer r9()
	r10 := withDefaultCommandFn(noopDefaultCommand)
	defer r10()

	sf := buildSaveFile(Session{
		Name: "dev",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/", Active: true}}},
		},
	})
	path := writeSaveFile(t, dir, "err", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should have Done=true")
	}
	if last.Err == nil {
		t.Fatal("expected an error event")
	}
	if !errors.Is(last.Err, createErr) {
		t.Errorf("expected createErr, got: %v", last.Err)
	}
	if last.Kind != "error" {
		t.Errorf("expected kind %q, got %q", "error", last.Kind)
	}
}

// ── TestRestoreWithPaneArchive ───────────────────────────────────────────────

// TestRestoreWithPaneArchive verifies that when a companion .panes.tar.gz
// exists, startup commands are passed to creation functions instead of using
// paste-buffer.
func TestRestoreWithPaneArchive(t *testing.T) {
	dir := t.TempDir()

	// track startup commands passed to creation functions
	var sessionCommands []string
	var splitCommands []string

	r1 := withCreateSessionFn(func(_, _, _, command string) error {
		sessionCommands = append(sessionCommands, command)
		return nil
	})
	defer r1()
	r2 := withCreateWindowFn(noopWindow)
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(func(_, _, _, command string) error {
		splitCommands = append(splitCommands, command)
		return nil
	})
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{}, nil
	})
	defer r9()
	r10 := withDefaultCommandFn(func(_ string) string { return "/bin/bash" })
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{}, nil
	})
	defer r11()
	r12, r13 := withStatefulSessionOptionFns(nil)
	defer r12()
	defer r13()

	sf := buildSaveFile(Session{
		Name: "dev",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{
					{Index: 0, WorkingDir: "/home", Active: true},
					{Index: 1, WorkingDir: "/tmp"},
				}},
		},
	})
	sf.HasPaneContents = true
	path := writeSaveFile(t, dir, "withpane", sf)

	// write a companion archive with pane contents keyed by session:window.pane
	archivePath := paneArchivePath(path)
	paneContents := map[string]string{
		"dev:0.0": "output for pane 0",
		"dev:0.1": "output for pane 1",
	}
	if err := WritePaneArchive(archivePath, paneContents); err != nil {
		t.Fatalf("WritePaneArchive: %v", err)
	}

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}

	// verify session creation got a startup command (for pane 0)
	if len(sessionCommands) != 1 {
		t.Fatalf("expected 1 createSession call, got %d", len(sessionCommands))
	}
	if sessionCommands[0] == "" {
		t.Error("expected non-empty startup command for session creation (pane 0)")
	}

	// verify split got a startup command (for pane 1)
	if len(splitCommands) != 1 {
		t.Fatalf("expected 1 splitPane call, got %d", len(splitCommands))
	}
	if splitCommands[0] == "" {
		t.Error("expected non-empty startup command for split (pane 1)")
	}

	// total should NOT include separate pane content send steps
	expected := computeRestoreTotal(sf)
	if events[0].Total != expected {
		t.Errorf("total: got %d, want %d", events[0].Total, expected)
	}
}

// ── TestRestoreStepCountMatchesTotal ─────────────────────────────────────────

// TestRestoreStepCountMatchesTotal checks that the step counter exactly
// reaches total for a multi-session, multi-window, multi-pane save.
func TestRestoreStepCountMatchesTotal(t *testing.T) {
	dir := t.TempDir()
	defer installNoopRestoreFns(t)()

	sf := buildSaveFile(
		Session{
			Name: "s1",
			Windows: []Window{
				{Index: 0, Name: "w0", Layout: "tiled", Active: true,
					Panes: []Pane{
						{Index: 0, WorkingDir: "/", Active: true},
						{Index: 1, WorkingDir: "/tmp"},
						{Index: 2, WorkingDir: "/var"},
					}},
				{Index: 1, Name: "w1", Layout: "even-horizontal",
					Panes: []Pane{{Index: 0, WorkingDir: "/opt", Active: true}}},
			},
		},
		Session{
			Name: "s2",
			Windows: []Window{
				{Index: 0, Name: "main", Layout: "tiled", Active: true,
					Panes: []Pane{{Index: 0, WorkingDir: "/home", Active: true}}},
			},
		},
	)
	path := writeSaveFile(t, dir, "multi", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("unexpected state: done=%v err=%v", last.Done, last.Err)
	}
	if last.Step != last.Total {
		t.Errorf("final step %d != total %d", last.Step, last.Total)
	}

	expected := computeRestoreTotal(sf)
	if events[0].Total != expected {
		t.Errorf("total mismatch: got %d, want %d", events[0].Total, expected)
	}
}

// ── TestRestoreReadFileError ──────────────────────────────────────────────────

// TestRestoreReadFileError checks that a missing save file produces an error event.
func TestRestoreReadFileError(t *testing.T) {
	dir := t.TempDir()
	defer installNoopRestoreFns(t)()

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, filepath.Join(dir, "nonexistent.json"))
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should be done")
	}
	if last.Err == nil {
		t.Error("expected an error")
	}
}

// ── TestRestoreSwitchesClientWithID ──────────────────────────────────────────

// TestRestoreSwitchesClientWithID verifies that the restore passes the
// configured ClientID to switchClientFn so the terminal client (not the
// control-mode client) is switched.
func TestRestoreSwitchesClientWithID(t *testing.T) {
	dir := t.TempDir()

	var switchSocket, switchClient, switchTarget string
	r1 := withCreateSessionFn(noopSession)
	defer r1()
	r2 := withCreateWindowFn(noopWindow)
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(noopSplit)
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(func(socket, clientID, target string) error {
		switchSocket = socket
		switchClient = clientID
		switchTarget = target
		return nil
	})
	defer r8()
	r9 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{}, nil
	})
	defer r9()
	r10 := withDefaultCommandFn(noopDefaultCommand)
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{}, nil
	})
	defer r11()
	r12, r13 := withStatefulSessionOptionFns(nil)
	defer r12()
	defer r13()

	sf := buildSaveFile(Session{
		Name: "dev",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/", Active: true}}},
		},
	})
	sf.ClientSession = "dev"
	path := writeSaveFile(t, dir, "switchtest", sf)

	cfg := Config{
		SocketPath: "/tmp/test.sock",
		SaveDir:    dir,
		ClientID:   "/dev/ttys004",
	}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}

	if switchSocket != "/tmp/test.sock" {
		t.Errorf("switch socket: got %q, want %q", switchSocket, "/tmp/test.sock")
	}
	if switchClient != "/dev/ttys004" {
		t.Errorf("switch clientID: got %q, want %q", switchClient, "/dev/ttys004")
	}
	if switchTarget != "dev" {
		t.Errorf("switch target: got %q, want %q", switchTarget, "dev")
	}
}

// ── TestRestoreMergeIndexRemapping ───────────────────────────────────────────

// TestRestoreMergeIndexRemapping verifies that when merging into a session with
// existing windows at indices 0 and 1, saved windows (originally 0,1,2) are
// remapped to indices 2,3,4.
func TestRestoreMergeIndexRemapping(t *testing.T) {
	dir := t.TempDir()

	createSessionCalled := false
	var createdWindows []struct{ session string; index int; name string }
	r1 := withCreateSessionFn(func(_, _, _, _ string) error {
		createSessionCalled = true
		return nil
	})
	defer r1()
	r2 := withCreateWindowFn(func(_, session string, index int, name, _, _ string) error {
		createdWindows = append(createdWindows, struct{ session string; index int; name string }{session, index, name})
		return nil
	})
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(noopSplit)
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withDefaultCommandFn(noopDefaultCommand)
	defer r9()

	// existing session has windows at indices 0 and 1
	r10 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Sessions: []tmux.Session{{Name: "work"}},
		}, nil
	})
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{0: true, 1: true}, nil
	})
	defer r11()
	r12, r13 := withStatefulSessionOptionFns(nil)
	defer r12()
	defer r13()

	sf := buildSaveFile(Session{
		Name: "work",
		Windows: []Window{
			{Index: 0, Name: "editor", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/code", Active: true}}},
			{Index: 1, Name: "server", Layout: "even-horizontal",
				Panes: []Pane{{Index: 0, WorkingDir: "/srv", Active: true}}},
			{Index: 2, Name: "logs", Layout: "tiled",
				Panes: []Pane{{Index: 0, WorkingDir: "/var/log", Active: true}}},
		},
	})
	path := writeSaveFile(t, dir, "remap", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	if createSessionCalled {
		t.Error("createSessionFn should not be called for existing session")
	}

	// all 3 windows should be created at remapped indices 2, 3, 4
	if len(createdWindows) != 3 {
		t.Fatalf("expected 3 createWindow calls, got %d", len(createdWindows))
	}
	wantIndices := []int{2, 3, 4}
	wantNames := []string{"editor", "server", "logs"}
	for i, cw := range createdWindows {
		if cw.index != wantIndices[i] {
			t.Errorf("window %d: index got %d, want %d", i, cw.index, wantIndices[i])
		}
		if cw.name != wantNames[i] {
			t.Errorf("window %d: name got %q, want %q", i, cw.name, wantNames[i])
		}
	}

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}
	if last.Step != last.Total {
		t.Errorf("done: Step=%d Total=%d; want equal", last.Step, last.Total)
	}
}

// ── TestRestoreMergeWithPaneArchive ─────────────────────────────────────────

// TestRestoreMergeWithPaneArchive verifies that pane content startup commands
// use the original (saved) indices to locate archive files, while windows are
// created at remapped indices.
func TestRestoreMergeWithPaneArchive(t *testing.T) {
	dir := t.TempDir()

	var windowCmds []struct{ index int; cmd string }
	var splitCmds []struct{ target string; cmd string }

	r1 := withCreateSessionFn(func(_, _, _, _ string) error { return nil })
	defer r1()
	r2 := withCreateWindowFn(func(_, _ string, index int, _, _, cmd string) error {
		windowCmds = append(windowCmds, struct{ index int; cmd string }{index, cmd})
		return nil
	})
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(func(_, target, _, cmd string) error {
		splitCmds = append(splitCmds, struct{ target string; cmd string }{target, cmd})
		return nil
	})
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withDefaultCommandFn(func(_ string) string { return "/bin/bash" })
	defer r9()

	// existing session with window 0
	r10 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Sessions: []tmux.Session{{Name: "dev"}},
		}, nil
	})
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{0: true}, nil
	})
	defer r11()
	r12, r13 := withStatefulSessionOptionFns(nil)
	defer r12()
	defer r13()

	sf := buildSaveFile(Session{
		Name: "dev",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{
					{Index: 0, WorkingDir: "/home", Active: true},
					{Index: 1, WorkingDir: "/tmp"},
				}},
		},
	})
	sf.HasPaneContents = true
	path := writeSaveFile(t, dir, "mergepane", sf)

	// write companion archive keyed by ORIGINAL saved indices
	archivePath := paneArchivePath(path)
	paneContents := map[string]string{
		"dev:0.0": "output for pane 0",
		"dev:0.1": "output for pane 1",
	}
	if err := WritePaneArchive(archivePath, paneContents); err != nil {
		t.Fatalf("WritePaneArchive: %v", err)
	}

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}

	// window should be created at remapped index 1
	if len(windowCmds) != 1 {
		t.Fatalf("expected 1 createWindow call, got %d", len(windowCmds))
	}
	if windowCmds[0].index != 1 {
		t.Errorf("window index: got %d, want 1", windowCmds[0].index)
	}
	// startup command should reference the original pane key (dev:0.0)
	if windowCmds[0].cmd == "" {
		t.Error("expected non-empty startup command for window creation (pane 0)")
	}

	// split should target the remapped window index
	if len(splitCmds) != 1 {
		t.Fatalf("expected 1 splitPane call, got %d", len(splitCmds))
	}
	if splitCmds[0].target != "dev:1" {
		t.Errorf("split target: got %q, want %q", splitCmds[0].target, "dev:1")
	}
	if splitCmds[0].cmd == "" {
		t.Error("expected non-empty startup command for split (pane 1)")
	}
}

// ── TestRestoreMergeIdempotent ───────────────────────────────────────────────

// TestRestoreMergeIdempotent verifies that re-running the same restore against
// a session that was already merged skips the session instead of duplicating
// windows.
func TestRestoreMergeIdempotent(t *testing.T) {
	dir := t.TempDir()

	createWindowCalled := false
	r1 := withCreateSessionFn(func(_, _, _, _ string) error { return nil })
	defer r1()
	r2 := withCreateWindowFn(func(_, _ string, _ int, _, _, _ string) error {
		createWindowCalled = true
		return nil
	})
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(noopSplit)
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withDefaultCommandFn(noopDefaultCommand)
	defer r9()

	r10 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Sessions: []tmux.Session{{Name: "work"}},
		}, nil
	})
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{0: true}, nil
	})
	defer r11()

	// simulate that the marker was already set from a prior restore
	markerKey := restoreMarkerKey("work")
	r12, r13 := withStatefulSessionOptionFns(map[string]string{markerKey: "1"})
	defer r12()
	defer r13()

	sf := buildSaveFile(Session{
		Name: "work",
		Windows: []Window{
			{Index: 0, Name: "editor", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/code", Active: true}}},
		},
	})
	path := writeSaveFile(t, dir, "repeat", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	// no windows should have been created
	if createWindowCalled {
		t.Error("createWindowFn should not be called when marker exists")
	}

	// should have a skip message
	found := false
	for _, ev := range events {
		if ev.Kind == "info" && ev.ID == "work" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a skip event for session 'work'")
	}

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}
	if last.Step != last.Total {
		t.Errorf("done: Step=%d Total=%d; want equal", last.Step, last.Total)
	}
}

// ── TestRestoreMergeSetsMarker ──────────────────────────────────────────────

// TestRestoreMergeSetsMarker verifies that after a successful merge the
// idempotency marker is set on the session.
func TestRestoreMergeSetsMarker(t *testing.T) {
	dir := t.TempDir()

	var setOptions []struct{ session, option, value string }
	r1 := withCreateSessionFn(func(_, _, _, _ string) error { return nil })
	defer r1()
	r2 := withCreateWindowFn(noopWindow)
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(noopSplit)
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withDefaultCommandFn(noopDefaultCommand)
	defer r9()

	r10 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Sessions: []tmux.Session{{Name: "dev"}},
		}, nil
	})
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{0: true}, nil
	})
	defer r11()

	// no marker yet — use stateful stubs but also record calls
	store := make(map[string]string)
	r12 := withSessionOptionFn(func(_, _, option string) string {
		return store[option]
	})
	defer r12()
	r13 := withSetSessionOptionFn(func(_, session, option, value string) error {
		store[option] = value
		setOptions = append(setOptions, struct{ session, option, value string }{session, option, value})
		return nil
	})
	defer r13()

	sf := buildSaveFile(Session{
		Name: "dev",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/", Active: true}}},
		},
	})
	path := writeSaveFile(t, dir, "marker", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}

	// verify marker was set
	if len(setOptions) != 1 {
		t.Fatalf("expected 1 set-option call, got %d", len(setOptions))
	}
	if setOptions[0].session != "dev" {
		t.Errorf("marker session: got %q, want %q", setOptions[0].session, "dev")
	}
	wantKey := "@tmux-popup-control-session-restored-dev"
	if setOptions[0].option != wantKey {
		t.Errorf("marker key: got %q, want %q", setOptions[0].option, wantKey)
	}
}

// ── TestRestoreNewSessionSetsMarker ──────────────────────────────────────────

// TestRestoreNewSessionSetsMarker verifies that creating a brand-new session
// still sets the idempotency marker so a second restore doesn't duplicate it.
func TestRestoreNewSessionSetsMarker(t *testing.T) {
	dir := t.TempDir()

	var setOptions []struct{ session, option, value string }

	r1 := withCreateSessionFn(noopSession)
	defer r1()
	r2 := withCreateWindowFn(noopWindow)
	defer r2()
	r3 := withRenameWindowFn(noopRename)
	defer r3()
	r4 := withSplitPaneFn(noopSplit)
	defer r4()
	r5 := withSelectLayoutTargetFn(noopLayout)
	defer r5()
	r6 := withSelectPaneFn(noopPane)
	defer r6()
	r7 := withSelectWindowFn(noopSelectWindow)
	defer r7()
	r8 := withSwitchClientFn(noopSwitch)
	defer r8()
	r9 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{}, nil
	})
	defer r9()
	r10 := withDefaultCommandFn(noopDefaultCommand)
	defer r10()
	r11 := withExistingWindowIndicesFn(func(_, _ string) (map[int]bool, error) {
		return map[int]bool{}, nil
	})
	defer r11()
	// no marker check for new sessions (merge=false), but set IS called
	store := make(map[string]string)
	r12 := withSessionOptionFn(func(_, _, option string) string {
		return store[option]
	})
	defer r12()
	r13 := withSetSessionOptionFn(func(_, session, option, value string) error {
		store[option] = value
		setOptions = append(setOptions, struct{ session, option, value string }{session, option, value})
		return nil
	})
	defer r13()

	sf := buildSaveFile(Session{
		Name: "fresh",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/", Active: true}}},
		},
	})
	path := writeSaveFile(t, dir, "fresh", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Fatalf("restore failed: done=%v err=%v", last.Done, last.Err)
	}

	if len(setOptions) != 1 {
		t.Fatalf("expected 1 set-option call, got %d", len(setOptions))
	}
	wantKey := "@tmux-popup-control-session-restored-fresh"
	if setOptions[0].option != wantKey {
		t.Errorf("marker key: got %q, want %q", setOptions[0].option, wantKey)
	}
}

// ── TestPaneStartupCommand ───────────────────────────────────────────────────

func TestPaneStartupCommand(t *testing.T) {
	got := paneStartupCommand("/tmp/restore-123/dev:0.0", "/bin/bash")
	want := `cat "/tmp/restore-123/dev:0.0"; exec /bin/bash`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
