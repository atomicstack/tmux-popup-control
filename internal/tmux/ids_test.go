package tmux

import (
	"context"
	"strings"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// These tests pin the invariant that every tmux target this package builds
// for a session/window/pane is a tmux id ($N/@N/%N), never a name-derived
// string. tmux next-3.8 permits ':' and '.' in session and window names,
// which makes "session:index.pane" targets unparseable.

func TestSwitchPaneTargetsIDsOnly(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	ref := PaneRef{SessionID: "$3", WindowID: "@7", PaneID: "%9"}
	if err := SwitchPane("sock", "/dev/ttys009", ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastSwitchOpts == nil ||
		fake.lastSwitchOpts.TargetSession != "$3" ||
		fake.lastSwitchOpts.TargetClient != "/dev/ttys009" {
		t.Fatalf("unexpected switch opts %#v", fake.lastSwitchOpts)
	}
	if len(fake.selectWindowCalls) != 1 || fake.selectWindowCalls[0] != "@7" {
		t.Fatalf("unexpected select window calls %#v", fake.selectWindowCalls)
	}
	if len(fake.selectPaneCalls) != 1 || fake.selectPaneCalls[0] != "%9" {
		t.Fatalf("unexpected select pane calls %#v", fake.selectPaneCalls)
	}
}

func TestSwitchPaneFallsBackToPaneIDWhenSessionUnknown(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SwitchPane("sock", "", PaneRef{PaneID: "%9"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// tmux resolves a %N switch-client target to the pane's session, so the
	// pane id is a valid stand-in when no session id is known.
	if fake.lastSwitchOpts == nil || fake.lastSwitchOpts.TargetSession != "%9" {
		t.Fatalf("unexpected switch opts %#v", fake.lastSwitchOpts)
	}
	if len(fake.selectWindowCalls) != 0 {
		t.Fatalf("expected no select-window without a window id, got %#v", fake.selectWindowCalls)
	}
	if len(fake.selectPaneCalls) != 1 || fake.selectPaneCalls[0] != "%9" {
		t.Fatalf("unexpected select pane calls %#v", fake.selectPaneCalls)
	}
}

func TestSwitchPaneRequiresPaneID(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SwitchPane("sock", "", PaneRef{SessionID: "$3"}); err == nil || !strings.Contains(err.Error(), "pane id required") {
		t.Fatalf("expected validation error, got %v", err)
	}
	if fake.switchCalls != 0 {
		t.Fatalf("expected no switch call, got %d", fake.switchCalls)
	}
}

func TestFetchSessionsCarriesSessionID(t *testing.T) {
	fake := &fakeClient{
		sessions: []*gotmux.Session{{Name: "we:ird", Id: "$3", Windows: 1}},
		clients:  []*gotmux.Client{{Session: "we:ird"}},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_PANE", "")

	snap, err := FetchSessions("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Sessions) != 1 || snap.Sessions[0].ID != "$3" || snap.Sessions[0].Name != "we:ird" {
		t.Fatalf("expected session id $3 for we:ird, got %#v", snap.Sessions)
	}
}

func TestFetchSessionsExecFallbackCarriesSessionID(t *testing.T) {
	fake := &fakeClient{sessions: nil, clients: []*gotmux.Client{{Session: "we:ird"}}}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommanderContext(t, func(_ context.Context, _ string, args ...string) commander {
		if strings.Contains(strings.Join(args, " "), "list-sessions") {
			return stubCommander{output: []byte("$3\twe:ird\t2\t1\n")}
		}
		return stubCommander{output: nil}
	})
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_PANE", "")

	snap, err := FetchSessions("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Sessions) != 1 || snap.Sessions[0].ID != "$3" || snap.Sessions[0].Name != "we:ird" || snap.Sessions[0].Windows != 2 {
		t.Fatalf("expected exec fallback session with id, got %#v", snap.Sessions)
	}
}

func TestFetchWindowsCarriesIDsForColonSession(t *testing.T) {
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@7", Index: 1, Name: "do.tted", Active: true, ActiveSessionsList: []string{"we:ird"}},
		},
		clients: []*gotmux.Client{{Session: "we:ird"}},
		listWindowsFormatLines: []string{
			"@7\t$3\twe:ird\t1\twe:ird:1\twe:ird:1: do.tted",
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")

	snap, err := FetchWindows("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Windows) != 1 {
		t.Fatalf("expected 1 window, got %#v", snap.Windows)
	}
	w := snap.Windows[0]
	if w.ID != "we:ird:1" || w.Session != "we:ird" || w.Index != 1 || w.InternalID != "@7" || w.SessionID != "$3" {
		t.Fatalf("unexpected window entry %#v", w)
	}
}

// When the window is absent from ListAllWindows (a race), the snapshot must
// still take session/index from dedicated format fields rather than
// splitting the display id at its first ':'.
func TestFetchWindowsUnlistedWindowDoesNotParseDisplayID(t *testing.T) {
	fake := &fakeClient{
		windows: nil,
		clients: []*gotmux.Client{{Session: "we:ird"}},
		listWindowsFormatLines: []string{
			"@7\t$3\twe:ird\t1\twe:ird:1\twe:ird:1: do.tted",
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")

	snap, err := FetchWindows("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Windows) != 1 {
		t.Fatalf("expected 1 window, got %#v", snap.Windows)
	}
	w := snap.Windows[0]
	if w.Session != "we:ird" || w.Index != 1 || w.InternalID != "@7" || w.SessionID != "$3" {
		t.Fatalf("unexpected window entry %#v", w)
	}
}

func TestFetchPanesCarriesIDsForColonSession(t *testing.T) {
	fake := &fakeClient{
		panes: []*gotmux.Pane{{Id: "%9", Title: "top", Active: true}},
		listPanesFormatLines: []string{
			"%9\t@7\t$3\twe:ird:1.0\tlabel\twe:ird\tdo.tted\t1\t0\t1",
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "")

	snap, err := FetchPanes("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Panes) != 1 {
		t.Fatalf("expected 1 pane, got %#v", snap.Panes)
	}
	p := snap.Panes[0]
	if p.ID != "we:ird:1.0" || p.PaneID != "%9" || p.WindowID != "@7" || p.SessionID != "$3" ||
		p.Session != "we:ird" || p.Window != "do.tted" || p.WindowIdx != 1 || p.Index != 0 {
		t.Fatalf("unexpected pane entry %#v", p)
	}
}

func TestKillSessionsAcceptsSessionID(t *testing.T) {
	handle := &stubSessionHandle{id: "$3"}
	fake := &fakeClient{
		sessions:    []*gotmux.Session{{Name: "we:ird", Id: "$3"}},
		getSessions: map[string]*gotmux.Session{}, // name lookups must not be needed
	}
	fake.useSessionHandles(t, map[string]*stubSessionHandle{"we:ird": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillSessions("", []string{"$3"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.killCalls != 1 {
		t.Fatalf("expected kill call via session id, got %d", handle.killCalls)
	}
}

func TestFindSessionKeepsColonInName(t *testing.T) {
	handle := &stubSessionHandle{id: "$3"}
	fake := &fakeClient{
		getSessions: map[string]*gotmux.Session{
			"we:ird": {Name: "we:ird", Id: "$3"},
		},
	}
	fake.useSessionHandles(t, map[string]*stubSessionHandle{"we:ird": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillSessions("", []string{"we:ird"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.killCalls != 1 {
		t.Fatalf("expected kill call for we:ird, got %d", handle.killCalls)
	}
}

func TestNewSessionReturnsID(t *testing.T) {
	fake := &fakeClient{newSessionID: "$5"}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	id, err := NewSession("sock", "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "$5" {
		t.Fatalf("expected $5, got %q", id)
	}
}
