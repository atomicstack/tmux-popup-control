package resurrect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestRelativeSaveLastSymlink(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("saves", 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("saves", "snapshot.json")
	if err := WriteSaveFile(path, buildSaveFile()); err != nil {
		t.Fatal(err)
	}
	if err := updateLastSymlink("saves", path); err != nil {
		t.Fatal(err)
	}
	if _, err := LatestSave("saves"); err != nil {
		t.Fatalf("last symlink cannot resolve the saved file: %v", err)
	}
}

func TestMalformedAutoSaveStateFallsBackToSave(t *testing.T) {
	dir := t.TempDir()
	sf := buildSaveFile()
	sf.Kind = SaveKindAuto
	writeSaveFile(t, dir, "auto", sf)
	if err := os.WriteFile(autosaveStatePath(dir), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := LastAutoSaveSuccess(dir)
	if err != nil || !got.Equal(sf.Timestamp) {
		t.Fatalf("got %v, %v; want %v", got, err, sf.Timestamp)
	}
}

func TestAtomicSaveWritesReplaceSymlink(t *testing.T) {
	for _, kind := range []string{"save", "state", "archive"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			original := filepath.Join(dir, "original")
			path := filepath.Join(dir, "save.json")
			if kind == "state" {
				path = autosaveStatePath(dir)
			}
			if err := os.WriteFile(original, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(original, path); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "save":
				err = WriteSaveFile(path, buildSaveFile())
			case "state":
				err = WriteAutoSaveState(dir, time.Now())
			case "archive":
				err = WritePaneArchive(path, map[string]string{"pane": "content"})
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(original)
			if err != nil || string(got) != "original" {
				t.Fatalf("overwrote existing inode: %q, %v", got, err)
			}
		})
	}
}

func TestRestoreDifferentSnapshotsSameSession(t *testing.T) {
	t.Cleanup(installNoopRestoreFns(t))
	t.Cleanup(withExistingSessionsFn(func(string) (tmux.SessionSnapshot, error) { return makeSessions("alpha"), nil }))
	created := 0
	t.Cleanup(withCreateWindowFn(func(tmux.WindowSpec) error { created++; return nil }))
	dir := t.TempDir()
	for _, name := range []string{"one", "two", "two"} {
		sf := buildSaveFile(Session{Name: "alpha", Windows: []Window{{Name: name, Panes: []Pane{{}}}}})
		sf.Timestamp = time.Time{}
		file := writeSaveFile(t, dir, name, sf)
		for ev := range Restore(t.Context(), Config{}, file) {
			if ev.Err != nil {
				t.Fatal(ev.Err)
			}
		}
	}
	if created != 2 {
		t.Fatalf("created %d windows, want two distinct snapshots restored once each", created)
	}
}

func TestRestoreContentCleanupWaitsForFirstPane(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "error"}[fail], func(t *testing.T) {
			t.Cleanup(installNoopRestoreFns(t))
			tempRoot := t.TempDir()
			t.Setenv("TMPDIR", tempRoot)
			dir := t.TempDir()
			file := writeSaveFile(t, dir, "saved", buildSaveFile(Session{Name: "alpha", Windows: []Window{{Panes: []Pane{{}}}}}))
			if err := WritePaneArchive(paneArchivePath(file), map[string]string{"alpha:0.0": "hello"}); err != nil {
				t.Fatal(err)
			}
			waited := false
			t.Cleanup(withWaitForFn(func(context.Context, string, string) error {
				waited = true
				files, _ := filepath.Glob(filepath.Join(tempRoot, "tmux-restore-*", "alpha:0.0"))
				if len(files) != 1 {
					t.Error("content removed before replay completed")
				}
				if fail {
					return errors.New("replay failed")
				}
				return nil
			}))
			for range Restore(t.Context(), Config{}, file) {
			}
			if !waited {
				t.Error("first pane replay was not awaited")
			}
			files, _ := filepath.Glob(filepath.Join(tempRoot, "tmux-restore-*"))
			if len(files) != 0 {
				t.Errorf("temporary content leaked: %v", files)
			}
		})
	}
}

