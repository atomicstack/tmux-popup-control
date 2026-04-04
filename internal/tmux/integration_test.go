package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	testutil "github.com/atomicstack/tmux-popup-control/internal/testutil"
)

func TestFetchSnapshotsIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		testutil.AssertNoServerCrash(t, logDir)
	})
	t.Setenv("TMUX_TMPDIR", filepath.Dir(socket))

	sessionName := "tmux-integration"
	if err := NewSession(socket, sessionName); err != nil {
		t.Skipf("skipping: unable to create session (%v)", err)
	}
	waitForSession(t, socket, sessionName)

	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "#{session_name}")
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FORMAT", "#{window_name}")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_FORMAT", "#{pane_title}")

	sessions, err := FetchSessions(socket)
	if err != nil {
		t.Fatalf("FetchSessions failed: %v", err)
	}
	for _, sess := range sessions.Sessions {
		t.Logf("snapshot session: name=%q label=%q windows=%d attached=%v", sess.Name, sess.Label, sess.Windows, sess.Attached)
	}
	if !containsSession(sessions.Sessions, sessionName) {
		t.Fatalf("expected session %q in snapshot %#v", sessionName, sessions.Sessions)
	}

	windows, err := FetchWindows(socket)
	if err != nil {
		t.Fatalf("FetchWindows failed: %v", err)
	}
	for _, window := range windows.Windows {
		t.Logf("initial window: id=%q session=%q label=%q", window.ID, window.Session, window.Label)
	}
	win := firstWindowForSession(windows.Windows, sessionName)
	if win == nil {
		t.Fatalf("expected window for session %q, got %#v", sessionName, windows.Windows)
	}

	newWindowName := "integration-window"
	if err := RenameWindow(socket, win.ID, newWindowName); err != nil {
		t.Fatalf("RenameWindow failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	windowsAfter, err := FetchWindows(socket)
	if err != nil {
		t.Fatalf("FetchWindows after rename failed: %v", err)
	}
	for _, window := range windowsAfter.Windows {
		t.Logf("window after rename: id=%q session=%q label=%q", window.ID, window.Session, window.Label)
	}
	winAfter := firstWindowForSession(windowsAfter.Windows, sessionName)
	if winAfter == nil || !strings.Contains(winAfter.Label, newWindowName) {
		t.Fatalf("expected renamed window label with %q, got %#v", newWindowName, winAfter)
	}

	if err := exec.Command("tmux", "-S", socket, "new-window", "-t", sessionName, "-n", "temp-window").Run(); err != nil {
		t.Fatalf("failed to create window: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := KillWindows(socket, []string{" " + sessionName + ":1 "}); err != nil {
		t.Fatalf("KillWindows failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	windowsPostKill, err := FetchWindows(socket)
	if err != nil {
		t.Fatalf("FetchWindows after kill failed: %v", err)
	}
	for _, window := range windowsPostKill.Windows {
		t.Logf("window post kill: id=%q session=%q label=%q", window.ID, window.Session, window.Label)
	}
	if containsWindow(windowsPostKill.Windows, sessionName, 1) {
		t.Fatalf("expected window %s:1 to be gone", sessionName)
	}

	detachedSession := "tmux-detach"
	if err := NewSession(socket, detachedSession); err != nil {
		t.Fatalf("NewSession for detach failed: %v", err)
	}
	waitForSession(t, socket, detachedSession)
	if err := DetachSessions(socket, []string{" " + detachedSession + " "}); err != nil {
		t.Fatalf("DetachSessions failed: %v", err)
	}
	if err := KillSessions(socket, []string{detachedSession}); err != nil {
		t.Fatalf("KillSessions failed: %v", err)
	}
	if err := exec.Command("tmux", "-S", socket, "has-session", "-t", detachedSession).Run(); err == nil {
		t.Fatalf("expected session %q to be removed", detachedSession)
	}
}

func TestPanePreviewCapturesLiveCursorIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		testutil.AssertNoServerCrash(t, logDir)
	})

	sessionName := "cursor-preview"
	if err := exec.Command("tmux", "-S", socket, "new-session", "-d", "-s", sessionName, "cat").Run(); err != nil {
		t.Fatalf("failed to create cursor preview session: %v", err)
	}
	waitForSession(t, socket, sessionName)

	paneOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", sessionName, "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("get pane id: %v", err)
	}
	paneID := strings.TrimSpace(string(paneOut))
	if paneID == "" {
		t.Fatal("expected pane id")
	}

	testutil.SendText(t, socket, paneID, "abcd")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	testutil.WaitForContent(t, ctx, socket, paneID, "abcd")

	data, err := PanePreview(socket, paneID)
	if err != nil {
		t.Fatalf("PanePreview failed: %v", err)
	}
	if !data.CursorVisible {
		t.Fatalf("expected visible cursor, got %+v", data)
	}
	if data.CursorX != 4 {
		t.Fatalf("expected cursor x=4 after typing abcd, got %+v", data)
	}
	if len(data.Lines) == 0 {
		t.Fatalf("expected preview lines, got %+v", data)
	}
	if !strings.Contains(strings.Join(data.Lines, "\n"), "abcd") {
		t.Fatalf("expected preview to contain typed text, got %+v", data)
	}
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return out
}

