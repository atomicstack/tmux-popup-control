package testutil

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// installPlugin sources the repo's main.tmux on socket from a temp dir that
// holds the test binary under the name main.tmux expects, as a plugin
// checkout would.
func installPlugin(t *testing.T, socket string) {
	t.Helper()
	bin := BuildBinary(t)
	dir := t.TempDir()
	if err := os.Symlink(bin, filepath.Join(dir, "tmux-popup-control")); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(repoRoot(t), "main.tmux"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.tmux"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := TmuxCommand(socket, "run-shell", filepath.Join(dir, "main.tmux")).CombinedOutput(); err != nil {
		t.Fatalf("source main.tmux: %v\n%s", err, out)
	}
}

func showGlobal(t *testing.T, socket, option string) string {
	t.Helper()
	out, _ := TmuxCommand(socket, "show-options", "-gqv", option).Output()
	return strings.TrimSpace(string(out))
}

// TestKeyBindingLauncherIntegration runs a binding's run-shell -C launcher
// exactly as main.tmux binds it, for a session whose name needs escaping
// both for tmux's command parser and for the shell, and checks the popup
// opens as a modal floating pane with the name intact.
func TestKeyBindingLauncherIntegration(t *testing.T) {
	socket, cleanup, logDir := StartIsolatedTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })
	installPlugin(t, socket)

	const session = `odd #na'me "$x`
	if err := TmuxCommand(socket, "new-session", "-d", "-s", session, "-x", "160", "-y", "48", "sleep 300").Run(); err != nil {
		t.Fatalf("create session: %v", err)
	}
	hostOut, err := TmuxCommand(socket, "display-message", "-p", "-t", "="+session+":", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("host pane id: %v", err)
	}
	host := strings.TrimSpace(string(hostOut))

	keys, err := TmuxCommand(socket, "list-keys", "-T", "prefix", "$").Output()
	if err != nil {
		t.Fatalf("list-keys: %v", err)
	}
	if !strings.Contains(string(keys), "run-shell -C") || strings.Contains(string(keys), "main.sh") {
		t.Fatalf("rename binding does not use the run-shell -C launcher: %s", keys)
	}

	launch := "#{E:@tmux-popup-control-launch} --root-menu session:rename --menu-args #{q/a:session_name}"
	if out, err := TmuxCommand(socket, "run-shell", "-t", host, "-C", launch).CombinedOutput(); err != nil {
		t.Fatalf("launch: %v\n%s", err, out)
	}

	var popup string
	deadline := time.Now().Add(5 * time.Second)
	for popup == "" && time.Now().Before(deadline) {
		out, _ := TmuxCommand(socket, "list-panes", "-t", host, "-F", "#{pane_id} #{pane_floating_flag} #{pane_modal_flag} #{pane_title}").Output()
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if fields := strings.Fields(line); len(fields) == 4 && fields[1] == "1" && fields[2] == "1" && fields[3] == "tmux-popup-control" {
				popup = fields[0]
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if popup == "" {
		t.Fatal("no modal floating popup pane appeared")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	output := WaitForContent(t, ctx, socket, popup, session)
	t.Logf("popup:\n%s", output)
}

// TestAutosaveMonitorHookIntegration checks that tmux itself schedules
// autosaves: once the interval option makes the monitor's format true, the
// hook runs autosave-now, which writes an autosave, advances the schedule and
// leaves the status indicator cleared.
func TestAutosaveMonitorHookIntegration(t *testing.T) {
	socket, cleanup, logDir := StartIsolatedTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { AssertNoServerCrash(t, logDir) })
	installPlugin(t, socket)

	storage := t.TempDir()
	set := func(option, value string) {
		t.Helper()
		if err := TmuxCommand(socket, "set-option", "-g", option, value).Run(); err != nil {
			t.Fatalf("set %s: %v", option, err)
		}
	}
	set("@tmux-popup-control-session-storage-dir", storage)
	// a monitor takes its first value on its first one-second tick and never
	// fires for it, so let it see "not due" before autosave is enabled.
	time.Sleep(1500 * time.Millisecond)
	// pretend the last save was a minute before the server started, so one
	// minute's interval is already due the moment it is enabled.
	set("@tmux-popup-control-autosave-last", "-60")
	set("@tmux-popup-control-autosave-interval-minutes", "1")

	var saves []string
	deadline := time.Now().Add(15 * time.Second)
	for len(saves) == 0 && time.Now().Before(deadline) {
		saves, _ = filepath.Glob(filepath.Join(storage, "auto-*.json"))
		time.Sleep(100 * time.Millisecond)
	}
	if len(saves) == 0 {
		t.Fatal("the monitor hook never produced an autosave")
	}

	last, err := strconv.Atoi(showGlobal(t, socket, "@tmux-popup-control-autosave-last"))
	if err != nil || last < 0 {
		t.Fatalf("autosave-last was not advanced to the server uptime: %q", showGlobal(t, socket, "@tmux-popup-control-autosave-last"))
	}
	deadline = time.Now().Add(5 * time.Second)
	for showGlobal(t, socket, "@tmux-popup-control-autosave-saving") != "" && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if got := showGlobal(t, socket, "@tmux-popup-control-autosave-saving"); got != "" {
		t.Fatalf("status indicator left set: %q", got)
	}
}
