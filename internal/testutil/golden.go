package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	RequireTmux(t)
	root := repoRoot(t)
	tdir := t.TempDir()
	bin := filepath.Join(tdir, "tmux-popup-control")
	cmd := exec.Command("go", "build", "-o", bin, "./")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(tdir, ".gocache"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}
	return bin
}

// launchBinary starts the tmux-popup-control binary in a new detached session
// on the given socket and returns the pane target ("session:0.0") and the path
// to the file where the script will write the exit code once the binary exits.
// rootMenu may be empty (root menu) or any valid menu path (e.g. "session:switch").
func launchBinary(t *testing.T, bin, socket, session, rootMenu string) (pane, exitFile string) {
	t.Helper()
	scriptDir := t.TempDir()
	exitFile = filepath.Join(scriptDir, "exit-code")
	scriptPath := filepath.Join(scriptDir, "run.sh")
	var rootLine string
	if rootMenu != "" {
		rootLine = fmt.Sprintf("export TMUX_POPUP_CONTROL_ROOT_MENU=%s\n", shellQuote(rootMenu))
	}
	// Embed paths directly so no env-var propagation through tmux is needed.
	// Do NOT redirect stdout here: the binary writes via os.Stdout which is the
	// pane's PTY. Redirecting it to /dev/null silences all rendering.
	script := "#!/bin/sh\n" +
		"POPUP_BIN=" + shellQuote(bin) + "\n" +
		"POPUP_SOCKET=" + shellQuote(socket) + "\n" +
		"POPUP_EXIT=" + shellQuote(exitFile) + "\n" +
		rootLine +
		"\"$POPUP_BIN\" -socket \"$POPUP_SOCKET\" -width 80 -height 24 2>/dev/null\n" +
		"printf '%s' $? > \"$POPUP_EXIT\"\n" +
		"sleep 300\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write launcher script: %v", err)
	}
	cmd := tmuxCommand(socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", session, scriptPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("start session %s: %v", session, err)
	}
	return session + ":0.0", exitFile
}

// waitForExit polls exitFile until it contains a non-empty string and returns
// the exit-code string. The test fails if ctx expires first.
func waitForExit(t *testing.T, ctx context.Context, exitFile string) string {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for binary to exit (exit file: %s)", exitFile)
			return ""
		case <-time.After(50 * time.Millisecond):
			data, err := os.ReadFile(exitFile)
			if err != nil {
				continue
			}
			if code := strings.TrimSpace(string(data)); code != "" {
				return code
			}
		}
	}
}

func waitForRender(t *testing.T, ctx context.Context, socket, target, exitPath string) {
	t.Helper()
	loggedPaneMissing := false
	loggedEmpty := false
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for render: %v", ctx.Err())
		case <-time.After(50 * time.Millisecond):
			if exitPath != "" {
				if data, err := os.ReadFile(exitPath); err == nil {
					code := strings.TrimSpace(string(data))
					if code != "" && code != "0" {
						t.Fatalf("tmux-popup-control exited early with code %s", code)
					}
				}
			}
			out, err := CapturePane(t, socket, target)
			if err != nil {
				if errors.Is(err, ErrPaneUnavailable) {
					if !loggedPaneMissing {
						t.Logf("waiting for pane %s to become available", target)
						loggedPaneMissing = true
					}
					continue
				}
				t.Fatalf("capture-pane error: %v", err)
			}
			if out != "" {
				return
			}
			if !loggedEmpty {
				t.Logf("pane %s captured but empty, retrying", target)
				loggedEmpty = true
			}
		}
	}
}

func assertGolden(t *testing.T, goldenName, output string) {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", goldenName)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
			t.Fatalf("failed to update golden: %v", err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden %s: %v", goldenName, err)
	}
	if string(data) != output {
		t.Fatalf("output mismatch for %s\nexpected:\n%s\nactual:\n%s", goldenName, string(data), output)
	}
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
// Safe for paths produced by os.MkdirTemp and standard temp-dir locations.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}
