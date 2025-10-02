package testutil

import (
	"context"
	"errors"
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
	tdir := t.TempDir()
	bin := filepath.Join(tdir, "tmux-popup-control")
	cmd := exec.Command("go", "build", "-o", bin, "./")
	cmd.Env = append(os.Environ(), "GOCACHE="+filepath.Join(tdir, ".gocache"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}
	return bin
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
