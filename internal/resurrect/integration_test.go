package resurrect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// tmuxCmd builds a tmux command against the given socket with TMUX stripped
// from the environment so we never contaminate the user's session.
func tmuxCmd(socket string, args ...string) *exec.Cmd {
	all := append([]string{"-S", socket}, args...)
	cmd := exec.Command("tmux", all...)
	var env []string
	for _, e := range cmd.Environ() {
		if !strings.HasPrefix(e, "TMUX=") {
			env = append(env, e)
		}
	}
	env = append(env, "TMUX=")
	cmd.Env = env
	return cmd
}

// listSessionNames returns the sorted session names on the given socket.
func listSessionNames(t *testing.T, socket string) []string {
	t.Helper()
	out, err := tmuxCmd(socket, "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatalf("list-sessions: %v", err)
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			names = append(names, s)
		}
	}
	sort.Strings(names)
	return names
}

// listWindows returns window names for a session, sorted by index.
func listWindows(t *testing.T, socket, session string) []string {
	t.Helper()
	out, err := tmuxCmd(socket, "list-windows", "-t", session, "-F", "#{window_index}:#{window_name}").Output()
	if err != nil {
		t.Fatalf("list-windows %s: %v", session, err)
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			names = append(names, s)
		}
	}
	sort.Strings(names)
	return names
}

// countPanes returns the number of panes in a window.
func countPanes(t *testing.T, socket, target string) int {
	t.Helper()
	out, err := tmuxCmd(socket, "list-panes", "-t", target, "-F", "#{pane_index}").Output()
	if err != nil {
		t.Fatalf("list-panes %s: %v", target, err)
	}
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// waitSession polls until the session exists (up to 2s).
func waitSession(t *testing.T, socket, session string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := tmuxCmd(socket, "has-session", "-t", session).Run(); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("session %q did not appear in time", session)
}

// startTmuxServerWithConfig boots a tmux server using a real config file
// instead of -f /dev/null. Returns socket, cleanup, logDir.
func startTmuxServerWithConfig(t *testing.T, confPath string) (string, func(), string) {
	t.Helper()
	testutil.RequireTmux(t)
	baseDir, err := os.MkdirTemp("/tmp", "tmux-popup-control-*")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(baseDir) })
	socket := filepath.Join(baseDir, "tmux-test.sock")

	cmd := tmuxCmd(socket, "-f", confPath, "new-session", "-d", "-s", "init", "-x", "80", "-y", "24")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("skipping: failed to start tmux with config %s: %v\noutput: %s", confPath, err, out)
	}
	t.Logf("started tmux with config=%s socket=%s", confPath, socket)

	cleanup := func() { _ = tmuxCmd(socket, "kill-server").Run() }
	return socket, cleanup, baseDir
}

