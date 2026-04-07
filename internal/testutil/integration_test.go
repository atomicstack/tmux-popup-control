package testutil

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
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
	pane, _ := launchBinary(t, bin, socket, session, "")
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	output := WaitForContent(t, ctx, socket, pane, "process")
	assertGolden(t, filepath.Join("capture", "root_menu.txt"), output)
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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
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
	exitCtx, exitCancel := context.WithTimeout(t.Context(), 5*time.Second)
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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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

// TestCommandMenuMoveWindowRenumber verifies the command submenu by typing
// the full "move-window -r -t <target>" command in the filter bar and pressing
// Enter to execute directly.
func TestCommandMenuMoveWindowRenumber(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	target := "renumber-target"

	// Create a target session with three windows, then kill the middle one to
	// produce a gap: indices 0, 2 (window 1 removed).
	if err := tmuxCommand(socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", target).Run(); err != nil {
		t.Fatalf("create target session: %v", err)
	}
	if err := tmuxCommand(socket, "new-window", "-t", target).Run(); err != nil {
		t.Fatalf("new-window 1: %v", err)
	}
	if err := tmuxCommand(socket, "new-window", "-t", target).Run(); err != nil {
		t.Fatalf("new-window 2: %v", err)
	}
	// Kill window index 1 to create a gap (leaves indices 0, 2).
	if err := tmuxCommand(socket, "kill-window", "-t", target+":1").Run(); err != nil {
		t.Fatalf("kill-window 1: %v", err)
	}

	// Verify gap exists.
	beforeIndices := windowIndices(t, socket, target)
	t.Logf("window indices before: %v", beforeIndices)
	if len(beforeIndices) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(beforeIndices))
	}
	hasGap := false
	for i := 1; i < len(beforeIndices); i++ {
		if beforeIndices[i]-beforeIndices[i-1] > 1 {
			hasGap = true
			break
		}
	}
	if !hasGap {
		t.Fatalf("expected gap in window indices %v", beforeIndices)
	}

	// Launch the binary in a separate session at the "command" root menu.
	pane, exitFile := launchBinary(t, bin, socket, "cmd-runner", "command")

	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()

	// Wait for the command list to render (any command visible means it loaded).
	WaitForContent(t, ctx, socket, pane, "command")

	// Type the full command in the filter bar and execute with Enter.
	// Use Tab to accept the autocomplete for "move-window", then type args.
	SendText(t, socket, pane, "move-win")
	WaitForContent(t, ctx, socket, pane, "move-window")
	SendKeys(t, socket, pane, "Tab")

	// Append " -r -t <target>" to the filter.
	// Use -- to prevent tmux send-keys from interpreting "-r" as a flag.
	if err := tmuxCommand(socket, "send-keys", "-l", "-t", pane, "--", " -r -t "+target).Run(); err != nil {
		t.Fatalf("send-text to pane %s: %v", pane, err)
	}

	// Press Enter to execute the command directly from the filter.
	SendKeys(t, socket, pane, "Enter")

	// The binary should exit after execution.
	exitCtx, exitCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer exitCancel()
	code := waitForExit(t, exitCtx, exitFile)
	t.Logf("binary exit code: %s", code)
	if code != "0" {
		errOutput, _ := CapturePane(t, socket, pane)
		t.Fatalf("binary exited with code %s; pane output:\n%s", code, errOutput)
	}

	// Verify windows are renumbered: should be 0, 1 (no gap).
	afterIndices := windowIndices(t, socket, target)
	t.Logf("window indices after: %v", afterIndices)
	if len(afterIndices) != 2 {
		t.Fatalf("expected 2 windows after renumber, got %d", len(afterIndices))
	}
	for i, idx := range afterIndices {
		if idx != i {
			t.Fatalf("expected contiguous indices starting at 0, got %v", afterIndices)
		}
	}

	_ = tmuxCommand(socket, "kill-session", "-t", "cmd-runner").Run()
	_ = tmuxCommand(socket, "kill-session", "-t", target).Run()
}