func waitForSession(t *testing.T, socket, session string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := exec.Command("tmux", "-S", socket, "has-session", "-t", session).Run(); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("session %q did not appear in time", session)
}

func containsSession(sessions []Session, name string) bool {
	for _, s := range sessions {
		if s.Name == name {
			return true
		}
	}
	return false
}

func firstWindowForSession(windows []Window, session string) *Window {
	for i := range windows {
		if windows[i].Session == session {
			return &windows[i]
		}
	}
	return nil
}

func containsWindow(windows []Window, session string, index int) bool {
	search := fmt.Sprintf("%s:%d", session, index)
	for _, w := range windows {
		if w.Session == session && w.Index == index {
			return true
		}
		if w.ID == search {
			return true
		}
	}
	return false
}

// TestCurrentClientIDSkipsControlModeClients verifies that CurrentClientID
// returns empty when only control-mode clients are attached (no TTY clients).
// In production (inside a popup), a real TTY client would be present and
// CurrentClientID would return its name.
func TestCurrentClientIDSkipsControlModeClients(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		testutil.AssertNoServerCrash(t, logDir)
	})

	// Clear any cached control-mode connection.
	Shutdown()

	initialSession := "tmux-popup-control-test"

	// Get a pane ID from the initial session.
	paneOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", initialSession, "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("get pane id: %v", err)
	}
	paneID := strings.TrimSpace(string(paneOut))
	t.Logf("test pane: %s (session: %s)", paneID, initialSession)

	// Set TMUX_PANE as CurrentClientID expects.
	t.Setenv("TMUX_PANE", paneID)

	// Call CurrentClientID — should return empty because only
	// control-mode clients exist (no real TTY clients in the test env).
	clientID := CurrentClientID(socket)
	t.Logf("CurrentClientID returned: %q", clientID)

	// List clients to confirm only control-mode clients are present.
	clientsOut, _ := exec.Command("tmux", "-S", socket, "list-clients", "-F", "#{client_name} control=#{client_control_mode} session=#{client_session}").Output()
	t.Logf("tmux clients:\n%s", strings.TrimSpace(string(clientsOut)))

	// With no real TTY clients, CurrentClientID should return empty.
	if clientID != "" {
		t.Errorf("expected empty clientID when no TTY clients exist, got %q", clientID)
	}

	Shutdown()
}

