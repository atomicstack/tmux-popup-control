package menu

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

// TestCustomizeModeTargetsHostPaneIntegration pins that customize-mode opens
// in the pane the popup was launched from. The popup is a modal floating pane,
// and tmux resolves an untargeted command to the modal pane even when
// $TMUX_PANE names another one, so customize-mode opened inside the popup and
// vanished when it closed. It must target the host pane explicitly.
func TestCustomizeModeTargetsHostPaneIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, _ := testutil.StartIsolatedTmuxServer(t)
	defer cleanup()

	host := tmuxOut(t, socket, "display-message", "-p", "-t", "tmux-popup-control-test:", "#{pane_id}")
	popup := tmuxOut(t, socket, "new-pane", "-d", "-P", "-F", "#{pane_id}", "-t", host,
		"-O", "-x", "50%", "-y", "50%", "-X", "25%", "-Y", "25%")
	t.Setenv("TMUX_PANE", popup)

	msg := CustomizeModeAction(Context{SocketPath: socket, CurrentPaneID: host}, Item{ID: "customize-mode"})()
	if res, ok := msg.(ActionResult); !ok || res.Err != nil {
		t.Fatalf("customize-mode: %#v", msg)
	}

	if mode := tmuxOut(t, socket, "display-message", "-p", "-t", host, "#{pane_mode}"); mode != "options-mode" {
		t.Fatalf("host pane mode = %q, want options-mode", mode)
	}
	if mode := tmuxOut(t, socket, "display-message", "-p", "-t", popup, "#{pane_mode}"); mode != "" {
		t.Fatalf("popup pane mode = %q, want none", mode)
	}
}
