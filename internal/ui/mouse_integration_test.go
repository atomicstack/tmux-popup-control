package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

// sgrClick is the bytes a terminal sends for a left press and release at the
// 0-indexed cell x, y (SGR mouse reporting is 1-indexed).
func sgrClick(x, y int) string {
	return fmt.Sprintf("\x1b[<0;%d;%dM\x1b[<0;%d;%dm", x+1, y+1, x+1, y+1)
}

// Drives the real binary with terminal mouse reports: a double click on a
// root menu row must open that submenu, exactly as enter would.
func TestMouseDoubleClickOpensSubmenuIntegration(t *testing.T) {
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	bin := testutil.BuildBinary(t)
	pane, _ := testutil.LaunchBinary(t, bin, socket, "ui-mouse", "")
	t.Cleanup(func() { _ = testutil.TmuxCommand(socket, "kill-session", "-t", "ui-mouse").Run() })
	out := testutil.WaitForContent(t, ctx, socket, pane, "resurrect")

	row := -1
	for y, line := range strings.Split(ansi.Strip(out), "\n") {
		if strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "▌")) == "window" {
			row = y
		}
	}
	if row < 0 {
		t.Fatalf("no window row in the root menu:\n%s", out)
	}

	// A single click only moves the cursor: the root menu stays up.
	testutil.SendText(t, socket, pane, sgrClick(6, row))
	time.Sleep(600 * time.Millisecond) // longer than the double-click window
	if after, _ := testutil.CapturePane(t, socket, pane); !strings.Contains(after, "resurrect") {
		t.Fatalf("a single click left the root menu:\n%s", after)
	}

	// Two clicks in quick succession open the window menu.
	testutil.SendText(t, socket, pane, sgrClick(6, row)+sgrClick(6, row))
	testutil.WaitForContent(t, ctx, socket, pane, "find-window-by-content")
}
