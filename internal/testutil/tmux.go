package testutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

var ErrPaneUnavailable = errors.New("tmux pane unavailable")

// Per-package shared tmux server. Each test binary (one per Go test
// package) starts at most one tmux server for its lifetime via
// sync.Once. StartTmuxServer hands out the same socket to every test
// in the package; cleanup kills only the sessions a given test added.
//
// Why: spawning a fresh tmux server per test (the prior model) caused
// 11–13 concurrent tmux server processes at peak under `make test`
// because async cleanup left old servers dying in the background while
// the next test spawned its own. The fork+exec churn dominated test
// wall-clock and produced enough CPU contention to time out the
// timing-sensitive integration tests.
var (
	sharedServerOnce   sync.Once
	sharedServerMu     sync.Mutex
	sharedServerSocket string
	sharedServerLogDir string
	sharedServerErr    error
)

// keepaliveSession is the long-lived session created by the shared server
// at startup. It exists solely to keep the tmux server process alive
// across tests. Tests must not delete it; cleanup excludes it from the
// kill set.
const keepaliveSession = "tmux-popup-control-test"

func startSharedServer() {
	baseDir, err := os.MkdirTemp("/tmp", "tmux-popup-control-pool-*")
	if err != nil {
		sharedServerErr = fmt.Errorf("mktemp: %w", err)
		return
	}
	socketPath := filepath.Join(baseDir, "tmux-test.sock")
	// sleep is long enough to outlive any reasonable `make test` run; if
	// ShutdownSharedServer is called it gets killed sooner.
	cmd := TmuxCommand(socketPath, "-f", "/dev/null", "-vv", "new-session", "-d", "-s", keepaliveSession, "sleep", "3600")
	if err := cmd.Run(); err != nil {
		sharedServerErr = fmt.Errorf("tmux new-session: %w", err)
		_ = os.RemoveAll(baseDir)
		return
	}
	sharedServerSocket = socketPath
	sharedServerLogDir = baseDir
}

