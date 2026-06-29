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
