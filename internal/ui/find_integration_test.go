package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

// Drives pane:find-pane-by-content against a real tmux server: a pane whose
// scrollback holds one marker and whose viewport holds another. Typing finds
// the on-screen marker; the scrolled-off marker is only found once alt+a
// ticks "all history".
//
// The binary runs in an ordinary pane here (not the launcher's popup pane,
// which the pane list leaves out), so its own pane also matches whatever is
// typed into its prompt. The assertions therefore key on the target pane.
func TestFindPaneByContentIntegration(t *testing.T) {
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	const script = `echo zebra-in-history; i=0; while [ $i -lt 80 ]; do echo filler $i; i=$((i+1)); done; echo walrus-on-screen; sleep 300`
	if err := testutil.TmuxCommand(socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", "find-target", "sh", "-c", script).Run(); err != nil {
		t.Fatalf("start target session: %v", err)
	}
	t.Cleanup(func() { _ = testutil.TmuxCommand(socket, "kill-session", "-t", "find-target").Run() })
	testutil.WaitForContent(t, ctx, socket, "find-target:0.0", "walrus-on-screen")

	bin := testutil.BuildBinary(t)
	pane, _ := testutil.LaunchBinary(t, bin, socket, "ui-find", "pane:find-pane-by-content")
	t.Cleanup(func() { _ = testutil.TmuxCommand(socket, "kill-session", "-t", "ui-find").Run() })
	testutil.WaitForContent(t, ctx, socket, pane, "type to search pane contents")

	// The on-screen marker is found by a viewport search.
	testutil.SendText(t, socket, pane, "walrus-on")
	// The list column is narrow at 80 columns, so the row's snippet is cut
	// short; the preview shows the rest of the matching line.
	out := testutil.WaitForContent(t, ctx, socket, pane, "find-target:0.0")
	if !strings.Contains(out, "-screen") {
		t.Fatalf("preview should show the matching line of find-target:0.0:\n%s", out)
	}

	// The scrolled-off marker is not on screen, so a viewport search must
	// not list the target pane for it.
	testutil.SendKeys(t, socket, pane, "C-u")
	testutil.SendText(t, socket, pane, "zebra-in")
	out = testutil.WaitForContent(t, ctx, socket, pane, "1 pane matches")
	if strings.Contains(out, "-history") || strings.Contains(out, "find-target:0.0") {
		t.Fatalf("viewport search found text that is only in scrollback:\n%s", out)
	}

	// alt+a ticks "all history" and the scrollback match appears.
	testutil.SendKeys(t, socket, pane, "M-a")
	out = testutil.WaitForContent(t, ctx, socket, pane, "find-target:0.0")
	if !strings.Contains(out, "■") || !strings.Contains(out, "2 panes match") {
		t.Fatalf("history search should tick the box and list find-target:0.0:\n%s", out)
	}
}

// The window variant lists the window that holds a matching pane.
func TestFindWindowByContentIntegration(t *testing.T) {
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	if err := testutil.TmuxCommand(socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", "findw-target", "-n", "aquarium", "sh", "-c", "echo narwhal-on-screen; sleep 300").Run(); err != nil {
		t.Fatalf("start target session: %v", err)
	}
	t.Cleanup(func() { _ = testutil.TmuxCommand(socket, "kill-session", "-t", "findw-target").Run() })
	testutil.WaitForContent(t, ctx, socket, "findw-target:0.0", "narwhal-on-screen")

	bin := testutil.BuildBinary(t)
	pane, _ := testutil.LaunchBinary(t, bin, socket, "ui-findw", "window:find-window-by-content")
	t.Cleanup(func() { _ = testutil.TmuxCommand(socket, "kill-session", "-t", "ui-findw").Run() })
	testutil.WaitForContent(t, ctx, socket, pane, "type to search pane contents")

	testutil.SendText(t, socket, pane, "narwhal-on")
	out := testutil.WaitForContent(t, ctx, socket, pane, "findw-target:0")
	for _, want := range []string{"aquarium", "-screen"} {
		if !strings.Contains(out, want) {
			t.Fatalf("window result row missing %q:\n%s", want, out)
		}
	}
}
