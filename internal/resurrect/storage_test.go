package resurrect

import (
	"errors"
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
	info, err := os.Stat(got)
	if err != nil {
		t.Errorf("directory not created: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Errorf("directory mode: got %03o, want 700", got)
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
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat save file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("save file mode: got %03o, want 600", got)
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

// TestDeleteSaveRemovesFileAndArchive verifies DeleteSave removes both
// the JSON save file and its companion pane-contents archive.
func TestDeleteSaveRemovesFileAndArchive(t *testing.T) {
	dir := t.TempDir()
	sf := &SaveFile{Version: currentVersion, Timestamp: time.Now(), Name: "snap"}
	path := filepath.Join(dir, "snap.json")
	if err := WriteSaveFile(path, sf); err != nil {
		t.Fatalf("WriteSaveFile: %v", err)
	}
	archive := paneArchivePath(path)
	if err := os.WriteFile(archive, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	if err := DeleteSave(dir, path); err != nil {
		t.Fatalf("DeleteSave: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected save file removed, got err=%v", err)
	}
	if _, err := os.Stat(archive); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected archive removed, got err=%v", err)
	}
}

// TestDeleteSaveRepointsLastSymlink verifies that deleting the save the
// "last" symlink points at causes the symlink to be repointed at the
// next-newest remaining save.
func TestDeleteSaveRepointsLastSymlink(t *testing.T) {
	dir := t.TempDir()

	older := filepath.Join(dir, "older.json")
	newer := filepath.Join(dir, "newer.json")
	if err := WriteSaveFile(older, &SaveFile{Version: currentVersion, Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Name: "older"}); err != nil {
		t.Fatalf("write older: %v", err)
	}
	if err := WriteSaveFile(newer, &SaveFile{Version: currentVersion, Timestamp: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), Name: "newer"}); err != nil {
		t.Fatalf("write newer: %v", err)
	}
	if err := updateLastSymlink(dir, "newer.json"); err != nil {
		t.Fatalf("updateLastSymlink: %v", err)
	}

	if err := DeleteSave(dir, newer); err != nil {
		t.Fatalf("DeleteSave: %v", err)
	}
	got, err := LatestSave(dir)
	if err != nil {
		t.Fatalf("LatestSave after delete: %v", err)
	}
	if got != older {
		t.Errorf("expected last symlink to repoint at %q, got %q", older, got)
	}
}

// TestDeleteSaveRemovesDanglingSymlink verifies the symlink is removed
// (not left dangling) when deleting the only remaining save.
func TestDeleteSaveRemovesDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "only.json")
	if err := WriteSaveFile(path, &SaveFile{Version: currentVersion, Timestamp: time.Now(), Name: "only"}); err != nil {
		t.Fatalf("WriteSaveFile: %v", err)
	}
	if err := updateLastSymlink(dir, "only.json"); err != nil {
		t.Fatalf("updateLastSymlink: %v", err)
	}

	if err := DeleteSave(dir, path); err != nil {
		t.Fatalf("DeleteSave: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "last")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected last symlink removed, got err=%v", err)
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
	// create a file matching the new naming convention: mysnap_TIMESTAMP.json
	p := filepath.Join(dir, "mysnap_20260322T120000.json")
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

// TestSavePath: UUID-based unnamed and timestamped named variants.
func TestSavePath(t *testing.T) {
	dir := "/some/dir"

	// named: name_TIMESTAMP.json
	got := savePath(dir, "mysave")
	base := filepath.Base(got)
	if !strings.HasPrefix(base, "mysave_") {
		t.Errorf("named: expected prefix mysave_, got %q", base)
	}
	if !strings.HasSuffix(base, ".json") {
		t.Errorf("named: expected suffix .json, got %q", base)
	}

	// unnamed: UUID_TIMESTAMP.json (UUID is 36 chars with hyphens)
	auto := savePath(dir, "")
	autoBase := filepath.Base(auto)
	if !strings.HasSuffix(autoBase, ".json") {
		t.Errorf("auto: expected suffix .json, got %q", autoBase)
	}
	stem := strings.TrimSuffix(autoBase, ".json")
	// last segment after final _ is the timestamp; everything before is the UUID
	lastUnderscore := strings.LastIndex(stem, "_")
	if lastUnderscore < 0 {
		t.Fatalf("auto: expected UUID_TIMESTAMP format, got %q", autoBase)
	}
	uuidStr := stem[:lastUnderscore]
	if len(uuidStr) != 36 {
		t.Errorf("auto: expected 36-char UUID, got %q (%d chars)", uuidStr, len(uuidStr))
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

// TestRelativeTime: high-resolution relative timestamps.
func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 3, 22, 14, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"1m ago", now.Add(-90 * time.Second), "1m ago"},
		{"5m ago", now.Add(-5 * time.Minute), "5m ago"},
		{"59m ago", now.Add(-59 * time.Minute), "59m ago"},
		{"1h ago", now.Add(-time.Hour), "1h ago"},
		{"12h ago", now.Add(-12 * time.Hour), "12h ago"},
		{"yesterday", now.Add(-36 * time.Hour), "yesterday"},
		{"3d ago", now.Add(-3 * 24 * time.Hour), "3d ago"},
		{"29d ago", now.Add(-29 * 24 * time.Hour), "29d ago"},
		{"1 month ago", now.Add(-35 * 24 * time.Hour), "1 month ago"},
		{"3 months ago", now.Add(-91 * 24 * time.Hour), "3 months ago"},
		{"1 year ago", now.Add(-400 * 24 * time.Hour), "1 year ago"},
		{"2 years ago", now.Add(-800 * 24 * time.Hour), "2 years ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RelativeTime(tt.t, now)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestDisplayName: named saves return name, unnamed return truncated UUID.
func TestDisplayName(t *testing.T) {
	named := SaveEntry{Name: "mysnap", Path: "/data/mysnap_20260322T120000.json"}
	if got := named.DisplayName(); got != "mysnap" {
		t.Errorf("named: got %q, want %q", got, "mysnap")
	}

	uuid := SaveEntry{Name: "", Path: "/data/a1b2c3d4-e5f6-7890-abcd-ef1234567890_20260322T120000.json"}
	if got := uuid.DisplayName(); got != "a1b2c3d4" {
		t.Errorf("uuid: got %q, want %q", got, "a1b2c3d4")
	}

	short := SaveEntry{Name: "", Path: "/data/abc.json"}
	if got := short.DisplayName(); got != "abc" {
		t.Errorf("short: got %q, want %q", got, "abc")
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
