package ui

import (
	"context"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

func TestCommandCompletionIntegration(t *testing.T) {
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	targetSession := "completion-target"
	if err := testutil.TmuxCommand(socket, "new-session", "-d", "-s", targetSession, "sleep", "600").Run(); err != nil {
		t.Fatalf("create target session: %v", err)
	}

	bin := testutil.BuildBinary(t)
	pane, exitFile := testutil.LaunchBinary(t, bin, socket, "ui-completion", "command")

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()

	testutil.WaitForContent(t, ctx, socket, pane, "command")
	testutil.SendText(t, socket, pane, "kill-session -t comp")
	testutil.WaitForContent(t, ctx, socket, pane, targetSession)
	testutil.SendKeys(t, socket, pane, "Tab")
	testutil.WaitForContent(t, ctx, socket, pane, "kill-session -t "+targetSession)

	_ = exitFile
	_ = testutil.TmuxCommand(socket, "kill-session", "-t", "ui-completion").Run()
	_ = testutil.TmuxCommand(socket, "kill-session", "-t", targetSession).Run()
}
