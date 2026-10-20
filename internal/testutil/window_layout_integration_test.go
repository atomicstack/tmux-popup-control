package testutil

import (
	"context"
	"strings"
	"testing"
	"time"
)

func windowLayout(t *testing.T, socket, target string) string {
	t.Helper()
	out, err := TmuxCommand(socket, "display-message", "-p", "-t", target, "#{window_layout}").Output()
	if err != nil {
		t.Fatalf("window_layout %s: %v", target, err)
	}
	return strings.TrimSpace(string(out))
}

// TestWindowLayoutAppliesToHostWindowIntegration is the regression test for
// window:layout changing nothing in the window the popup was opened over.
// select-layout was sent with no target over the control connection, which
// tmux resolves against the control client's own session, not the user's.
// A decoy session that sorts first makes that difference visible.
func TestWindowLayoutAppliesToHostWindowIntegration(t *testing.T) {
	socket, cleanup, logDir := StartIsolatedTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })
	installPlugin(t, socket)

	for _, session := range []string{"aaa-decoy", "zzz-host"} {
		if err := TmuxCommand(socket, "new-session", "-d", "-s", session, "-x", "160", "-y", "48", "sleep 300").Run(); err != nil {
			t.Fatalf("create %s: %v", session, err)
		}
		// two side-by-side panes: even-horizontal, so even-vertical is a change.
		if err := TmuxCommand(socket, "split-window", "-h", "-t", session+":", "sleep 300").Run(); err != nil {
			t.Fatalf("split %s: %v", session, err)
		}
	}
	hostOut, err := TmuxCommand(socket, "display-message", "-p", "-t", "zzz-host:", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("host pane id: %v", err)
	}
	host := strings.TrimSpace(string(hostOut))
	hostBefore := windowLayout(t, socket, "zzz-host:")
	decoyBefore := windowLayout(t, socket, "aaa-decoy:")

	launch := "#{E:@tmux-popup-control-launch} --root-menu window:layout"
	if out, err := TmuxCommand(socket, "run-shell", "-t", host, "-C", launch).CombinedOutput(); err != nil {
		t.Fatalf("launch: %v\n%s", err, out)
	}
	popup := waitForPopupPane(t, socket, host)

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	WaitForContent(t, ctx, socket, popup, "even-vertical")
	SendText(t, socket, popup, "even-vertical")
	WaitForContent(t, ctx, socket, popup, "even-vertical")
	SendKeys(t, socket, popup, "Enter")

	deadline := time.Now().Add(5 * time.Second)
	for windowLayout(t, socket, "zzz-host:") == hostBefore && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if got := windowLayout(t, socket, "aaa-decoy:"); got != decoyBefore {
		t.Errorf("the layout was applied to another session's window: %s -> %s", decoyBefore, got)
	}
	if got := windowLayout(t, socket, "zzz-host:"); got == hostBefore {
		t.Fatalf("the host window's layout did not change: still %s", got)
	}
}

// waitForPopupPane returns the modal floating popup pane opened over host.
func waitForPopupPane(t *testing.T, socket, host string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out, _ := TmuxCommand(socket, "list-panes", "-t", host, "-F", "#{pane_id} #{pane_floating_flag} #{pane_modal_flag} #{pane_title}").Output()
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if fields := strings.Fields(line); len(fields) == 4 && fields[1] == "1" && fields[2] == "1" && fields[3] == "tmux-popup-control" {
				return fields[0]
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("no modal floating popup pane appeared")
	return ""
}
