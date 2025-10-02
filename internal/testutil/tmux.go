package testutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var ErrPaneUnavailable = errors.New("tmux pane unavailable")

// RequireTmux aborts the calling test when tmux is not present on PATH.
func RequireTmux(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("skipping: tmux binary not available")
	}
	return path
}

// StartTmuxServer boots a temporary tmux server bound to a unique socket.
// The returned cleanup function terminates the server and removes any
// temporary files that were created during setup.
func StartTmuxServer(t *testing.T) (string, func()) {
	t.Helper()
	RequireTmux(t)
	baseDir, err := os.MkdirTemp("/tmp", "tmux-popup-control-*")
	if err != nil {
		t.Fatalf("failed to create tmux temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(baseDir) })
	socketPath := filepath.Join(baseDir, "tmux-test.sock")
	cmd := tmuxCommand(socketPath, "-f", "/dev/null", "new-session", "-d", "-s", "tmux-popup-control-test", "sleep", "600")
	if err := cmd.Run(); err != nil {
		t.Skipf("skipping: failed to start tmux server: %v", err)
	}
	cleanup := func() {
		_ = tmuxCommand(socketPath, "kill-server").Run()
	}
	return socketPath, cleanup
}

// CapturePane returns the rendered contents of a tmux pane.
func CapturePane(t *testing.T, socketPath, target string) (string, error) {
	t.Helper()
	RequireTmux(t)
	args := []string{"capture-pane", "-e", "-p"}
	if target != "" {
		args = append(args, "-t", target)
	}
	cmd := tmuxCommand(socketPath, args...)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", ErrPaneUnavailable
		}
		return "", fmt.Errorf("capture-pane failed: %w", err)
	}
	return string(output), nil
}

func tmuxCommand(socket string, extra ...string) *exec.Cmd {
	trimmed := strings.TrimSpace(socket)
	args := make([]string, 0, len(extra)+2)
	if trimmed != "" {
		args = append(args, "-S", trimmed)
	}
	args = append(args, extra...)
	cmd := exec.Command("tmux", args...)
	if trimmed != "" {
		cmd.Env = append(os.Environ(), "TMUX_TMPDIR="+filepath.Dir(trimmed))
	}
	return cmd
}

// TODO: launch the tmux-popup-control binary inside the temporary server and
// verify the UI by comparing CapturePane output against golden files.