// TestTreeFilterShowsOnlyMatchingItems launches the binary at session:tree
// with multiple sessions and windows, types a filter that matches a session
// name, and verifies that only the matching session is shown — its windows
// (which don't independently match) must NOT appear.
func TestTreeFilterShowsOnlyMatchingItems(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	// Create sessions with distinct names and windows whose names do NOT
	// contain the session name, so we can verify per-item filtering.
	for _, name := range []string{"shells", "devbox"} {
		if err := tmuxCommand(socket, "new-session", "-d", "-s", name).Run(); err != nil {
			t.Fatalf("create session %s: %v", name, err)
		}
	}
	// Rename windows to names that definitely don't contain "shells".
	if err := tmuxCommand(socket, "rename-window", "-t", "shells:0", "vim").Run(); err != nil {
		t.Fatalf("rename window: %v", err)
	}
	if err := tmuxCommand(socket, "new-window", "-t", "shells", "-n", "htop").Run(); err != nil {
		t.Fatalf("new-window htop: %v", err)
	}
	if err := tmuxCommand(socket, "rename-window", "-t", "devbox:0", "code").Run(); err != nil {
		t.Fatalf("rename window: %v", err)
	}

	// Launch at session:tree with expanded so windows are visible initially.
	// Disable the preview panel — the preview would show the selected
	// session's windows, causing false matches on window names.
	pane, exitFile := launchBinaryWithEnv(t, bin, socket, "tree-filter", "session:tree",
		[]string{
			"export TMUX_POPUP_CONTROL_MENU_ARGS=expanded",
			"export TMUX_POPUP_CONTROL_NO_PREVIEW=1",
		})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Wait for the tree to render — both sessions should be visible.
	WaitForContent(t, ctx, socket, pane, "shells")
	WaitForContent(t, ctx, socket, pane, "devbox")

	// Verify windows are visible in the expanded tree (pre-filter).
	output := WaitForContent(t, ctx, socket, pane, "vim")
	t.Logf("tree before filter:\n%s", output)
	if !strings.Contains(output, "htop") {
		t.Fatalf("expected htop window visible before filtering, got:\n%s", output)
	}

	// Type "shells" to filter.
	SendText(t, socket, pane, "shells")

	// Wait for non-matching items to disappear.
	WaitForAbsent(t, ctx, socket, pane, "devbox")

	// Capture the final filtered state.
	output, err := CapturePane(t, socket, pane)
	if err != nil {
		t.Fatalf("capture-pane after filter: %v", err)
	}
	t.Logf("tree after filtering 'shells':\n%s", output)

	// The "shells" session must be visible.
	if !strings.Contains(output, "shells") {
		t.Fatalf("expected 'shells' session visible after filter, got:\n%s", output)
	}

	// Windows "vim" and "htop" do NOT contain "shells" in their own metadata,
	// so they must NOT appear.
	if strings.Contains(output, "vim") {
		t.Fatalf("window 'vim' should not be visible when filtering 'shells' (it doesn't match):\n%s", output)
	}
	if strings.Contains(output, "htop") {
		t.Fatalf("window 'htop' should not be visible when filtering 'shells' (it doesn't match):\n%s", output)
	}

	// Clean up: Escape exits the binary.
	SendKeys(t, socket, pane, "Escape")
	exitCtx, exitCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer exitCancel()
	_ = waitForExit(t, exitCtx, exitFile)
	_ = tmuxCommand(socket, "kill-session", "-t", "tree-filter").Run()
	_ = tmuxCommand(socket, "kill-session", "-t", "shells").Run()
	_ = tmuxCommand(socket, "kill-session", "-t", "devbox").Run()
}

// TestTreeFilterChildMatchShowsAncestor verifies that when a window name
// matches the filter, its parent session is shown as an ancestor even if
// the session name itself doesn't match.
func TestTreeFilterChildMatchShowsAncestor(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	// Create a session with a uniquely-named window.
	if err := tmuxCommand(socket, "new-session", "-d", "-s", "mywork").Run(); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := tmuxCommand(socket, "rename-window", "-t", "mywork:0", "xyzzyfind").Run(); err != nil {
		t.Fatalf("rename window: %v", err)
	}
	// A second window that should NOT match.
	if err := tmuxCommand(socket, "new-window", "-t", "mywork", "-n", "bash").Run(); err != nil {
		t.Fatalf("new-window: %v", err)
	}

	pane, exitFile := launchBinaryWithEnv(t, bin, socket, "tree-child", "session:tree",
		[]string{"export TMUX_POPUP_CONTROL_MENU_ARGS=expanded"})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Wait for tree to render with both windows visible.
	WaitForContent(t, ctx, socket, pane, "xyzzyfind")
	WaitForContent(t, ctx, socket, pane, "bash")

	// Filter for "xyzzy" — should match window "xyzzyfind" only.
	SendText(t, socket, pane, "xyzzy")

	// "bash" window should disappear.
	WaitForAbsent(t, ctx, socket, pane, "bash")

	output, err := CapturePane(t, socket, pane)
	if err != nil {
		t.Fatalf("capture-pane after filter: %v", err)
	}
	t.Logf("tree after filtering 'xyzzy':\n%s", output)

	// Parent session "mywork" must still be visible as an ancestor.
	if !strings.Contains(output, "mywork") {
		t.Fatalf("expected ancestor session 'mywork' visible, got:\n%s", output)
	}
	// The matching window must be visible.
	if !strings.Contains(output, "xyzzyfind") {
		t.Fatalf("expected matching window 'xyzzyfind' visible, got:\n%s", output)
	}

	SendKeys(t, socket, pane, "Escape")
	exitCtx, exitCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer exitCancel()
	_ = waitForExit(t, exitCtx, exitFile)
	_ = tmuxCommand(socket, "kill-session", "-t", "tree-child").Run()
	_ = tmuxCommand(socket, "kill-session", "-t", "mywork").Run()
}

