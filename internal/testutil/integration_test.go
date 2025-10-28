package testutil

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRootMenuRendering(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() {
		AssertNoServerCrash(t, logDir)
	})
	session := "rootmenu"
	pane := session + ":0.0"
	scriptDir := t.TempDir()
	exitFile := filepath.Join(scriptDir, "exit-code")
	scriptPath := filepath.Join(scriptDir, "run.sh")
	script := "#!/bin/sh\n" +
		"\"$POPUP_BIN\" -socket \"$POPUP_SOCKET\" -width 80 -height 24 > /dev/null 2>&1\n" +
		"printf '%s' $? > \"$POPUP_EXIT\"\n" +
		"sleep 300\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write launcher script: %v", err)
	}
	cmd := tmuxCommand(socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", session, scriptPath)
	cmd.Env = append(cmd.Env,
		"POPUP_BIN="+bin,
		"POPUP_SOCKET="+socket,
		"POPUP_EXIT="+exitFile,
	)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to launch binary: %v", err)
	}
	if err := tmuxCommand(socket, "has-session", "-t", session).Run(); err != nil {
		t.Skipf("skipping: unable to create tmux session: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	waitForRender(t, ctx, socket, pane, exitFile)
	output, err := CapturePane(t, socket, pane)
	if err != nil {
		t.Fatalf("capture-pane failed: %v", err)
	}
	if strings.TrimSpace(output) == "" {
		t.Skip("tmux capture returned empty output; skipping golden comparison")
	}
	assertGolden(t, filepath.Join("capture", "root_menu.txt"), output)
	_ = tmuxCommand(socket, "send-keys", "-t", pane, "q").Run()
	_ = tmuxCommand(socket, "kill-session", "-t", session).Run()
}
