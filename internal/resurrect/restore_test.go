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

// ── TestRestoreSessionConflict ───────────────────────────────────────────────

// TestRestoreSessionConflict verifies that when a session with the same name
// already exists it is skipped and the progress total remains accurate.
func TestRestoreSessionConflict(t *testing.T) {
	dir := t.TempDir()

	createCalled := false
	r1 := withCreateSessionFn(func(_, name, _, _ string) error {
		createCalled = true
		return nil
	})
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

	// pretend "conflict" session already exists
	r10 := withExistingSessionsFn(func(_ string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Sessions: []tmux.Session{{Name: "conflict"}},
		}, nil
	})
	defer r10()

	sf := buildSaveFile(Session{
		Name: "conflict",
		Windows: []Window{
			{Index: 0, Name: "main", Layout: "tiled", Active: true,
				Panes: []Pane{{Index: 0, WorkingDir: "/", Active: true}}},
		},
	})
	path := writeSaveFile(t, dir, "conflict", sf)

	cfg := Config{SaveDir: dir}
	ch := Restore(cfg, path)
	events := collectRestoreEvents(ch)

	// createSession must NOT have been called
	if createCalled {
		t.Error("createSessionFn should not be called for conflicting session")
	}

	last := events[len(events)-1]
	if !last.Done {
		t.Error("last event should be done")
	}
	if last.Err != nil {
		t.Errorf("unexpected error: %v", last.Err)
	}

	// step count must still reach total
	if last.Step != last.Total {
		t.Errorf("done: Step=%d Total=%d; want equal", last.Step, last.Total)
	}

	// total equals what computeRestoreTotal predicts
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

// ── TestPaneStartupCommand ───────────────────────────────────────────────────

func TestPaneStartupCommand(t *testing.T) {
	got := paneStartupCommand("/tmp/restore-123/dev:0.0", "/bin/bash")
	want := `cat "/tmp/restore-123/dev:0.0"; exec /bin/bash`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