// TestSwitchClientWithoutClientIDIntegration verifies that SwitchClient
// works via control mode when no explicit clientID is available. In this
// mode, switch-client targets the control-mode connection itself, which
// TestGotmuxccConnectionDoesNotCreateSessionIntegration verifies that
// establishing a gotmuxcc control-mode connection to an existing server
// does NOT create an extra session. This is a regression test for a bug
// where users ended up with a phantom numeric session after restore.
func TestGotmuxccConnectionDoesNotCreateSessionIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		testutil.AssertNoServerCrash(t, logDir)
	})

	Shutdown()

	// list sessions before connecting — should be exactly one (auto-created).
	beforeOut, err := exec.Command("tmux", "-S", socket, "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatalf("list-sessions before: %v", err)
	}
	before := nonEmptyLines(string(beforeOut))
	t.Logf("sessions before gotmuxcc connect: %v", before)

	// establish a gotmuxcc connection via newTmux.
	client, err := newTmux(socket)
	if err != nil {
		t.Fatalf("newTmux: %v", err)
	}
	_ = client

	// give the connection a moment to stabilise.
	time.Sleep(500 * time.Millisecond)

	// list sessions after connecting — should be unchanged.
	afterOut, err := exec.Command("tmux", "-S", socket, "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatalf("list-sessions after: %v", err)
	}
	after := nonEmptyLines(string(afterOut))
	t.Logf("sessions after gotmuxcc connect: %v", after)

	if len(after) != len(before) {
		t.Errorf("gotmuxcc connection created extra session(s): before=%v after=%v", before, after)
	}

	// now create a named session — should result in exactly 2.
	if err := NewSession(socket, "test-named"); err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	waitForSession(t, socket, "test-named")

	finalOut, err := exec.Command("tmux", "-S", socket, "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		t.Fatalf("list-sessions final: %v", err)
	}
	final := nonEmptyLines(string(finalOut))
	t.Logf("sessions after NewSession: %v", final)

	expected := len(before) + 1
	if len(final) != expected {
		t.Errorf("expected %d sessions, got %d: %v", expected, len(final), final)
	}

	Shutdown()
}

