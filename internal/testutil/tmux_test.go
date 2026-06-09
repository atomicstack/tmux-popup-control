package testutil

import "testing"

func TestStartTmuxServerLifecycle(t *testing.T) {
	// Use an isolated server here: killing the server is the whole point of the
	// test, and doing that to the package-shared server (whose sync.Once is
	// already consumed) would hand every subsequent test in the binary a dead
	// socket. The bug only stayed hidden because this file sorted last.
	socket, cleanup, logDir := StartIsolatedTmuxServer(t)
	defer cleanup()
	if err := TmuxCommand(socket, "list-sessions").Run(); err != nil {
		t.Skipf("skipping: list-sessions failed: %v", err)
	}
	_ = TmuxCommand(socket, "kill-server").Run()
	AssertNoServerCrash(t, logDir)
}
