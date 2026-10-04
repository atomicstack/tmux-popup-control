package testutil

import (
	"os"
	"testing"
)

// TestMain ensures the package's shared tmux server is killed at process
// exit. Without this the keepalive `sleep 3600` would hold the server
// process open for an hour after the test binary returns.
func TestMain(m *testing.M) {
	isolationCleanup := IsolateFromUserServer()
	code := m.Run()
	ShutdownSharedServer()
	isolationCleanup()
	os.Exit(code)
}