// TestSaveRestoreRoundTripIntegration exercises a full save→restore cycle
// across two independent tmux servers and verifies that the restored
// structure matches the original. It also checks that no phantom sessions
// are created during the process.
func TestSaveRestoreRoundTripIntegration(t *testing.T) {
	testutil.RequireTmux(t)

	// ── server 1: build the source structure ────────────────────────────

	socket1, cleanup1, logDir1 := testutil.StartTmuxServer(t)
	defer cleanup1()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir1) })

	// StartTmuxServer creates one session "tmux-popup-control-test".
	// Rename it to "alpha" and use it as our first session.
	if err := tmuxCmd(socket1, "rename-session", "-t", "tmux-popup-control-test", "alpha").Run(); err != nil {
		t.Fatalf("rename to alpha: %v", err)
	}
	// rename the auto-created window
	if err := tmuxCmd(socket1, "rename-window", "-t", "alpha:0", "editor").Run(); err != nil {
		t.Fatalf("rename alpha:0: %v", err)
	}
	if err := tmuxCmd(socket1, "new-window", "-t", "alpha", "-n", "server").Run(); err != nil {
		t.Fatalf("create alpha:server: %v", err)
	}
	if err := tmuxCmd(socket1, "split-window", "-t", "alpha:1", "-d").Run(); err != nil {
		t.Fatalf("split alpha:server: %v", err)
	}

	// create session "beta" with one window
	if err := tmuxCmd(socket1, "new-session", "-d", "-s", "beta", "-n", "main", "-x", "80", "-y", "24").Run(); err != nil {
		t.Fatalf("create beta: %v", err)
	}
	waitSession(t, socket1, "beta")

	// snapshot the source structure
	srcSessions := listSessionNames(t, socket1)
	t.Logf("source sessions: %v", srcSessions)
	if len(srcSessions) != 2 {
		t.Fatalf("expected 2 source sessions, got %d: %v", len(srcSessions), srcSessions)
	}

	srcAlphaWins := listWindows(t, socket1, "alpha")
	srcBetaWins := listWindows(t, socket1, "beta")
	srcAlpha1Panes := countPanes(t, socket1, "alpha:1")
	t.Logf("source alpha windows: %v, alpha:1 panes: %d", srcAlphaWins, srcAlpha1Panes)
	t.Logf("source beta windows: %v", srcBetaWins)

	// ── save ────────────────────────────────────────────────────────────

	saveDir := t.TempDir()
	saveCfg := Config{
		SocketPath:          socket1,
		SaveDir:             saveDir,
		CapturePaneContents: false,
		Name:                "roundtrip",
	}
	ch := Save(saveCfg)
	for ev := range ch {
		t.Logf("save: [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("save error: %v", ev.Err)
		}
	}

	// verify save file was written
	entries, err := ListSaves(saveDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no save file: err=%v entries=%d", err, len(entries))
	}
	savedPath := entries[0].Path
	t.Logf("saved to %s (%d sessions)", savedPath, entries[0].SessionCount)

	// read back the save file to verify contents
	sf, err := ReadSaveFile(savedPath)
	if err != nil {
		t.Fatalf("read save file: %v", err)
	}
	if len(sf.Sessions) != 2 {
		t.Fatalf("save file has %d sessions, want 2", len(sf.Sessions))
	}

	// shut down server 1's gotmuxcc connection before switching to server 2
	tmux.Shutdown()

	// ── server 2: restore into a fresh instance ─────────────────────────

	socket2, cleanup2, logDir2 := testutil.StartTmuxServer(t)
	defer cleanup2()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir2) })

	// record sessions before restore
	beforeSessions := listSessionNames(t, socket2)
	t.Logf("server 2 sessions before restore: %v", beforeSessions)

	restoreCfg := Config{
		SocketPath: socket2,
		SaveDir:    saveDir,
	}
	ch2 := Restore(restoreCfg, savedPath)
	for ev := range ch2 {
		t.Logf("restore: [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("restore error: %v", ev.Err)
		}
	}

	// shut down server 2's gotmuxcc connection
	tmux.Shutdown()

	// give tmux a moment to settle
	time.Sleep(200 * time.Millisecond)

	// ── compare ─────────────────────────────────────────────────────────

	afterSessions := listSessionNames(t, socket2)
	t.Logf("server 2 sessions after restore: %v", afterSessions)

	// the restored sessions should exist
	for _, name := range srcSessions {
		found := false
		for _, s := range afterSessions {
			if s == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("restored server missing session %q; has: %v", name, afterSessions)
		}
	}

	// count should be: original server-2 sessions + restored sessions.
	// the test server auto-creates "tmux-popup-control-test", which is not
	// in the save file, so it should remain untouched.
	expectedCount := len(beforeSessions) + len(srcSessions)
	if len(afterSessions) != expectedCount {
		t.Errorf("expected %d sessions after restore (%v before + %v restored), got %d: %v",
			expectedCount, beforeSessions, srcSessions, len(afterSessions), afterSessions)
	}

	// check for phantom sessions — any session not in beforeSessions or
	// srcSessions is unexpected
	known := make(map[string]bool)
	for _, s := range beforeSessions {
		known[s] = true
	}
	for _, s := range srcSessions {
		known[s] = true
	}
	for _, s := range afterSessions {
		if !known[s] {
			t.Errorf("phantom session detected: %q (not in before=%v or source=%v)", s, beforeSessions, srcSessions)
		}
	}

	// compare window structure
	dstAlphaWins := listWindows(t, socket2, "alpha")
	dstBetaWins := listWindows(t, socket2, "beta")
	dstAlpha1Panes := countPanes(t, socket2, "alpha:1")

	if fmt.Sprint(srcAlphaWins) != fmt.Sprint(dstAlphaWins) {
		t.Errorf("alpha windows mismatch: src=%v dst=%v", srcAlphaWins, dstAlphaWins)
	}
	if fmt.Sprint(srcBetaWins) != fmt.Sprint(dstBetaWins) {
		t.Errorf("beta windows mismatch: src=%v dst=%v", srcBetaWins, dstBetaWins)
	}
	if srcAlpha1Panes != dstAlpha1Panes {
		t.Errorf("alpha:1 pane count mismatch: src=%d dst=%d", srcAlpha1Panes, dstAlpha1Panes)
	}

	t.Logf("round-trip comparison passed: sessions=%v alpha_wins=%v beta_wins=%v alpha:1_panes=%d",
		afterSessions, dstAlphaWins, dstBetaWins, dstAlpha1Panes)
}

