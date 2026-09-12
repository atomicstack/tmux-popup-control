package menu

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestCommandRefreshesCachedOptions(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, _ := testutil.StartTmuxServer(t)
	defer cleanup()
	defer tmux.Shutdown()
	const option = "@review-cache-invalidation"
	defer tmuxCmd(socket, "set-option", "-gu", option).Run()
	first := RunCommand(socket, "set-option -g "+option+" before")().(ActionResult)
	if first.Err != nil {
		t.Fatal(first.Err)
	}
	if got := tmux.ShowOption(socket, option); got != "before" {
		t.Fatalf("initial value %q", got)
	}
	second := RunCommand(socket, "set-option -g "+option+" after")().(ActionResult)
	if second.Err != nil {
		t.Fatal(second.Err)
	}
	if got := tmux.ShowOption(socket, option); got != "after" {
		t.Fatalf("command left stale cached option %q", got)
	}
}
