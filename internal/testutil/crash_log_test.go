package testutil

import (
	"path/filepath"
	"testing"
)

// TestServerLogsLandInLogDir proves that the tmux server's -vv verbose logs
// are written into the directory that AssertNoServerCrash inspects. Before the
// fix the server inherited the test binary's cwd (the package source dir) so
// the glob in AssertNoServerCrash always came up empty and the assertion was a
// silent no-op.
func TestServerLogsLandInLogDir(t *testing.T) {
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()

	// Touch the server so it has run and flushed something to its log.
	if err := TmuxCommand(socket, "list-sessions").Run(); err != nil {
		t.Skipf("skipping: list-sessions failed: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(logDir, "tmux-server-*.log"))
	if err != nil {
		t.Fatalf("glob tmux server logs: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("expected tmux server logs under %s, found none; AssertNoServerCrash would be a no-op", logDir)
	}
}
