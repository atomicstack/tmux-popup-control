package tmux

import (
	"errors"
	"slices"
	"testing"
)

// TestConfigureControlClientSuppressesOutput verifies that a freshly
// established control-mode client is told to suppress %output notifications via
// `refresh-client -f no-output`. This app never consumes pane-output events
// (previews use request/response capture-pane; the backend polls list-*), so
// receiving them is pure waste — and during a restore the buffered output for
// our client can stall tmux's draining of pane PTYs, blocking content replay.
func TestConfigureControlClientSuppressesOutput(t *testing.T) {
	fake := &fakeClient{}
	configureControlClient(fake)
	if len(fake.controlFlags) != 2 || fake.controlFlags[0] != "no-output" || fake.controlFlags[1] != "new-layouts" {
		t.Fatalf("expected SetControlFlags(\"no-output\") then SetControlFlags(\"new-layouts\"), got %v", fake.controlFlags)
	}
}

// TestConfigureControlClientRequestsNewLayouts verifies that the client opts
// into the JSON (v2) layout format. tmux next-3.9 keeps sending the old v1
// format to control clients unless they set the new-layouts flag; v1 strings
// omit floating panes, which is what breaks resurrect for windows with a
// floater. The flag is sent in its own refresh-client call so an older tmux
// rejecting it cannot undo no-output.
func TestConfigureControlClientRequestsNewLayouts(t *testing.T) {
	fake := &fakeClient{}
	configureControlClient(fake)
	if !slices.Contains(fake.controlFlags, "new-layouts") {
		t.Fatalf("expected a SetControlFlags(\"new-layouts\") call, got %v", fake.controlFlags)
	}
}

// TestConfigureControlClientIgnoresError ensures flag setup is best-effort: an
// older tmux that rejects the flag must not fail the connection.
func TestConfigureControlClientIgnoresError(t *testing.T) {
	fake := &fakeClient{controlFlagsErr: errors.New("unknown flag")}
	configureControlClient(fake) // must not panic or propagate
}
