package testutil

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestPopupPaneHiddenFromTreeIntegration launches the binary the way main.sh
// does — in a modal floating pane opened over a host pane, with the host
// recorded in TMUX_POPUP_CONTROL_PANE_ID — and verifies the popup's own pane
// is not listed: the host window shows one pane, not two.
func TestPopupPaneHiddenFromTreeIntegration(t *testing.T) {
	bin := BuildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	const session = "popup-host"
	if err := TmuxCommand(socket, "new-session", "-d", "-s", session, "-x", "200", "-y", "50", "-c", "/", "sleep 300").Run(); err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer func() { _ = TmuxCommand(socket, "kill-session", "-t", session).Run() }()
	hostOut, err := TmuxCommand(socket, "display-message", "-p", "-t", session+":", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("host pane id: %v", err)
	}
	host := strings.TrimSpace(string(hostOut))

	popupCmd := bin + " -socket " + socket + " -width 120 -height 30 2>/dev/null; sleep 300"
	popupOut, err := TmuxCommand(socket, "new-pane", "-d", "-P", "-F", "#{pane_id}", "-t", host,
		"-O", "-K", "-x", "120", "-y", "30", "-X", "2", "-Y", "2",
		"-e", "TMUX_POPUP_CONTROL_PANE_ID="+host,
		"-e", "TMUX_POPUP_CONTROL_SESSION="+session,
		"-e", "TMUX_POPUP_CONTROL_ROOT_MENU=session:tree",
		"-e", "TMUX_POPUP_CONTROL_MENU_ARGS=expanded",
		"-e", "TMUX_POPUP_CONTROL_NO_PREVIEW=1",
		"-e", "TMUX_POPUP_CONTROL_COLOR_PROFILE=ascii",
		popupCmd).Output()
	if err != nil {
		t.Skipf("skipping: tmux cannot open a modal floating pane: %v", err)
	}
	popup := strings.TrimSpace(string(popupOut))

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	output := WaitForContent(t, ctx, socket, popup, session)
	output = WaitForContent(t, ctx, socket, popup, "pane)")
	t.Logf("tree:\n%s", output)

	if strings.Contains(output, "(2 panes)") {
		t.Fatalf("host window lists the popup as a second pane:\n%s", output)
	}
	if !strings.Contains(output, "(1 pane)") {
		t.Fatalf("expected the host window to show one pane:\n%s", output)
	}
}
