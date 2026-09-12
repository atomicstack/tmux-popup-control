package ui

import (
	"os"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

func TestMain(m *testing.M) {
	// Theme colour lookups go through a live control-mode connection; unit
	// tests must never reach a real tmux server for them. Tests that care
	// about the lookup swap in their own stub.
	resolveThemeColourFn = func(string, string, string) (string, bool) { return "", false }
	code := m.Run()
	testutil.ShutdownSharedServer()
	os.Exit(code)
}
