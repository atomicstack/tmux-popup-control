package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
)

func TestCommandCompletionIntegration(t *testing.T) {
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	targetSession := "completion-target"
	if err := exec.Command("tmux", "-S", socket, "new-session", "-d", "-s", targetSession, "sleep", "600").Run(); err != nil {
		t.Fatalf("create target session: %v", err)
	}

	bin := buildCompletionBinary(t)
	pane, exitFile := launchCompletionBinary(t, bin, socket, "ui-completion", "command")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	testutil.WaitForContent(t, ctx, socket, pane, "command")
	testutil.SendText(t, socket, pane, "kill-session -t comp")
	testutil.WaitForContent(t, ctx, socket, pane, targetSession)
	testutil.SendKeys(t, socket, pane, "Tab")
	testutil.WaitForContent(t, ctx, socket, pane, "kill-session -t "+targetSession)

	_ = exitFile
	_ = exec.Command("tmux", "-S", socket, "kill-session", "-t", "ui-completion").Run()
	_ = exec.Command("tmux", "-S", socket, "kill-session", "-t", targetSession).Run()
}

func buildCompletionBinary(t *testing.T) string {
	t.Helper()
	root := completionRepoRoot(t)
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "tmux-popup-control")
	cmd := exec.Command("go", "build", "-o", bin, "./")
	cmd.Dir = root
	cmd.Env = append(os.Environ(),
		"GOCACHE="+filepath.Join(root, ".gocache"),
		"GOMODCACHE="+filepath.Join(root, ".gomodcache"),
		"GOFLAGS=-modcacherw",
		"GOPROXY=off",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build binary: %s: %v", output, err)
	}
	return bin
}

func launchCompletionBinary(t *testing.T, bin, socket, session, rootMenu string) (pane, exitFile string) {
	t.Helper()
	scriptDir := t.TempDir()
	exitFile = filepath.Join(scriptDir, "exit-code")
	scriptPath := filepath.Join(scriptDir, "run.sh")
	script := "#!/bin/sh\n" +
		"POPUP_BIN=" + shellQuote(bin) + "\n" +
		"POPUP_SOCKET=" + shellQuote(socket) + "\n" +
		"POPUP_EXIT=" + shellQuote(exitFile) + "\n" +
		fmt.Sprintf("export TMUX_POPUP_CONTROL_ROOT_MENU=%s\n", shellQuote(rootMenu)) +
		"export TMUX_POPUP_CONTROL_COLOR_PROFILE=ansi256\n" +
		"\"$POPUP_BIN\" -socket \"$POPUP_SOCKET\" -width 80 -height 24 2>/dev/null\n" +
		"printf '%s' $? > \"$POPUP_EXIT\"\n" +
		"sleep 300\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write launcher script: %v", err)
	}
	cmd := exec.Command("tmux", "-S", socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", session, scriptPath)
	if err := cmd.Run(); err != nil {
		t.Fatalf("launch binary session %s: %v", session, err)
	}
	return session + ":0.0", exitFile
}

func completionRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
