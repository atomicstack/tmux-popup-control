package testutil

import (
	"context"
	"strings"
	"testing"
	"time"
)

// windowMoveFixture starts an isolated server with a two-window destination
// session and launches the binary straight into window:move in a separate
// session, so the window being moved is the one running the binary.
func windowMoveFixture(t *testing.T) (socket, pane, exitFile, movingID string) {
	t.Helper()
	bin := BuildBinary(t)
	socket, cleanup, logDir := StartIsolatedTmuxServer(t)
	t.Cleanup(cleanup)
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	if err := TmuxCommand(socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", "mv-dest").Run(); err != nil {
		t.Fatalf("create dest session: %v", err)
	}
	if err := TmuxCommand(socket, "new-window", "-d", "-t", "mv-dest:").Run(); err != nil {
		t.Fatalf("new-window in dest: %v", err)
	}
	pane, exitFile = LaunchBinary(t, bin, socket, "mv-runner", "window:move")
	out, err := TmuxCommand(socket, "display-message", "-p", "-t", pane, "#{window_id}").Output()
	if err != nil {
		t.Fatalf("resolve runner window id: %v", err)
	}
	return socket, pane, exitFile, strings.TrimSpace(string(out))
}

func windowIndexIDs(t *testing.T, socket, session string) []string {
	t.Helper()
	out, err := TmuxCommand(socket, "list-windows", "-t", session, "-F", "#{window_index}=#{window_id}").Output()
	if err != nil {
		t.Fatalf("list-windows %s: %v", session, err)
	}
	return strings.Fields(string(out))
}

func TestWindowMoveIntoOtherSessionIntegration(t *testing.T) {
	socket, pane, exitFile, movingID := windowMoveFixture(t)

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	WaitForContent(t, ctx, socket, pane, "⇅ 0:")

	// The runner session sorts after mv-dest, so one step up lands after
	// mv-dest's last window, on the free index 2.
	SendKeys(t, socket, pane, "Up")
	WaitForContent(t, ctx, socket, pane, "⇅ 2:")
	SendKeys(t, socket, pane, "Enter")

	if code := waitForExit(t, ctx, exitFile); code != "0" {
		output, _ := CapturePane(t, socket, pane)
		t.Fatalf("binary exited with code %s; pane output:\n%s", code, output)
	}
	got := windowIndexIDs(t, socket, "mv-dest")
	if len(got) != 3 || got[2] != "2="+movingID {
		t.Fatalf("mv-dest windows = %v, want %s at index 2", got, movingID)
	}
}

func TestWindowMoveEscapeLeavesWindowIntegration(t *testing.T) {
	socket, pane, exitFile, movingID := windowMoveFixture(t)

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	WaitForContent(t, ctx, socket, pane, "⇅ 0:")
	SendKeys(t, socket, pane, "Up")
	WaitForContent(t, ctx, socket, pane, "⇅ 2:")
	SendKeys(t, socket, pane, "Escape")

	if code := waitForExit(t, ctx, exitFile); code != "0" {
		output, _ := CapturePane(t, socket, pane)
		t.Fatalf("binary exited with code %s; pane output:\n%s", code, output)
	}
	if got := windowIndexIDs(t, socket, "mv-dest"); len(got) != 2 {
		t.Fatalf("mv-dest windows = %v, want the original two", got)
	}
	if got := windowIndexIDs(t, socket, "mv-runner"); len(got) != 1 || got[0] != "0="+movingID {
		t.Fatalf("mv-runner windows = %v, want %s still at index 0", got, movingID)
	}
}

func TestWindowMoveTracksLiveWindowChangesIntegration(t *testing.T) {
	socket, pane, exitFile, movingID := windowMoveFixture(t)

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	WaitForContent(t, ctx, socket, pane, "⇅ 0:")
	SendKeys(t, socket, pane, "Up")
	WaitForContent(t, ctx, socket, pane, "⇅ 2:")

	// A window created elsewhere while the move screen is open must show up,
	// and the pinned window must stay anchored after mv-dest's (old) last
	// window, so it now lands between that window and the new one.
	if err := TmuxCommand(socket, "new-window", "-d", "-t", "mv-dest:5", "-n", "fresh").Run(); err != nil {
		t.Fatalf("new-window in dest: %v", err)
	}
	WaitForContent(t, ctx, socket, pane, "5: fresh")
	SendKeys(t, socket, pane, "Enter")

	if code := waitForExit(t, ctx, exitFile); code != "0" {
		output, _ := CapturePane(t, socket, pane)
		t.Fatalf("binary exited with code %s; pane output:\n%s", code, output)
	}
	got := windowIndexIDs(t, socket, "mv-dest")
	if len(got) != 4 || got[2] != "2="+movingID {
		t.Fatalf("mv-dest windows = %v, want %s at index 2", got, movingID)
	}
}