// TestRestoreWithUserConfigIntegration repeats the restore half of the
// round-trip test but boots the destination server with the user's real
// ~/.tmux.conf (including plugins). This catches phantom-session bugs that
// only appear when the user's config and plugins are loaded.
func TestRestoreWithUserConfigIntegration(t *testing.T) {
	testutil.RequireTmux(t)

	confPath := filepath.Join(os.Getenv("HOME"), ".tmux.conf")
	if _, err := os.Stat(confPath); err != nil {
		t.Skipf("skipping: no ~/.tmux.conf found")
	}

	// ── build source save file using a clean server ─────────────────────

	socket1, cleanup1, logDir1 := testutil.StartTmuxServer(t)
	defer cleanup1()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir1) })

	if err := tmuxCmd(socket1, "rename-session", "-t", "tmux-popup-control-test", "shells").Run(); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := tmuxCmd(socket1, "rename-window", "-t", "shells:0", "main").Run(); err != nil {
		t.Fatalf("rename window: %v", err)
	}

	saveDir := t.TempDir()
	ch := Save(Config{
		SocketPath: socket1,
		SaveDir:    saveDir,
		Name:       "userconf",
	})
	for ev := range ch {
		if ev.Err != nil {
			t.Fatalf("save error: %v", ev.Err)
		}
	}
	tmux.Shutdown()

	entries, err := ListSaves(saveDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no save file: %v", err)
	}
	savedPath := entries[0].Path

	// ── restore into server booted with real config ─────────────────────

	socket2, cleanup2, _ := startTmuxServerWithConfig(t, confPath)
	defer cleanup2()

	// wait for plugins to finish sourcing
	time.Sleep(2 * time.Second)

	beforeSessions := listSessionNames(t, socket2)
	t.Logf("sessions before restore (user config): %v", beforeSessions)

	ch2 := Restore(Config{SocketPath: socket2, SaveDir: saveDir}, savedPath)
	for ev := range ch2 {
		t.Logf("restore: [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("restore error: %v", ev.Err)
		}
	}
	tmux.Shutdown()
	time.Sleep(200 * time.Millisecond)

	afterSessions := listSessionNames(t, socket2)
	t.Logf("sessions after restore (user config): %v", afterSessions)

	// check for phantoms
	known := make(map[string]bool)
	for _, s := range beforeSessions {
		known[s] = true
	}
	known["shells"] = true // the one we restored
	for _, s := range afterSessions {
		if !known[s] {
			t.Errorf("phantom session detected: %q (before=%v, expected restored=[shells])", s, beforeSessions)
		}
	}

	expectedCount := len(beforeSessions) + 1 // +1 for "shells"
	if len(afterSessions) != expectedCount {
		t.Errorf("expected %d sessions, got %d: %v", expectedCount, len(afterSessions), afterSessions)
	}
}
