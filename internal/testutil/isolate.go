package testutil

import (
	"fmt"
	"os"
	"strings"
)

// IsolateFromUserServer cuts a test binary off from the tmux server it was
// launched from. Call it first in TestMain, and run the returned cleanup after
// m.Run.
//
// Code under test treats an empty socket path as "the current server", which
// tmux resolves from $TMUX — or, with $TMUX unset, from the default socket
// under $TMUX_TMPDIR, which is the user's own server too. A test that reached
// a real `tmux` exec without a socket therefore acted on the user's live
// sessions: TestKillSessionsAcceptsSessionID killed whichever session had id
// $3. Clearing the tmux and popup variables and pointing TMUX_TMPDIR at an
// empty directory makes any such call fail with "no server running" instead.
// Tests that need a server get one with an explicit socket from
// StartTmuxServer / StartIsolatedTmuxServer.
func IsolateFromUserServer() func() {
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if name == "TMUX" || name == "TMUX_PANE" || strings.HasPrefix(name, "TMUX_POPUP_") {
			_ = os.Unsetenv(name)
		}
	}
	dir, err := os.MkdirTemp("/tmp", "tmux-popup-control-noserver-*")
	if err != nil {
		panic(fmt.Sprintf("testutil: creating isolated TMUX_TMPDIR: %v", err))
	}
	_ = os.Setenv("TMUX_TMPDIR", dir)
	return func() { _ = os.RemoveAll(dir) }
}