// TestFetchPanesCurrentPaneMultiSessionIntegration verifies that FetchPanes
// identifies the correct current pane when multiple sessions exist. This
// reproduces a bug where, inside display-popup (TMUX_PANE empty), the current
// pane resolves to a low-numbered pane (e.g. %0) from the wrong session
// instead of the actual active pane (e.g. %4) in the host session.
//
// The bug occurs because: (1) currentSessionName falls back to the first
// client from ListClients, which may belong to a different session, and
// (2) the tmux format "session_attached" is false for all sessions in the
// test env (no real TTY clients), so the Active fallback picks the first
// active pane it finds — which is in whichever session appears first.
func TestFetchPanesCurrentPaneMultiSessionIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		testutil.AssertNoServerCrash(t, logDir)
	})

	Shutdown()

	// StartTmuxServer creates "tmux-popup-control-test" (%0) as the initial
	// session. Create a second session — this will get higher pane IDs.
	// We'll pretend the popup belongs to the SECOND session. The bug is
	// that FetchPanes picks the first session's active pane instead.
	//
	// Name the target session so it sorts AFTER the initial session to
	// ensure the initial session's panes appear first in list-panes output,
	// reproducing the ordering the user sees in production.
	targetSession := "zzz-target"
	if err := exec.Command("tmux", "-S", socket, "new-session", "-d", "-s", targetSession).Run(); err != nil {
		t.Fatalf("create target session: %v", err)
	}
	waitForSession(t, socket, targetSession)

	// Split a few times to create panes with higher IDs.
	for i := 0; i < 3; i++ {
		if err := exec.Command("tmux", "-S", socket, "split-window", "-t", targetSession).Run(); err != nil {
			t.Fatalf("split-window %d: %v", i, err)
		}
	}
	time.Sleep(200 * time.Millisecond)

	// Find the active pane ID in the target session via tmux directly.
	activeOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", targetSession, "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("get active pane: %v", err)
	}
	targetPaneID := strings.TrimSpace(string(activeOut))
	t.Logf("active pane in %s: %s", targetSession, targetPaneID)

	// Also get the active pane in the initial session for comparison.
	initialSession := "tmux-popup-control-test"
	initialOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", initialSession, "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("get initial pane: %v", err)
	}
	initialPaneID := strings.TrimSpace(string(initialOut))
	t.Logf("active pane in %s: %s", initialSession, initialPaneID)

	if targetPaneID == initialPaneID {
		t.Fatalf("expected different active panes in different sessions, both are %s", targetPaneID)
	}

	// Simulate the display-popup environment: TMUX_PANE is empty.
	t.Setenv("TMUX_PANE", "")
	// Set TMUX to point at the target session's ID.
	sessionIDOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", targetSession, "-p", "#{session_id}").Output()
	if err != nil {
		t.Fatalf("get session id: %v", err)
	}
	sessionID := strings.TrimSpace(string(sessionIDOut))
	t.Logf("target session ID: %s", sessionID)

	pidOut, err := exec.Command("tmux", "-S", socket, "display-message", "-p", "#{pid}").Output()
	if err != nil {
		t.Fatalf("get pid: %v", err)
	}
	pid := strings.TrimSpace(string(pidOut))
	tmuxEnv := fmt.Sprintf("%s,%s,%s", socket, pid, strings.TrimPrefix(sessionID, "$"))
	t.Setenv("TMUX", tmuxEnv)
	t.Logf("TMUX=%s", tmuxEnv)

	t.Setenv("TMUX_POPUP_CONTROL_PANE_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SESSION", targetSession)
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_ID", strings.TrimPrefix(sessionID, "$"))

	snap, err := FetchPanes(socket)
	if err != nil {
		t.Fatalf("FetchPanes: %v", err)
	}

	t.Logf("FetchPanes returned CurrentID=%q CurrentLabel=%q CurrentWindow=%q",
		snap.CurrentID, snap.CurrentLabel, snap.CurrentWindow)
	for _, p := range snap.Panes {
		t.Logf("  pane: id=%q paneID=%q session=%q current=%v active=%v",
			p.ID, p.PaneID, p.Session, p.Current, p.Active)
	}

	// Find the pane that FetchPanes considers "current" — either via the
	// Current field or via snapshot.CurrentID (set by the Active fallback).
	if snap.CurrentID == "" {
		t.Fatalf("FetchPanes did not identify any current pane")
	}

	// Look up the pane entry matching CurrentID.
	var currentPane *Pane
	for i := range snap.Panes {
		if snap.Panes[i].ID == snap.CurrentID {
			currentPane = &snap.Panes[i]
			break
		}
	}
	if currentPane == nil {
		t.Fatalf("CurrentID %q not found in pane entries", snap.CurrentID)
	}

	if currentPane.Session != targetSession {
		t.Fatalf("current pane is in session %q, want %q (pane %s instead of one in %s)",
			currentPane.Session, targetSession, currentPane.PaneID, targetSession)
	}

	if currentPane.PaneID != targetPaneID {
		t.Fatalf("current pane is %s, want %s (the active pane in %s)",
			currentPane.PaneID, targetPaneID, targetSession)
	}

	Shutdown()
}

