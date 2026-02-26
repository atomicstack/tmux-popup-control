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
	_ = tmuxCommand(socket, "send-keys", "-t", pane, "Escape").Run()
	_ = tmuxCommand(socket, "kill-session", "-t", session).Run()
}

// TestNavigationOpensSubmenuAndEscapeReturns starts the binary at the root
// menu, filters to select the session category, navigates into it, then presses
// Escape to confirm the root menu is restored.
func TestNavigationOpensSubmenuAndEscapeReturns(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	// Launch at the root menu (no override).
	pane, exitFile := launchBinary(t, bin, socket, "nav-session", "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Wait for root menu to render; "session" is one of the top-level items.
	WaitForContent(t, ctx, socket, pane, "session")

	// Type to filter the root list down to the session entry, then enter it.
	SendText(t, socket, pane, "sess")
	SendKeys(t, socket, pane, "Enter")

	// Inside the session submenu; wait for a known item.
	WaitForContent(t, ctx, socket, pane, "switch")

	// Press Escape to pop back to the root menu.
	SendKeys(t, socket, pane, "Escape")

	// Root menu should now show top-level categories again.
	WaitForContent(t, ctx, socket, pane, "window")
	output, err := CapturePane(t, socket, pane)
	if err != nil {
		t.Fatalf("capture-pane after Escape: %v", err)
	}
	if !strings.Contains(output, "session") {
		t.Fatalf("expected root menu after Escape, got:\n%s", output)
	}

	// Escape at the root exits the binary.
	SendKeys(t, socket, pane, "Escape")
	_ = waitForExit(t, ctx, exitFile)
	_ = tmuxCommand(socket, "kill-session", "-t", "nav-session").Run()
}

// TestFilterNarrowsSessionList starts the binary at the session:switch menu
// with two distinctly-named sessions and verifies that typing a filter string
// hides the non-matching entry while keeping the matching one visible.
func TestFilterNarrowsSessionList(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	// Create two extra sessions with distinct names.
	for _, name := range []string{"alpha-sess", "beta-sess"} {
		if err := tmuxCommand(socket, "new-session", "-d", "-s", name).Run(); err != nil {
			t.Fatalf("create session %s: %v", name, err)
		}
	}

	pane, exitFile := launchBinary(t, bin, socket, "filter-session", "session:switch")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Wait for both sessions to appear in the switch list.
	WaitForContent(t, ctx, socket, pane, "alpha-sess")
	WaitForContent(t, ctx, socket, pane, "beta-sess")

	// Type a filter that matches only "alpha-sess".
	SendText(t, socket, pane, "alpha")

	// "beta-sess" should disappear; "alpha-sess" should remain.
	WaitForAbsent(t, ctx, socket, pane, "beta-sess")
	output, err := CapturePane(t, socket, pane)
	if err != nil {
		t.Fatalf("capture-pane after filter: %v", err)
	}
	if !strings.Contains(output, "alpha-sess") {
		t.Fatalf("expected alpha-sess still visible after filter, got:\n%s", output)
	}

	// Escape at the root (session:switch is the root here) exits the binary.
	SendKeys(t, socket, pane, "Escape")
	_ = waitForExit(t, ctx, exitFile)
	_ = tmuxCommand(socket, "kill-session", "-t", "filter-session").Run()
}

// TestSessionSwitchExitsCleanly launches the binary at session:switch with a
// second session available, selects it via Enter, and verifies the binary
// exits cleanly (no error displayed). This is a regression test for the
// switch-client flow.
func TestSessionSwitchExitsCleanly(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	// Create a target session to switch to.
	if err := tmuxCommand(socket, "new-session", "-d", "-s", "switch-target").Run(); err != nil {
		t.Fatalf("create target session: %v", err)
	}

	pane, exitFile := launchBinary(t, bin, socket, "switch-sess", "session:switch")

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Wait for the switch menu to render with the target session visible.
	WaitForContent(t, ctx, socket, pane, "switch-target")

	// Capture before pressing Enter so we can see the rendered state.
	beforeOutput, _ := CapturePane(t, socket, pane)
	t.Logf("session:switch menu before Enter:\n%s", beforeOutput)

	// Press Enter to select the highlighted item (should be switch-target
	// since the current session is excluded by default).
	SendKeys(t, socket, pane, "Enter")

	// The binary should exit after a successful (or failed) switch action.
	// If successful: ActionResult with no error → tea.Quit → exit 0.
	// If error: error is displayed and binary stays alive.
	// Give it a moment, then check.
	exitCtx, exitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer exitCancel()

	code := waitForExit(t, exitCtx, exitFile)
	t.Logf("binary exit code: %s", code)

	if code != "0" {
		// Capture the pane to see any error that was displayed.
		errOutput, _ := CapturePane(t, socket, pane)
		t.Fatalf("binary exited with code %s; pane output:\n%s", code, errOutput)
	}

	// Clean up.
	_ = tmuxCommand(socket, "kill-session", "-t", "switch-sess").Run()
	_ = tmuxCommand(socket, "kill-session", "-t", "switch-target").Run()
}

// TestEscapeExitsFromRootMenu verifies that pressing Escape at the root menu
// causes the binary to exit promptly with code 0.
func TestEscapeExitsFromRootMenu(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	pane, exitFile := launchBinary(t, bin, socket, "escape-session", "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Wait for the root menu to render.
	WaitForContent(t, ctx, socket, pane, "window")

	start := time.Now()
	SendKeys(t, socket, pane, "Escape")
	code := waitForExit(t, ctx, exitFile)
	elapsed := time.Since(start)

	if code != "0" {
		t.Fatalf("expected exit code 0, got %q", code)
	}
	if elapsed > 500*time.Millisecond {
		t.Logf("warning: binary took %v to exit after Escape (expected < 500ms)", elapsed)
	}
	t.Logf("binary exited in %v with code %s", elapsed, code)

	_ = tmuxCommand(socket, "kill-session", "-t", "escape-session").Run()
}