// ShutdownSharedServer kills the package-shared tmux server and removes
// its log directory. Call from TestMain when you want to clean up at
// process exit; otherwise the server's keepalive `sleep 3600` will hold
// it open for an hour.
func ShutdownSharedServer() {
	sharedServerMu.Lock()
	socket := sharedServerSocket
	dir := sharedServerLogDir
	sharedServerMu.Unlock()
	if socket == "" {
		return
	}
	_ = TmuxCommand(socket, "kill-server").Run()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

// listSessionNames returns the current set of session names on socket,
// or nil if the lookup fails.
func listSessionNames(socket string) []string {
	out, err := TmuxCommand(socket, "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil
	}
	var names []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// RequireTmux aborts the calling test when tmux is not present on PATH.
func RequireTmux(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("skipping: tmux binary not available")
	}
	return path
}

// StartIsolatedTmuxServer spawns a fresh, dedicated tmux server with a
// unique socket. Use this only for tests that explicitly need an isolated
// server (e.g. cross-server save/restore round-trip tests); other tests
// should use the cheaper pooled StartTmuxServer.
func StartIsolatedTmuxServer(t *testing.T) (string, func(), string) {
	t.Helper()
	RequireTmux(t)
	baseDir, err := os.MkdirTemp("/tmp", "tmux-popup-control-iso-*")
	if err != nil {
		t.Fatalf("failed to create tmux temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(baseDir) })
	socketPath := filepath.Join(baseDir, "tmux-test.sock")
	cmd := TmuxCommand(socketPath, "-f", "/dev/null", "-vv", "new-session", "-d", "-s", keepaliveSession, "sleep", "600")
	if err := cmd.Run(); err != nil {
		t.Skipf("skipping: failed to start tmux server: %v", err)
	}
	if out, err := TmuxCommand(socketPath, "display-message", "-p", "#{pid}").Output(); err == nil {
		if pid := strings.TrimSpace(string(out)); pid != "" {
			t.Logf("started isolated tmux test server pid=%s socket=%s", pid, socketPath)
		}
	}
	cleanup := func() {
		_ = TmuxCommand(socketPath, "kill-server").Run()
	}
	return socketPath, cleanup, baseDir
}

// StartTmuxServer returns a tmux socket and per-test cleanup func. The
// socket points at a long-lived tmux server shared across every test in
// the calling test binary (one per Go package). Cleanup tears down only
// the sessions created during this test — the server itself stays up so
// the next test reuses it without paying tmux's process spawn cost.
func StartTmuxServer(t *testing.T) (string, func(), string) {
	t.Helper()
	RequireTmux(t)
	sharedServerOnce.Do(startSharedServer)
	if sharedServerErr != nil {
		t.Skipf("shared tmux server unavailable: %v", sharedServerErr)
	}

	socket := sharedServerSocket
	logDir := sharedServerLogDir

	// Snapshot existing sessions so cleanup leaves them alone (specifically
	// the keepalive plus anything earlier tests deliberately left behind).
	preexisting := listSessionNames(socket)
	t.Logf("started tmux test server (shared) socket=%s pre-existing=%v", socket, preexisting)

	cleanup := func() {
		now := listSessionNames(socket)
		for _, name := range now {
			if name == keepaliveSession || slices.Contains(preexisting, name) {
				continue
			}
			_ = TmuxCommand(socket, "kill-session", "-t", name).Run()
		}
	}
	return socket, cleanup, logDir
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
	cmd := TmuxCommand(socketPath, args...)
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

// TmuxCommand builds a tmux command targeting the given socket with all
// tmux-related environment variables sanitised to prevent contamination
// of the user's live session when tests run inside an existing tmux.
func TmuxCommand(socket string, extra ...string) *exec.Cmd {
	trimmed := strings.TrimSpace(socket)
	args := make([]string, 0, len(extra)+2)
	if trimmed != "" {
		args = append(args, "-S", trimmed)
	}
	args = append(args, extra...)
	cmd := exec.Command("tmux", args...)
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "TMUX=") ||
			strings.HasPrefix(entry, "TMUX_PANE=") ||
			strings.HasPrefix(entry, "TMUX_POPUP_CONTROL_") {
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


// WaitForContent polls the given pane until its captured output contains
// substr, returning the full output when found. The test fails if ctx expires.
func WaitForContent(t *testing.T, ctx context.Context, socketPath, pane, substr string) string {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			out, _ := CapturePane(t, socketPath, pane)
			t.Fatalf("timeout waiting for %q in pane %s; last output:\n%s", substr, pane, out)
			return ""
		case <-time.After(50 * time.Millisecond):
			out, err := CapturePane(t, socketPath, pane)
			if err != nil {
				if errors.Is(err, ErrPaneUnavailable) {
					continue
				}
				t.Fatalf("capture-pane error waiting for %q: %v", substr, err)
			}
			if strings.Contains(out, substr) {
				return out
			}
		}
	}
}

// WaitForAbsent polls until the pane output no longer contains substr.
// The test fails if ctx expires.
func WaitForAbsent(t *testing.T, ctx context.Context, socketPath, pane, substr string) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			out, _ := CapturePane(t, socketPath, pane)
			t.Fatalf("timeout waiting for %q to disappear from pane %s; last output:\n%s", substr, pane, out)
		case <-time.After(50 * time.Millisecond):
			out, err := CapturePane(t, socketPath, pane)
			if err != nil {
				if errors.Is(err, ErrPaneUnavailable) {
					continue
				}
				t.Fatalf("capture-pane error waiting for absence of %q: %v", substr, err)
			}
			if !strings.Contains(out, substr) {
				return
			}
		}
	}
}

// SendKeys sends one or more named keys to a tmux pane (e.g. "Down", "Enter",
// "Escape", "q"). Each call to tmux send-keys sends all provided keys in one
// shot; use SendText for literal character input.
func SendKeys(t *testing.T, socketPath, pane string, keys ...string) {
	t.Helper()
	if len(keys) == 0 {
		return
	}
	args := append([]string{"send-keys", "-t", pane}, keys...)
	if err := TmuxCommand(socketPath, args...).Run(); err != nil {
		t.Fatalf("send-keys %v to pane %s: %v", keys, pane, err)
	}
}

// SendText sends a literal string to a tmux pane without key-name lookup.
// Use this for filter input (e.g. "alpha").
func SendText(t *testing.T, socketPath, pane, text string) {
	t.Helper()
	if err := TmuxCommand(socketPath, "send-keys", "-l", "-t", pane, text).Run(); err != nil {
		t.Fatalf("send-text %q to pane %s: %v", text, pane, err)
	}
}
