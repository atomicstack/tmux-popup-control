package testutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
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
func StartTmuxServer(t *testing.T) (string, func(), string) {
	t.Helper()
	RequireTmux(t)
	baseDir, err := os.MkdirTemp("/tmp", "tmux-popup-control-*")
	if err != nil {
		t.Fatalf("failed to create tmux temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(baseDir) })
	socketPath := filepath.Join(baseDir, "tmux-test.sock")
	cmd := tmuxCommand(socketPath, "-f", "/dev/null", "-vv", "new-session", "-d", "-s", "tmux-popup-control-test", "sleep", "600")
	if err := cmd.Run(); err != nil {
		t.Skipf("skipping: failed to start tmux server: %v", err)
	}
	serverPID := ""
	if out, err := tmuxCommand(socketPath, "display-message", "-p", "#{pid}").Output(); err == nil {
		serverPID = strings.TrimSpace(string(out))
		if serverPID != "" {
			t.Logf("started tmux test server pid=%s socket=%s", serverPID, socketPath)
		}
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := killTmuxServerControl(ctx, socketPath); err != nil {
			t.Logf("control-mode kill failed for socket %s: %v; falling back to tmux kill-server", socketPath, err)
			_ = tmuxCommand(socketPath, "kill-server").Run()
		}
	}
	return socketPath, cleanup, baseDir
}

// AssertNoServerCrash scans tmux server logs under logDir to ensure the server
// did not terminate unexpectedly. It should be called after exercising the
// temporary tmux instance started by StartTmuxServer.
func AssertNoServerCrash(t *testing.T, logDir string) {
	t.Helper()
	if strings.TrimSpace(logDir) == "" {
		return
	}
	files, err := filepath.Glob(filepath.Join(logDir, "tmux-server-*.log"))
	if err != nil {
		t.Fatalf("failed to glob tmux logs: %v", err)
	}
	if len(files) == 0 {
		t.Logf("no tmux server logs located under %s", logDir)
		return
	}
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read tmux server log %s: %v", path, err)
		}
		if bytes.Contains(content, []byte("server exited unexpectedly")) {
			t.Fatalf("tmux server reported unexpected exit; see %s", path)
		}
	}
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
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "TMUX=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "TMUX=")
	if trimmed != "" {
		env = append(env, "TMUX_TMPDIR="+filepath.Dir(trimmed))
	}
	cmd.Env = env
	return cmd
}

func killTmuxServerControl(ctx context.Context, socket string) error {
	if strings.TrimSpace(socket) == "" {
		return errors.New("empty tmux socket path")
	}
	client, err := gotmux.NewTmuxWithOptions(socket, gotmux.WithContext(ctx))
	if err != nil {
		return err
	}
	defer client.Close()
	return client.KillServer()
}

// TODO: launch the tmux-popup-control binary inside the temporary server and
// verify the UI by comparing CapturePane output against golden files.