// TestPaneCaptureResolvesCorrectPaneID launches the binary with two sessions
// and verifies that the pane:capture form's preview shows the pane ID of the
// active pane in the host session, not a pane from another session.
//
// Reproduces a bug where #{pane_id} in the capture template resolves to %1
// (from the first session) instead of the actual active pane (e.g. %4).
func TestPaneCaptureResolvesCorrectPaneID(t *testing.T) {
	bin := buildBinary(t)
	socket, cleanup, logDir := StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })

	// StartTmuxServer creates "tmux-popup-control-test" as the initial session.
	// Create a second session and split it to get higher pane IDs.
	targetSession := "zzz-target"
	if err := tmuxCommand(socket, "new-session", "-d", "-x", "80", "-y", "24", "-s", targetSession).Run(); err != nil {
		t.Fatalf("create target session: %v", err)
	}
	for i := range 3 {
		if err := tmuxCommand(socket, "split-window", "-t", targetSession).Run(); err != nil {
			t.Fatalf("split-window %d: %v", i, err)
		}
	}
	time.Sleep(200 * time.Millisecond)

	// Get the active pane ID in the target session.
	activeOut, err := tmuxCommand(socket, "display-message", "-t", targetSession, "-p", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("get active pane: %v", err)
	}
	targetPaneID := strings.TrimSpace(string(activeOut))
	t.Logf("active pane in %s: %s", targetSession, targetPaneID)

	// Get the session ID for the target session.
	sidOut, err := tmuxCommand(socket, "display-message", "-t", targetSession, "-p", "#{session_id}").Output()
	if err != nil {
		t.Fatalf("get session id: %v", err)
	}
	sessionID := strings.TrimSpace(string(sidOut))
	t.Logf("target session ID: %s", sessionID)

	// Launch the binary directly into the pane:capture form.
	// Pass env vars simulating the display-popup environment.
	pane, exitFile := launchBinaryWithEnv(t, bin, socket, "capture-test", "pane:capture",
		[]string{
			"export TMUX_POPUP_CONTROL_SESSION=" + targetSession,
			"export TMUX_POPUP_CONTROL_SESSION_ID=" + strings.TrimPrefix(sessionID, "$"),
			"export TMUX_POPUP_CONTROL_PANE_ID=" + targetPaneID,
		})

	// Ensure sessions are cleaned up even if the test fails mid-way.
	t.Cleanup(func() {
		_ = tmuxCommand(socket, "kill-session", "-t", "capture-test").Run()
		_ = tmuxCommand(socket, "kill-session", "-t", targetSession).Run()
	})

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	// Wait for the capture form's async preview to resolve with the
	// correct pane ID. Waiting for the exact expected string avoids a
	// race where we capture the pane output between "tmux-" appearing
	// and the full pane ID being rendered.
	expectedPreview := "tmux-" + targetPaneID + "."
	output := WaitForContent(t, ctx, socket, pane, expectedPreview)
	t.Logf("capture form output:\n%s", output)

	// Clean up: Escape quits directly since pane:capture was invoked
	// via root menu override.
	SendKeys(t, socket, pane, "Escape")
	exitCtx, exitCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer exitCancel()
	_ = waitForExit(t, exitCtx, exitFile)
}

// windowIndices returns the sorted window indices for the given session.
func windowIndices(t *testing.T, socket, session string) []int {
	t.Helper()
	out, err := tmuxCommand(socket, "list-windows", "-t", session, "-F", "#{window_index}").Output()
	if err != nil {
		t.Fatalf("list-windows: %v", err)
	}
	var indices []int
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("parse window index %q: %v", line, err)
		}
		indices = append(indices, idx)
	}
	sort.Ints(indices)
	return indices
}
