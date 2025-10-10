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
	socket, cleanup := testutil.StartTmuxServer(t)
	defer cleanup()
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
	if !containsSession(sessions.Sessions, sessionName) {
		t.Fatalf("expected session %q in snapshot %#v", sessionName, sessions.Sessions)
	}

	windows, err := FetchWindows(socket)
	if err != nil {
		t.Fatalf("FetchWindows failed: %v", err)
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
