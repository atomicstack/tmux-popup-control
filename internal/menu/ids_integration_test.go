package menu

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/testutil"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// weirdSessionFixture describes a live session whose name contains ':' so
// name-based tmux targets ("a:b:1", "a:b:0.1") are unparseable by tmux.
type weirdSessionFixture struct {
	socket    string
	name      string
	sessionID string // $N
	windowIDs []string
	paneIDs   [][]string // [windowIdx][paneIdx] -> %N
}

// setupWeirdSession builds a session with two windows (window 0 has two
// panes), then renames it to "a:b". Skips when the running tmux still
// rejects ':' in session names (pre next-3.8).
func setupWeirdSession(t *testing.T, socket string) weirdSessionFixture {
	t.Helper()
	const tmp = "ids-tmp"
	tmuxRun(t, socket, "new-session", "-d", "-s", tmp, "-n", "first", "-x", "80", "-y", "24")
	tmuxRun(t, socket, "new-window", "-t", tmp, "-n", "second")
	tmuxRun(t, socket, "split-window", "-d", "-t", tmp+":0")
	tmuxRun(t, socket, "select-window", "-t", tmp+":0")
	tmuxRun(t, socket, "select-pane", "-t", tmp+":0.0")

	// The name deliberately sorts *before* the pooled keepalive session.
	// gotmuxcc's control bridge attaches to the first session listed by
	// list-sessions, which tmux orders by name, so this fixture is the one
	// it picks. Since gotmuxcc v0.4.0 discoverAttachTarget asks for
	// #{session_id} rather than #{session_name}, a ':' in the name no
	// longer makes that target unparseable and the connection comes up.
	fx := weirdSessionFixture{socket: socket, name: "aa:weird"}
	fx.sessionID = tmuxOut(t, socket, "display-message", "-t", tmp, "-p", "#{session_id}")
	for _, line := range strings.Split(tmuxOut(t, socket, "list-panes", "-s", "-t", tmp,
		"-F", "#{window_index}\t#{window_id}\t#{pane_index}\t#{pane_id}"), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			t.Fatalf("unexpected list-panes line %q", line)
		}
		for len(fx.windowIDs) <= atoi(t, parts[0]) {
			fx.windowIDs = append(fx.windowIDs, "")
			fx.paneIDs = append(fx.paneIDs, nil)
		}
		widx := atoi(t, parts[0])
		fx.windowIDs[widx] = parts[1]
		fx.paneIDs[widx] = append(fx.paneIDs[widx], parts[3])
	}
	if len(fx.windowIDs) != 2 || len(fx.paneIDs[0]) != 2 {
		t.Fatalf("unexpected fixture topology: windows=%v panes=%v", fx.windowIDs, fx.paneIDs)
	}

	if err := exec.Command("tmux", "-S", socket, "rename-session", "-t", tmp, fx.name).Run(); err != nil {
		t.Skipf("skipping: this tmux rejects ':' in session names (%v)", err)
	}
	// Prove the premise: tmux itself cannot address the session by name.
	if err := exec.Command("tmux", "-S", socket, "has-session", "-t", fx.name).Run(); err == nil {
		t.Logf("note: has-session resolved %q by name on this tmux", fx.name)
	}
	return fx
}

