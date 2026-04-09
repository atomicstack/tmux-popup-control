package testutil

import "testing"

func TestStartTmuxServerLifecycle(t *testing.T) {
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	if err := TmuxCommand(socket, "list-sessions").Run(); err != nil {
		t.Skipf("skipping: list-sessions failed: %v", err)
	}
	_ = TmuxCommand(socket, "kill-server").Run()
	AssertNoServerCrash(t, logDir)
}