// TestFetchPanesCurrentPaneMultiWindowIntegration verifies that FetchPanes
// identifies the correct current pane when the host session has multiple
// windows and the active window is NOT the first one.
//
// Reproduces a bug where pane_id always resolves to %1 regardless of which
// window the popup is opened from, because FetchPanes infers the current
// pane via window_active/pane_active tmux formats rather than using the
// pane ID captured by main.sh before the popup opened.
func TestFetchPanesCurrentPaneMultiWindowIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		testutil.AssertNoServerCrash(t, logDir)
	})

	Shutdown()

	// StartTmuxServer creates "tmux-popup-control-test" as the initial
	// session with one window (pane %0). Create additional windows so
	// the session has three windows total.
	session := "tmux-popup-control-test"
	for i := 0; i < 2; i++ {
		if err := exec.Command("tmux", "-S", socket, "new-window", "-t", session).Run(); err != nil {
			t.Fatalf("new-window %d: %v", i, err)
		}
	}
	time.Sleep(200 * time.Millisecond)

	// Select the last window (index 2) so window 0 is NOT active.
	if err := exec.Command("tmux", "-S", socket, "select-window", "-t", session+":2").Run(); err != nil {
		t.Fatalf("select-window: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Get the pane ID of the active pane in the now-active window.
	activeOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", session, "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("get active pane: %v", err)
	}
	targetPaneID := strings.TrimSpace(string(activeOut))
	t.Logf("active pane in %s (window 2): %s", session, targetPaneID)

	// Also get the pane ID in window 0 — this is the WRONG pane that the
	// old code would pick via the Active fallback.
	wrongOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", session+":0", "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("get window-0 pane: %v", err)
	}
	wrongPaneID := strings.TrimSpace(string(wrongOut))
	t.Logf("pane in %s window 0 (wrong): %s", session, wrongPaneID)

	if targetPaneID == wrongPaneID {
		t.Fatalf("test setup error: target and wrong pane IDs are the same: %s", targetPaneID)
	}

	// Get the session ID.
	sidOut, err := exec.Command("tmux", "-S", socket, "display-message", "-t", session, "-p", "#{session_id}").Output()
	if err != nil {
		t.Fatalf("get session id: %v", err)
	}
	sessionID := strings.TrimSpace(string(sidOut))
	t.Logf("session ID: %s", sessionID)

	// Simulate the display-popup environment.
	t.Setenv("TMUX_PANE", "")
	pidOut, err := exec.Command("tmux", "-S", socket, "display-message", "-p", "#{pid}").Output()
	if err != nil {
		t.Fatalf("get pid: %v", err)
	}
	pid := strings.TrimSpace(string(pidOut))
	tmuxEnv := fmt.Sprintf("%s,%s,%s", socket, pid, strings.TrimPrefix(sessionID, "$"))
	t.Setenv("TMUX", tmuxEnv)

	t.Setenv("TMUX_POPUP_CONTROL_PANE_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SESSION", session)
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_ID", strings.TrimPrefix(sessionID, "$"))
	// This is the key env var — main.sh captures the pane ID before the
	// popup opens, so FetchPanes should use it directly.
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", targetPaneID)

	snap, err := FetchPanes(socket)
	if err != nil {
		t.Fatalf("FetchPanes: %v", err)
	}

	t.Logf("FetchPanes returned CurrentID=%q CurrentLabel=%q CurrentWindow=%q",
		snap.CurrentID, snap.CurrentLabel, snap.CurrentWindow)
	for _, p := range snap.Panes {
		t.Logf("  pane: id=%q paneID=%q session=%q current=%v active=%v",
			p.ID, p.PaneID, p.Session, p.Current, p.Active)
	}

	if snap.CurrentID == "" {
		t.Fatalf("FetchPanes did not identify any current pane")
	}

	var currentPane *Pane
	for i := range snap.Panes {
		if snap.Panes[i].ID == snap.CurrentID {
			currentPane = &snap.Panes[i]
			break
		}
	}
	if currentPane == nil {
		t.Fatalf("CurrentID %q not found in pane entries", snap.CurrentID)
	}

	if currentPane.PaneID != targetPaneID {
		t.Fatalf("current pane is %s, want %s (the active pane in window 2, not window 0's %s)",
			currentPane.PaneID, targetPaneID, wrongPaneID)
	}

	Shutdown()
}

// has no visible effect — but the command should not error.
func TestSwitchClientWithoutClientIDIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		testutil.AssertNoServerCrash(t, logDir)
	})

	Shutdown()

	// Create a target session.
	targetSession := "switch-target"
	if err := exec.Command("tmux", "-S", socket, "new-session", "-d", "-s", targetSession).Run(); err != nil {
		t.Fatalf("create target session: %v", err)
	}
	waitForSession(t, socket, targetSession)

	// SwitchClient with empty clientID — will skip -c flag.
	err := SwitchClient(socket, "", targetSession)
	if err != nil {
		t.Fatalf("SwitchClient returned error: %v", err)
	}
	t.Logf("SwitchClient with empty clientID succeeded (no error)")

	Shutdown()
}
