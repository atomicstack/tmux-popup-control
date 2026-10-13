package ui

import (
	"context"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

// Bubble Tea v2 enables bracketed paste, so a terminal paste reaches the
// program as a single tea.PasteMsg rather than a run of key presses. The
// command prompt must accept it as typed text. tmux applies the brackets
// itself (paste-buffer -p) because the pane requested the mode, which is the
// same path a cmd-v in the user's terminal takes into a floating pane.
func TestCommandPromptAcceptsBracketedPasteIntegration(t *testing.T) {
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	bin := testutil.BuildBinary(t)
	pane, _ := testutil.LaunchBinary(t, bin, socket, "ui-paste", "command")

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	testutil.WaitForContent(t, ctx, socket, pane, "command")

	const pasted = "display-message pasted-text"
	if err := testutil.TmuxCommand(socket, "set-buffer", "-b", "paste-test", pasted).Run(); err != nil {
		t.Fatalf("set-buffer: %v", err)
	}
	if err := testutil.TmuxCommand(socket, "paste-buffer", "-p", "-b", "paste-test", "-t", pane).Run(); err != nil {
		t.Fatalf("paste-buffer: %v", err)
	}
	testutil.WaitForContent(t, ctx, socket, pane, pasted)

	_ = testutil.TmuxCommand(socket, "kill-session", "-t", "ui-paste").Run()
}
