package resurrect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	for line := range strings.SplitSeq(string(out), "\n") {
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
	for line := range strings.SplitSeq(string(out), "\n") {
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
	for line := range strings.SplitSeq(string(out), "\n") {
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
		if !slices.Contains(afterSessions, name) {
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

// TestSessionOptionRoundTripIntegration verifies that a session option set
// via control mode can be immediately read back via display-message. This
// isolates the marker read-back path from the full restore machinery.
func TestSessionOptionRoundTripIntegration(t *testing.T) {
	testutil.RequireTmux(t)

	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	session := "tmux-popup-control-test"
	optionKey := "@tmux-popup-control-test-marker"

	// verify option is initially unset
	val := tmux.SessionOption(socket, session, optionKey)
	if val != "" {
		t.Fatalf("option should be empty initially, got %q", val)
	}

	// set the option
	if err := tmux.SetSessionOption(socket, session, optionKey, "1"); err != nil {
		t.Fatalf("SetSessionOption: %v", err)
	}

	// read it back immediately
	val = tmux.SessionOption(socket, session, optionKey)
	t.Logf("SessionOption returned %q after set", val)
	if val != "1" {
		t.Errorf("expected %q, got %q", "1", val)
	}

	// close and reopen the connection, read again
	tmux.Shutdown()

	val = tmux.SessionOption(socket, session, optionKey)
	t.Logf("SessionOption returned %q after reconnect", val)
	if val != "1" {
		t.Errorf("expected %q after reconnect, got %q", "1", val)
	}

	// also verify via raw tmux CLI as ground truth
	out, err := tmuxCmd(socket, "display-message", "-t", session+":", "-p", "#{"+optionKey+"}").Output()
	if err != nil {
		t.Fatalf("tmux display-message: %v", err)
	}
	cliVal := strings.TrimSpace(string(out))
	t.Logf("raw tmux CLI returned %q", cliVal)
	if cliVal != "1" {
		t.Errorf("raw CLI: expected %q, got %q", "1", cliVal)
	}
}

// countWindows returns the number of windows in a session.
func countWindows(t *testing.T, socket, session string) int {
	t.Helper()
	out, err := tmuxCmd(socket, "list-windows", "-t", session, "-F", "#{window_index}").Output()
	if err != nil {
		t.Fatalf("list-windows %s: %v", session, err)
	}
	n := 0
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// TestRestoreMergeIdempotentIntegration verifies that restoring the same save
// file twice into a server where the session already exists does not create
// duplicate windows on the second restore. This exercises the real
// SessionOption/SetSessionOption code path through control mode.
func TestRestoreMergeIdempotentIntegration(t *testing.T) {
	testutil.RequireTmux(t)

	// ── build a save file from a clean server ────────────────────────────

	socket1, cleanup1, logDir1 := testutil.StartTmuxServer(t)
	defer cleanup1()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir1) })

	if err := tmuxCmd(socket1, "rename-session", "-t", "tmux-popup-control-test", "work").Run(); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := tmuxCmd(socket1, "rename-window", "-t", "work:0", "editor").Run(); err != nil {
		t.Fatalf("rename window: %v", err)
	}
	if err := tmuxCmd(socket1, "new-window", "-t", "work", "-n", "server").Run(); err != nil {
		t.Fatalf("new-window: %v", err)
	}

	saveDir := t.TempDir()
	ch := Save(Config{
		SocketPath: socket1,
		SaveDir:    saveDir,
		Name:       "idempotent",
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

	// ── restore into a server that already has session "work" ────────────

	socket2, cleanup2, logDir2 := testutil.StartTmuxServer(t)
	defer cleanup2()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir2) })

	// create an existing "work" session with one window
	if err := tmuxCmd(socket2, "new-session", "-d", "-s", "work", "-n", "existing", "-x", "80", "-y", "24").Run(); err != nil {
		t.Fatalf("create work: %v", err)
	}
	waitSession(t, socket2, "work")

	// first restore — should merge (append windows)
	restoreCfg := Config{SocketPath: socket2, SaveDir: saveDir}
	ch1 := Restore(restoreCfg, savedPath)
	for ev := range ch1 {
		t.Logf("restore 1: [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("restore 1 error: %v", ev.Err)
		}
	}
	tmux.Shutdown()
	time.Sleep(200 * time.Millisecond)

	windowsAfterFirst := countWindows(t, socket2, "work")
	t.Logf("windows after first restore: %d", windowsAfterFirst)

	// the saved session had 2 windows; existing had 1; merged total = 3
	if windowsAfterFirst != 3 {
		t.Errorf("expected 3 windows after first restore, got %d", windowsAfterFirst)
	}

	// second restore (new connection) — should be idempotent
	ch2 := Restore(restoreCfg, savedPath)
	for ev := range ch2 {
		t.Logf("restore 2: [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("restore 2 error: %v", ev.Err)
		}
	}
	tmux.Shutdown()
	time.Sleep(200 * time.Millisecond)

	windowsAfterSecond := countWindows(t, socket2, "work")
	t.Logf("windows after second restore: %d", windowsAfterSecond)

	if windowsAfterSecond != windowsAfterFirst {
		t.Errorf("idempotency violation (new conn): %d windows after first restore, %d after second (expected equal)",
			windowsAfterFirst, windowsAfterSecond)
	}

	// third restore (same connection, no Shutdown) — tests same-connection idempotency
	ch3 := Restore(restoreCfg, savedPath)
	for ev := range ch3 {
		t.Logf("restore 3 (same conn): [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("restore 3 error: %v", ev.Err)
		}
	}

	windowsAfterThird := countWindows(t, socket2, "work")
	t.Logf("windows after third restore (same conn): %d", windowsAfterThird)

	if windowsAfterThird != windowsAfterFirst {
		t.Errorf("idempotency violation (same conn): %d windows after first restore, %d after third (expected equal)",
			windowsAfterFirst, windowsAfterThird)
	}
	tmux.Shutdown()
}

// sessionPath queries #{session_path} for a session on the given socket.
func sessionPath(t *testing.T, socket, session string) string {
	t.Helper()
	out, err := tmuxCmd(socket, "display-message", "-t", session+":", "-p", "#{session_path}").Output()
	if err != nil {
		t.Fatalf("display-message session_path for %s: %v", session, err)
	}
	return strings.TrimSpace(string(out))
}

// paneCurrentPath queries #{pane_current_path} for a pane on the given socket.
func paneCurrentPath(t *testing.T, socket, target string) string {
	t.Helper()
	out, err := tmuxCmd(socket, "display-message", "-t", target, "-p", "#{pane_current_path}").Output()
	if err != nil {
		t.Fatalf("display-message pane_current_path for %s: %v", target, err)
	}
	return strings.TrimSpace(string(out))
}

// TestRestoreSessionPathIntegration verifies that after a save→restore cycle,
// the restored session's session_path is $HOME (the tmux default), NOT the
// working directory of the first restored pane. This matters because tmux
// uses session_path as the default directory for new windows.
func TestRestoreSessionPathIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("skipping: $HOME not set")
	}
	// resolve symlinks so comparison works on macOS (/var → /private/var)
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}

	// ── server 1: build a session with windows in different directories ──

	socket1, cleanup1, logDir1 := testutil.StartTmuxServer(t)
	defer cleanup1()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir1) })

	// create temp directories to simulate per-window working dirs.
	// resolve symlinks because macOS /var → /private/var and tmux
	// reports the resolved path in pane_current_path.
	dirA, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("eval symlinks dirA: %v", err)
	}
	dirB, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("eval symlinks dirB: %v", err)
	}

	// rename default session and set up windows with specific directories
	if err := tmuxCmd(socket1, "rename-session", "-t", "tmux-popup-control-test", "work").Run(); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// kill the auto-created window (running sleep) and replace it with a
	// shell in dirA so that pane_current_path is actually set
	if err := tmuxCmd(socket1, "respawn-pane", "-k", "-t", "work:0", "-c", dirA).Run(); err != nil {
		t.Fatalf("respawn work:0 in dirA: %v", err)
	}
	if err := tmuxCmd(socket1, "rename-window", "-t", "work:0", "alpha").Run(); err != nil {
		t.Fatalf("rename work:0: %v", err)
	}

	// second window in dirB
	if err := tmuxCmd(socket1, "new-window", "-t", "work", "-n", "beta", "-c", dirB).Run(); err != nil {
		t.Fatalf("new-window beta: %v", err)
	}

	// give shells a moment to initialize and set pane_current_path
	time.Sleep(500 * time.Millisecond)

	// verify source directories are what we expect
	srcPathAlpha := paneCurrentPath(t, socket1, "work:0.0")
	srcPathBeta := paneCurrentPath(t, socket1, "work:1.0")
	t.Logf("source pane dirs: alpha=%s beta=%s", srcPathAlpha, srcPathBeta)

	if srcPathAlpha != dirA {
		t.Fatalf("expected alpha pane in %s, got %s", dirA, srcPathAlpha)
	}
	if srcPathBeta != dirB {
		t.Fatalf("expected beta pane in %s, got %s", dirB, srcPathBeta)
	}

	// verify original session_path (should be $HOME from session creation,
	// but the test server creates with -f /dev/null so it may vary)
	srcSessionPath := sessionPath(t, socket1, "work")
	t.Logf("source session_path: %s", srcSessionPath)

	// ── save ─────────────────────────────────────────────────────────────

	saveDir := t.TempDir()
	ch := Save(Config{
		SocketPath: socket1,
		SaveDir:    saveDir,
		Name:       "session-path-test",
	})
	for ev := range ch {
		t.Logf("save: [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("save error: %v", ev.Err)
		}
	}
	tmux.Shutdown()

	entries, err := ListSaves(saveDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("no save file: err=%v", err)
	}
	savedPath := entries[0].Path

	// ── server 2: restore into a fresh instance ──────────────────────────

	socket2, cleanup2, logDir2 := testutil.StartTmuxServer(t)
	defer cleanup2()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir2) })

	ch2 := Restore(Config{SocketPath: socket2, SaveDir: saveDir}, savedPath)
	for ev := range ch2 {
		t.Logf("restore: [%d/%d] %s", ev.Step, ev.Total, ev.Message)
		if ev.Err != nil {
			t.Fatalf("restore error: %v", ev.Err)
		}
	}
	tmux.Shutdown()
	time.Sleep(200 * time.Millisecond)

	// ── verify session_path is $HOME, not a restored pane's dir ──────────

	restoredSessionPath := sessionPath(t, socket2, "work")
	t.Logf("restored session_path: %s (want: %s)", restoredSessionPath, home)

	if restoredSessionPath != home {
		t.Errorf("session_path after restore = %q, want %q ($HOME); "+
			"new windows created by attached clients would incorrectly "+
			"start in the restored pane's directory",
			restoredSessionPath, home)
	}

	// verify restored panes still have their original working directories
	time.Sleep(300 * time.Millisecond)
	restoredAlphaPath := paneCurrentPath(t, socket2, "work:0.0")
	restoredBetaPath := paneCurrentPath(t, socket2, "work:1.0")
	t.Logf("restored pane dirs: alpha=%s beta=%s", restoredAlphaPath, restoredBetaPath)

	if restoredAlphaPath != dirA {
		t.Errorf("restored alpha pane_current_path = %q, want %q", restoredAlphaPath, dirA)
	}
	if restoredBetaPath != dirB {
		t.Errorf("restored beta pane_current_path = %q, want %q", restoredBetaPath, dirB)
	}
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
