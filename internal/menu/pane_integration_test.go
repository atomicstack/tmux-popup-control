package menu

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// TestPaneJoinActionMovesPaneToTargetWindowIntegration reproduces the bug
// reported by the user: selecting a pane to join into the current window
// caused the pane to "disappear" — it actually went to whatever window the
// control-mode client happened to consider current, not the user's current
// window. The action computes target=ctx.CurrentPaneID but historically
// passed "" through to move-pane, letting tmux pick the destination from
// the control-mode client's state instead of honouring the caller.
func TestPaneJoinActionMovesPaneToTargetWindowIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	// Reset any cached control-mode connection so this test owns the lifecycle.
	tmux.Shutdown()
	t.Cleanup(tmux.Shutdown)

	session := "pane-join-test"
	if err := exec.Command("tmux", "-S", socket, "new-session", "-d", "-s", session).Run(); err != nil {
		t.Fatalf("new-session: %v", err)
	}
	// Create two more windows so the session has windows 0, 1, 2.
	if err := exec.Command("tmux", "-S", socket, "new-window", "-t", session+":1").Run(); err != nil {
		t.Fatalf("new-window 1: %v", err)
	}
	if err := exec.Command("tmux", "-S", socket, "new-window", "-t", session+":2").Run(); err != nil {
		t.Fatalf("new-window 2: %v", err)
	}
	// Force window 2 to be the "active" window for the session. This is
	// the window the control-mode connection will treat as current when it
	// attaches — distinct from window 0 (the user's intended target).
	if err := exec.Command("tmux", "-S", socket, "select-window", "-t", session+":2").Run(); err != nil {
		t.Fatalf("select-window 2: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	targetPane := paneIDFor(t, socket, session+":0")
	sourcePane := paneIDFor(t, socket, session+":1")
	trapPane := paneIDFor(t, socket, session+":2")
	t.Logf("target=%s (win 0)  source=%s (win 1)  trap=%s (win 2, currently active)",
		targetPane, sourcePane, trapPane)

	if targetPane == sourcePane || targetPane == trapPane || sourcePane == trapPane {
		t.Fatalf("expected distinct pane IDs, got target=%s source=%s trap=%s",
			targetPane, sourcePane, trapPane)
	}

	ctx := Context{SocketPath: socket, CurrentPaneID: targetPane}
	item := Item{ID: sourcePane, Label: sourcePane}

	msg := PaneJoinAction(ctx, item)()
	res, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if res.Err != nil {
		t.Fatalf("PaneJoinAction returned error: %v", res.Err)
	}

	time.Sleep(200 * time.Millisecond)

	panesInTargetWindow := paneIDsIn(t, socket, session+":0")
	if !slices.Contains(panesInTargetWindow, sourcePane) {
		// Diagnostic: where did the pane actually land?
		all, _ := exec.Command("tmux", "-S", socket, "list-panes", "-s", "-t", session,
			"-F", "win=#{window_index} pane=#{pane_id}").CombinedOutput()
		t.Fatalf("pane:join dropped source pane %s into wrong window; expected it in window 0 (target %s).\nwindow 0 panes: %v\nall session panes:\n%s",
			sourcePane, targetPane, panesInTargetWindow, strings.TrimSpace(string(all)))
	}
}

func paneIDFor(t *testing.T, socket, target string) string {
	t.Helper()
	out, err := exec.Command("tmux", "-S", socket, "display-message", "-t", target, "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("display-message %s: %v", target, err)
	}
	return strings.TrimSpace(string(out))
}

func paneIDsIn(t *testing.T, socket, window string) []string {
	t.Helper()
	out, err := exec.Command("tmux", "-S", socket, "list-panes", "-t", window, "-F", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("list-panes %s: %v", window, err)
	}
	return strings.Fields(strings.TrimSpace(string(out)))
}

