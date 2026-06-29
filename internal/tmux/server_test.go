package tmux

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestServerStartTimeUsesExec verifies ServerStartTime resolves #{start_time}
// via a one-shot `tmux display-message` and never opens a control-mode client.
// The only caller (autosave-status) runs ~once/sec; a control-mode attach
// there forces a full server state-sync and previously leaked the tmux -C
// subprocess. newTmux is stubbed to fail so any control-mode use would surface.
func TestServerStartTimeUsesExec(t *testing.T) {
	withStubTmux(t, func(string) (tmuxClient, error) {
		return nil, errors.New("control-mode must not be used")
	})
	var gotArgs []string
	withStubCommander(t, func(name string, args ...string) commander {
		gotArgs = append([]string{name}, args...)
		return stubCommander{output: []byte("1700000000\n")}
	})

	got, err := ServerStartTime("/tmp/socket")
	if err != nil {
		t.Fatalf("ServerStartTime returned error: %v", err)
	}
	if want := time.Unix(1700000000, 0); !got.Equal(want) {
		t.Errorf("ServerStartTime = %v, want %v", got, want)
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "-S /tmp/socket") || !strings.Contains(joined, "display-message") || !strings.Contains(joined, "#{start_time}") {
		t.Errorf("unexpected exec invocation: %q", joined)
	}
}

// TestConfigureControlClientSuppressesOutput verifies that a freshly
// established control-mode client is told to suppress %output notifications via
// `refresh-client -f no-output`. This app never consumes pane-output events
// (previews use request/response capture-pane; the backend polls list-*), so
// receiving them is pure waste — and during a restore the buffered output for
// our client can stall tmux's draining of pane PTYs, blocking content replay.
func TestConfigureControlClientSuppressesOutput(t *testing.T) {
	fake := &fakeClient{}
	configureControlClient(fake)
	if len(fake.controlFlags) != 1 || fake.controlFlags[0] != "no-output" {
		t.Fatalf("expected SetControlFlags(\"no-output\"), got %v", fake.controlFlags)
	}
}

// TestConfigureControlClientIgnoresError ensures flag setup is best-effort: an
// older tmux that rejects the flag must not fail the connection.
func TestConfigureControlClientIgnoresError(t *testing.T) {
	fake := &fakeClient{controlFlagsErr: errors.New("unknown flag")}
	configureControlClient(fake) // must not panic or propagate
}

func TestServerStartTimeExecError(t *testing.T) {
	withStubCommander(t, func(string, ...string) commander {
		return stubCommander{err: errors.New("boom")}
	})
	if _, err := ServerStartTime(""); err == nil {
		t.Fatal("expected error when exec fails")
	}
}

func TestServerStartTimeEmptyOutput(t *testing.T) {
	withStubCommander(t, func(string, ...string) commander {
		return stubCommander{output: []byte("  \n")}
	})
	if _, err := ServerStartTime(""); err == nil {
		t.Fatal("expected error for empty start_time")
	}
}
