package ui

import (
	"os"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestMain(m *testing.M) {
	// Cut the tests off from the tmux server they were launched from; see
	// testutil.IsolateFromUserServer.
	isolationCleanup := testutil.IsolateFromUserServer()
	// Model fixtures carry placeholder socket paths and drive capture/preview
	// commands through menu loaders that are not stubbable from here. Refuse
	// every in-process control-mode dial so they can never exec a real tmux;
	// the live-server tests in this package run the binary out of process.
	restoreDial := tmux.ForbidDialForTests()
	// Theme colour lookups go through a live control-mode connection; unit
	// tests must never reach a real tmux server for them. Tests that care
	// about the lookup swap in their own stub.
	resolveThemeColourFn = func(string, string, string) (string, bool) { return "", false }
	code := m.Run()
	testutil.ShutdownSharedServer()
	restoreDial()
	isolationCleanup()
	os.Exit(code)
}
