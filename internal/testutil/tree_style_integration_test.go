package testutil

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestTreeStyleEnvAndCycleKeyIntegration verifies that the configured tree
// style reaches the rendered tree, and that ctrl+t cycles to the next style.
func TestTreeStyleEnvAndCycleKeyIntegration(t *testing.T) {
	bin := BuildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	if err := TmuxCommand(socket, "new-session", "-d", "-s", "styled", "-c", "/").Run(); err != nil {
		t.Fatalf("create session styled: %v", err)
	}

	pane, exitFile := launchBinaryWithEnv(t, bin, socket, "tree-style", "session:tree",
		[]string{
			"export TMUX_POPUP_CONTROL_MENU_ARGS=expanded",
			"export TMUX_POPUP_CONTROL_NO_PREVIEW=1",
			"export TMUX_POPUP_CONTROL_TREE_STYLE=rounded",
		})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Rounded draws expanded sessions as a tee joined to their children.
	output := WaitForContent(t, ctx, socket, pane, "─┬ styled")
	if strings.Contains(output, "▼") {
		t.Fatalf("rounded style should not draw label arrows, got:\n%s", output)
	}

	// Rounded is last in the cycle, so ctrl+t wraps to classic.
	SendKeys(t, socket, pane, "C-t")
	WaitForContent(t, ctx, socket, pane, "tree style: classic")
	output = WaitForContent(t, ctx, socket, pane, "─ ▼ styled")
	t.Logf("tree after cycling:\n%s", output)

	SendKeys(t, socket, pane, "Escape")
	exitCtx, exitCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer exitCancel()
	_ = waitForExit(t, exitCtx, exitFile)
	_ = TmuxCommand(socket, "kill-session", "-t", "tree-style").Run()
	_ = TmuxCommand(socket, "kill-session", "-t", "styled").Run()
}