func TestRestoreSparseIndicesIntegration(t *testing.T) {
	socket, cleanup, _ := testutil.StartIsolatedTmuxServer(t)
	defer cleanup()
	for _, args := range [][]string{{"set-option", "-g", "base-index", "1"}, {"set-option", "-gw", "pane-base-index", "1"}} {
		if out, err := tmuxCmd(socket, args...).CombinedOutput(); err != nil {
			t.Fatalf("configure: %s: %v", out, err)
		}
	}
	activeDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sf := buildSaveFile(Session{Name: "sparse", Windows: []Window{{Index: 4, Name: "editor", Panes: []Pane{{Index: 3, WorkingDir: t.TempDir()}, {Index: 7, WorkingDir: activeDir, Active: true}, {Index: 11, WorkingDir: t.TempDir()}}}, {Index: 9, Name: "logs", Active: true, Panes: []Pane{{Index: 2, Active: true}}}}})
	sf.ClientSession = ""
	file := writeSaveFile(t, t.TempDir(), "sparse", sf)
	for ev := range Restore(t.Context(), Config{SocketPath: socket}, file) {
		if ev.Err != nil {
			t.Fatal(ev.Err)
		}
	}
	if got := strings.Join(listWindows(t, socket, "sparse"), ","); got != "4:editor,9:logs" {
		t.Fatalf("windows = %s", got)
	}
	if got := countPanes(t, socket, "sparse:4"); got != 3 {
		t.Fatalf("pane count = %d", got)
	}
	out, err := tmuxCmd(socket, "display-message", "-p", "-t", "sparse:4", "#{pane_index}").Output()
	if err != nil || strings.TrimSpace(string(out)) != "2" {
		t.Fatalf("active pane = %s: %v", out, err)
	}
	out, err = tmuxCmd(socket, "display-message", "-p", "-t", "sparse:4", "#{pane_current_path}").Output()
	if err != nil || strings.TrimSpace(string(out)) != activeDir {
		t.Fatalf("active pane cwd = %s: %v; want %s", out, err, activeDir)
	}
}

func TestNamedSavePathsAreUnique(t *testing.T) {
	dir := t.TempDir()
	first := savePath(dir, "named")
	second := savePath(dir, "named")
	if first == second {
		t.Fatalf("save paths collide: %s", first)
	}
}

func TestSaveDoesNotPublishBeforeArchive(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return makeSessions("alpha"), nil }))
	t.Cleanup(withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return makeWindows("alpha", 0), nil }))
	t.Cleanup(withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) {
		snap := makePanes("alpha", 0)
		snap.Panes[0].ID = "bad\x00pane"
		return snap, nil
	}))
	t.Cleanup(withCapturePaneContentsFn(func(string, string) (string, error) { return "content", nil }))
	t.Cleanup(withClientInfoFn(func(string, string) (string, string) { return "", "" }))
	var saveErr error
	for ev := range Save(t.Context(), Config{SaveDir: dir, CapturePaneContents: true}) {
		if ev.Err != nil {
			saveErr = ev.Err
		}
	}
	if saveErr == nil {
		t.Fatal("invalid archive should fail")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("failed save published files: %v", entries)
	}
}

func TestRestoreWaitsForStartedReplayAfterWindowFailure(t *testing.T) {
	t.Cleanup(installNoopRestoreFns(t))
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)
	file := writeSaveFile(t, t.TempDir(), "saved", buildSaveFile(Session{Name: "alpha", Windows: []Window{{Panes: []Pane{{}}}, {Index: 1, Panes: []Pane{{}}}}}))
	if err := WritePaneArchive(paneArchivePath(file), map[string]string{"alpha:0.0": "hello"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(withCreateWindowFn(func(tmux.WindowSpec) error { return errors.New("window failed") }))
	waited := false
	t.Cleanup(withWaitForFn(func(context.Context, string, string) error {
		waited = true
		files, _ := filepath.Glob(filepath.Join(tempRoot, "tmux-restore-*", "alpha:0.0"))
		if len(files) != 1 {
			t.Error("content removed before replay completed")
		}
		return nil
	}))
	for range Restore(t.Context(), Config{}, file) {
	}
	if !waited {
		t.Error("started replay was not awaited after window failure")
	}
	files, _ := filepath.Glob(filepath.Join(tempRoot, "tmux-restore-*"))
	if len(files) != 0 {
		t.Errorf("temporary content leaked: %v", files)
	}
}

func TestRestoreCancellationStillWaitsForOwnedReplay(t *testing.T) {
	t.Cleanup(installNoopRestoreFns(t))
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)
	file := writeSaveFile(t, t.TempDir(), "saved", buildSaveFile(Session{Name: "alpha", Windows: []Window{{Panes: []Pane{{}}}}}))
	if err := WritePaneArchive(paneArchivePath(file), map[string]string{"alpha:0.0": "hello"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	waits := 0
	t.Cleanup(withWaitForFn(func(waitCtx context.Context, _, _ string) error {
		waits++
		if waits == 1 {
			cancel()
			return ctx.Err()
		}
		if waitCtx.Err() != nil {
			t.Error("cleanup inherited cancellation")
		}
		files, _ := filepath.Glob(filepath.Join(tempRoot, "tmux-restore-*", "alpha:0.0"))
		if len(files) != 1 {
			t.Error("content removed before cleanup wait completed")
		}
		return nil
	}))
	for range Restore(ctx, Config{}, file) {
	}
	if waits != 2 {
		t.Fatalf("replay wait count = %d, want normal wait and cleanup wait", waits)
	}
}
