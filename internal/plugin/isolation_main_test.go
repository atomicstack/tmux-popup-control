package plugin

import (
	"os"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

// TestMain cuts the tests off from the tmux server they were launched from;
// see testutil.IsolateFromUserServer.
func TestMain(m *testing.M) {
	cleanup := testutil.IsolateFromUserServer()
	code := m.Run()
	cleanup()
	os.Exit(code)
}