func tmuxRun(t *testing.T, socket string, args ...string) {
	t.Helper()
	if out, err := exec.Command("tmux", append([]string{"-S", socket}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("tmux %s: %v (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
}

func tmuxOut(t *testing.T, socket string, args ...string) string {
	t.Helper()
	out, err := exec.Command("tmux", append([]string{"-S", socket}, args...)...).Output()
	if err != nil {
		t.Fatalf("tmux %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			t.Fatalf("expected integer, got %q", s)
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// controlClientNames lists the names of control-mode clients on the server.
func controlClientNames(t *testing.T, socket string) []string {
	t.Helper()
	out := tmuxOut(t, socket, "list-clients", "-F", "#{client_control_mode}\t#{client_name}")
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "1\t") {
			names = append(names, strings.TrimPrefix(line, "1\t"))
		}
	}
	return names
}

func clientSessionID(t *testing.T, socket, client string) string {
	t.Helper()
	return tmuxOut(t, socket, "display-message", "-c", client, "-p", "#{session_id}")
}

// liveMenuContext builds a menu.Context from real snapshots, exactly as the
// backend poller would.
func liveMenuContext(t *testing.T, socket string) Context {
	t.Helper()
	sessions, err := tmux.FetchSessions(socket)
	if err != nil {
		t.Fatalf("FetchSessions: %v", err)
	}
	windows, err := tmux.FetchWindows(socket)
	if err != nil {
		t.Fatalf("FetchWindows: %v", err)
	}
	panes, err := tmux.FetchPanes(socket)
	if err != nil {
		t.Fatalf("FetchPanes: %v", err)
	}
	return Context{
		SocketPath: socket,
		Sessions:   SessionEntriesFromTmux(sessions.Sessions),
		Windows:    WindowEntriesFromTmux(windows.Windows),
		Panes:      PaneEntriesFromTmux(panes.Panes),
	}
}

func runActionResult(t *testing.T, what string, cmd func() any) ActionResult {
	t.Helper()
	msg := cmd()
	res, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("%s: expected ActionResult, got %T", what, msg)
	}
	if res.Err != nil {
		t.Fatalf("%s: unexpected error: %v", what, res.Err)
	}
	return res
}

// TestActionsTargetSessionWithColonInNameIntegration drives the window
// switch, pane switch, session switch, and session-tree paths against a
// live tmux server whose session is named "aa:weird". Every tmux target the
// actions build must be an id ($N/@N/%N); a name-derived target such as
// "aa:weird:1" makes tmux look for session "aa" window "weird" and fail.
func TestActionsTargetSessionWithColonInNameIntegration(t *testing.T) {
	testutil.RequireTmux(t)
	socket, cleanup, logDir := testutil.StartTmuxServer(t)
	defer cleanup()
	t.Cleanup(func() { testutil.AssertNoServerCrash(t, logDir) })

	tmux.Shutdown()
	t.Cleanup(tmux.Shutdown)

	fx := setupWeirdSession(t, socket)
	// A second, plainly named session the control client can be parked on
	// so a session switch is observable.
	tmuxRun(t, socket, "new-session", "-d", "-s", "ids-home")
	homeID := tmuxOut(t, socket, "display-message", "-t", "ids-home", "-p", "#{session_id}")

	before := controlClientNames(t, socket)
	ctx := liveMenuContext(t, socket)
	after := controlClientNames(t, socket)
	var ours string
	for _, name := range after {
		if !slices.Contains(before, name) {
			ours = name
			break
		}
	}
	if ours == "" {
		t.Fatalf("could not identify the control-mode client (before=%v after=%v)", before, after)
	}
	park := func() {
		tmuxRun(t, socket, "switch-client", "-c", ours, "-t", homeID)
		time.Sleep(50 * time.Millisecond)
		if got := clientSessionID(t, socket, ours); got != homeID {
			t.Fatalf("failed to park control client on %s, got %s", homeID, got)
		}
	}
	sessionState := func() (window, pane string) {
		out := tmuxOut(t, socket, "display-message", "-t", fx.sessionID, "-p", "#{window_id}\t#{pane_id}")
		w, p, _ := strings.Cut(out, "\t")
		return w, p
	}

	var window0, window1 WindowEntry
	for _, w := range ctx.Windows {
		if w.Session != fx.name {
			continue
		}
		switch w.Index {
		case 0:
			window0 = w
		case 1:
			window1 = w
		}
	}
	if window0.ID == "" || window1.ID == "" {
		t.Fatalf("expected both windows of %q in snapshot, got %#v", fx.name, ctx.Windows)
	}
	var pane01 PaneEntry
	for _, p := range ctx.Panes {
		if p.Session == fx.name && p.WindowIdx == 0 && p.Index == 1 {
			pane01 = p
		}
	}
	if pane01.ID == "" {
		t.Fatalf("expected pane 0.1 of %q in snapshot, got %#v", fx.name, ctx.Panes)
	}

	// window:switch -> window 1 of a:b.
	park()
	runActionResult(t, "window:switch", func() any { return WindowSwitchAction(ctx, Item{ID: window1.ID, Label: window1.Label})() })
	if got := clientSessionID(t, socket, ours); got != fx.sessionID {
		t.Fatalf("window:switch: client on %s, want %s", got, fx.sessionID)
	}
	if w, _ := sessionState(); w != fx.windowIDs[1] {
		t.Fatalf("window:switch: current window %s, want %s", w, fx.windowIDs[1])
	}

	// pane:switch -> pane 0.1 of a:b (window 0 must become current too).
	park()
	runActionResult(t, "pane:switch", func() any { return PaneSwitchAction(ctx, Item{ID: pane01.ID, Label: pane01.Label})() })
	if got := clientSessionID(t, socket, ours); got != fx.sessionID {
		t.Fatalf("pane:switch: client on %s, want %s", got, fx.sessionID)
	}
	if w, p := sessionState(); w != fx.windowIDs[0] || p != fx.paneIDs[0][1] {
		t.Fatalf("pane:switch: state %s/%s, want %s/%s", w, p, fx.windowIDs[0], fx.paneIDs[0][1])
	}

	// session:switch by name item.
	park()
	runActionResult(t, "session:switch", func() any { return SessionSwitchAction(ctx, Item{ID: fx.name, Label: fx.name})() })
	if got := clientSessionID(t, socket, ours); got != fx.sessionID {
		t.Fatalf("session:switch: client on %s, want %s", got, fx.sessionID)
	}

	// session:tree — window, pane, and session nodes.
	items := NewTreeState(true).BuildTreeItems(TreeItemsInput{Sessions: ctx.Sessions, Windows: ctx.Windows, Panes: ctx.Panes})
	var treeWindow1, treePane01, treeSession Item
	for _, it := range items {
		switch TreeItemKind(it.ID) {
		case "window":
			if it.Label == TreeWindowLabel(window1) {
				treeWindow1 = it
			}
		case "pane":
			if it.Label == TreePaneLabel(pane01) {
				treePane01 = it
			}
		case "session":
			if it.Label == fx.name {
				treeSession = it
			}
		}
	}
	if treeWindow1.ID == "" || treePane01.ID == "" || treeSession.ID == "" {
		t.Fatalf("missing tree items: window=%q pane=%q session=%q (items=%v)", treeWindow1.ID, treePane01.ID, treeSession.ID, items)
	}

	park()
	tmuxRun(t, socket, "select-window", "-t", fx.windowIDs[0])
	runActionResult(t, "session:tree window", func() any { return SessionTreeAction(ctx, treeWindow1)() })
	if got := clientSessionID(t, socket, ours); got != fx.sessionID {
		t.Fatalf("tree window: client on %s, want %s", got, fx.sessionID)
	}
	if w, _ := sessionState(); w != fx.windowIDs[1] {
		t.Fatalf("tree window: current window %s, want %s", w, fx.windowIDs[1])
	}

	park()
	tmuxRun(t, socket, "select-pane", "-t", fx.paneIDs[0][0])
	runActionResult(t, "session:tree pane", func() any { return SessionTreeAction(ctx, treePane01)() })
	if got := clientSessionID(t, socket, ours); got != fx.sessionID {
		t.Fatalf("tree pane: client on %s, want %s", got, fx.sessionID)
	}
	if w, p := sessionState(); w != fx.windowIDs[0] || p != fx.paneIDs[0][1] {
		t.Fatalf("tree pane: state %s/%s, want %s/%s", w, p, fx.windowIDs[0], fx.paneIDs[0][1])
	}

	park()
	runActionResult(t, "session:tree session", func() any { return SessionTreeAction(ctx, treeSession)() })
	if got := clientSessionID(t, socket, ours); got != fx.sessionID {
		t.Fatalf("tree session: client on %s, want %s", got, fx.sessionID)
	}
}
