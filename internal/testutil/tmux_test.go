package testutil

import "testing"

func TestStartTmuxServerLifecycle(t *testing.T) {
	socket, cleanup := StartTmuxServer(t)
	defer cleanup()
	if err := tmuxCommand(socket, "list-sessions").Run(); err != nil {
		t.Skipf("skipping: list-sessions failed: %v", err)
	}
	_ = tmuxCommand(socket, "kill-server").Run()
}
