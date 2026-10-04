package menu

import (
	"os"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

func TestMain(m *testing.M) {
	// Cut the tests off from the tmux server they were launched from; see
	// testutil.IsolateFromUserServer.
	isolationCleanup := testutil.IsolateFromUserServer()
	code := m.Run()
	testutil.ShutdownSharedServer()
	isolationCleanup()
	os.Exit(code)
}
