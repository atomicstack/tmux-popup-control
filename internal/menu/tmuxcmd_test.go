package menu

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTmuxCommandEnvRestoresHostPaneInPopup(t *testing.T) {
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "%42")

	env := tmuxCommandEnv("/tmp/tmux-popup-control/test.sock")

	if !containsEnv(env, "TMUX_PANE=%42") {
		t.Fatalf("expected TMUX_PANE fallback in env, got %v", env)
	}
	if !containsEnv(env, "TMUX_TMPDIR="+filepath.Dir("/tmp/tmux-popup-control/test.sock")) {
		t.Fatalf("expected TMUX_TMPDIR in env, got %v", env)
	}
}

func TestTmuxCommandEnvPreservesExistingPane(t *testing.T) {
	t.Setenv("TMUX_PANE", "%7")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "%42")

	env := tmuxCommandEnv("/tmp/tmux-popup-control/test.sock")

	if containsEnv(env, "TMUX_PANE=%42") {
		t.Fatalf("expected existing TMUX_PANE to win, got %v", env)
	}
}

func containsEnv(env []string, prefix string) bool {
	for _, entry := range env {
		if strings.TrimSpace(entry) == prefix {
			return true
		}
	}
	return false
}
