package tmux

import (
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
