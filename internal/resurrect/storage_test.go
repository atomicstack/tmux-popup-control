package resurrect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestResolveDirEnvVar: env var overrides everything.
func TestResolveDirEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", dir)
	t.Setenv("XDG_DATA_HOME", "")

	got, err := ResolveDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

// TestResolveDirXDG: XDG_DATA_HOME set, no env var override.
func TestResolveDirXDG(t *testing.T) {
	xdgBase := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", "")
	t.Setenv("XDG_DATA_HOME", xdgBase)
	defer withTmuxOptionFn(func(_, _ string) string { return "" })()

	got, err := ResolveDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(xdgBase, "tmux-popup-control-sessions")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	// directory must exist
	if _, err := os.Stat(got); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

// TestResolveDirHomeFallback: nothing set, falls back to HOME.
func TestResolveDirHomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", home)
	defer withTmuxOptionFn(func(_, _ string) string { return "" })()

	got, err := ResolveDir("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(home, "tmux-popup-control-sessions")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if _, err := os.Stat(got); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

// TestResolveDirTmuxOption: tmuxOptionFn returns a custom dir.
func TestResolveDirTmuxOption(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")

	restore := withTmuxOptionFn(func(socket, opt string) string {
		if opt == "@tmux-popup-control-session-storage-dir" {
			return customDir
		}
		return ""
	})
	defer restore()

	got, err := ResolveDir("dummy-socket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != customDir {
		t.Errorf("got %q, want %q", got, customDir)
	}
}

// TestWriteReadSaveFile: round-trip write + read.
func TestWriteReadSaveFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	now := time.Now().Truncate(time.Second)
	sf := &SaveFile{
		Version:   currentVersion,
		Timestamp: now,
		Name:      "mysnap",
		Sessions: []Session{
			{
				Name: "main",
				Windows: []Window{
					{
						Index: 0,
						Name:  "editor",
						Panes: []Pane{
							{Index: 0, WorkingDir: "/tmp", Command: "vim"},
						},
					},
				},
			},
		},
	}

	if err := WriteSaveFile(path, sf); err != nil {
		t.Fatalf("WriteSaveFile: %v", err)
	}

	got, err := ReadSaveFile(path)
	if err != nil {
		t.Fatalf("ReadSaveFile: %v", err)
	}

	if got.Name != sf.Name {
		t.Errorf("name: got %q, want %q", got.Name, sf.Name)
	}
	if got.Version != sf.Version {
		t.Errorf("version: got %d, want %d", got.Version, sf.Version)
	}
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions: got %d, want 1", len(got.Sessions))
	}
	if got.Sessions[0].Name != "main" {
		t.Errorf("session name: got %q, want %q", got.Sessions[0].Name, "main")
	}
	if len(got.Sessions[0].Windows[0].Panes) != 1 {
		t.Errorf("pane count: got %d, want 1", len(got.Sessions[0].Windows[0].Panes))
	}
}

// TestLatestSave: symlink present and target exists.
func TestLatestSave(t *testing.T) {
	dir := t.TempDir()

	sf := &SaveFile{Version: currentVersion, Timestamp: time.Now(), Name: "snap1"}
	target := filepath.Join(dir, "snap1.json")
	if err := WriteSaveFile(target, sf); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := updateLastSymlink(dir, target); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got, err := LatestSave(dir)
	if err != nil {
		t.Fatalf("LatestSave: %v", err)
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}
}

// TestLatestSaveMissing: no symlink returns an error.
func TestLatestSaveMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := LatestSave(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no saved session found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestListSaves: multiple files, verify newest-first ordering.
func TestListSaves(t *testing.T) {
	dir := t.TempDir()

	times := []time.Time{
		time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
	}

	for i, ts := range times {
		sessions := make([]Session, i+1)
		for j := range sessions {
			sessions[j] = Session{Name: fmt.Sprintf("s%d", j)}
		}
		sf := &SaveFile{
			Version:   currentVersion,
			Timestamp: ts,
			Sessions:  sessions,
		}
		p := savePath(dir, "")
		// write with a unique timestamp-derived name to avoid collisions
		p = filepath.Join(dir, ts.Format("save_20060102T150405")+".json")
		if err := WriteSaveFile(p, sf); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	// newest first
	if !entries[0].Timestamp.After(entries[1].Timestamp) {
		t.Errorf("entries not sorted newest-first: [0]=%v [1]=%v", entries[0].Timestamp, entries[1].Timestamp)
	}
	if !entries[1].Timestamp.After(entries[2].Timestamp) {
		t.Errorf("entries not sorted newest-first: [1]=%v [2]=%v", entries[1].Timestamp, entries[2].Timestamp)
	}
}

// TestSaveFileExists: check exists and not exists.
func TestSaveFileExists(t *testing.T) {
	dir := t.TempDir()

	sf := &SaveFile{Version: currentVersion, Timestamp: time.Now(), Name: "mysnap"}
	p := savePath(dir, "mysnap")
	if err := WriteSaveFile(p, sf); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if !SaveFileExists(dir, "mysnap") {
		t.Error("expected mysnap to exist")
	}
	if SaveFileExists(dir, "ghost") {
		t.Error("expected ghost to not exist")
	}
}

// TestSavePath: auto-timestamped and named variants.
func TestSavePath(t *testing.T) {
	dir := "/some/dir"

	// named
	got := savePath(dir, "mysave")
	if got != "/some/dir/mysave.json" {
		t.Errorf("named: got %q, want %q", got, "/some/dir/mysave.json")
	}

	// auto-timestamped: must start with save_ and end with .json
	auto := savePath(dir, "")
	base := filepath.Base(auto)
	if !strings.HasPrefix(base, "save_") {
		t.Errorf("auto: expected prefix save_, got %q", base)
	}
	if !strings.HasSuffix(base, ".json") {
		t.Errorf("auto: expected suffix .json, got %q", base)
	}
}

// TestResolvePaneContentsEnvVar: env var overrides tmux option.
func TestResolvePaneContentsEnvVar(t *testing.T) {
	restore := withTmuxOptionFn(func(socket, opt string) string {
		return "off"
	})
	defer restore()

	t.Setenv("TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS", "true")
	if !ResolvePaneContents("dummy") {
		t.Error("expected true when env var is 'true'")
	}

	t.Setenv("TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS", "false")
	if ResolvePaneContents("dummy") {
		t.Error("expected false when env var is 'false'")
	}
}

// TestResolvePaneContentsTmuxOption: tmux option used when env var unset.
func TestResolvePaneContentsTmuxOption(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS", "")

	restore := withTmuxOptionFn(func(socket, opt string) string {
		if opt == "@tmux-popup-control-restore-pane-contents" {
			return "on"
		}
		return ""
	})
	defer restore()

	if !ResolvePaneContents("dummy") {
		t.Error("expected true when tmux option is 'on'")
	}
}

// TestResolvePaneContentsDefault: no env var or tmux option → false.
func TestResolvePaneContentsDefault(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS", "")

	restore := withTmuxOptionFn(func(socket, opt string) string {
		return ""
	})
	defer restore()

	if ResolvePaneContents("dummy") {
		t.Error("expected false by default")
	}
}

// TestPaneArchivePath: .json → .panes.tar.gz
func TestPaneArchivePath(t *testing.T) {
	got := paneArchivePath("/data/snap.json")
	want := "/data/snap.panes.tar.gz"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
