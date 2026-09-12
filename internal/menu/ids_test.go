package menu

import (
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// weirdContext models a server whose session is named "we:ird" and whose
// window is named "do.tted" — both legal on tmux next-3.8 and both fatal for
// name-derived targets. Every action under test must build $N/@N/%N targets.
func weirdContext() Context {
	return Context{
		SocketPath: "sock",
		ClientID:   "/dev/ttys009",
		Sessions: []SessionEntry{
			{Name: "we:ird", ID: "$3", Label: "we:ird", Current: true, Windows: 2},
			{Name: "other", ID: "$4", Label: "other", Windows: 1},
		},
		Current: "we:ird",
		Windows: []WindowEntry{
			{ID: "we:ird:0", Label: "we:ird:0: shell", Name: "shell", Session: "we:ird", SessionID: "$3", Index: 0, InternalID: "@6"},
			{ID: "we:ird:1", Label: "we:ird:1: do.tted", Name: "do.tted", Session: "we:ird", SessionID: "$3", Index: 1, InternalID: "@7", Current: true},
			{ID: "other:0", Label: "other:0: vim", Name: "vim", Session: "other", SessionID: "$4", Index: 0, InternalID: "@8"},
		},
		CurrentWindowID:      "we:ird:1",
		CurrentWindowLabel:   "we:ird:1: do.tted",
		CurrentWindowSession: "we:ird",
		Panes: []PaneEntry{
			{ID: "we:ird:1.0", Label: "we:ird:1.0: zsh", PaneID: "%9", SessionID: "$3", WindowID: "@7", Session: "we:ird", Window: "do.tted", WindowIdx: 1, Index: 0, Current: true, Title: "zsh"},
			{ID: "we:ird:1.1", Label: "we:ird:1.1: vim", PaneID: "%10", SessionID: "$3", WindowID: "@7", Session: "we:ird", Window: "do.tted", WindowIdx: 1, Index: 1, Title: "vim"},
			{ID: "other:0.0", Label: "other:0.0: vim", PaneID: "%11", SessionID: "$4", WindowID: "@8", Session: "other", Window: "vim", WindowIdx: 0, Index: 0},
		},
		CurrentPaneID:    "we:ird:1.0",
		CurrentPaneLabel: "we:ird:1.0: zsh",
	}
}

func mustResult(t *testing.T, msg any) ActionResult {
	t.Helper()
	res, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	return res
}

func TestWindowSwitchActionTargetsIDs(t *testing.T) {
	var gotSession, gotWindow string
	defer withStub(&switchClientFn, func(_, _, target string) error { gotSession = target; return nil })()
	defer withStub(&selectWindowFn, func(_, target string) error { gotWindow = target; return nil })()

	ctx := weirdContext()
	mustResult(t, WindowSwitchAction(ctx, Item{ID: "we:ird:1", Label: "do.tted"})())
	if gotSession != "$3" || gotWindow != "@7" {
		t.Fatalf("expected switch-client $3 + select-window @7, got %q %q", gotSession, gotWindow)
	}
}

func TestWindowSwitchActionRejectsUnknownWindow(t *testing.T) {
	called := false
	defer withStub(&switchClientFn, func(_, _, _ string) error { called = true; return nil })()
	res := WindowSwitchAction(weirdContext(), Item{ID: "we:ird:9", Label: "?"})().(ActionResult)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "unknown window") {
		t.Fatalf("expected unknown window error, got %v", res.Err)
	}
	if called {
		t.Fatal("switch-client must not run for an unresolvable window")
	}
}

func TestPaneSwitchActionTargetsIDs(t *testing.T) {
	var got tmux.PaneRef
	defer withPaneStub(&switchPaneFn, func(_, _ string, ref tmux.PaneRef) error { got = ref; return nil })()

	mustResult(t, PaneSwitchAction(weirdContext(), Item{ID: "we:ird:1.1", Label: "vim"})())
	want := tmux.PaneRef{SessionID: "$3", WindowID: "@7", PaneID: "%10"}
	if got != want {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestPaneSwitchActionRejectsUnknownPane(t *testing.T) {
	called := false
	defer withPaneStub(&switchPaneFn, func(_, _ string, _ tmux.PaneRef) error { called = true; return nil })()
	res := PaneSwitchAction(weirdContext(), Item{ID: "we:ird:1.9", Label: "?"})().(ActionResult)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "unknown pane") {
		t.Fatalf("expected unknown pane error, got %v", res.Err)
	}
	if called {
		t.Fatal("switch-pane must not run for an unresolvable pane")
	}
}

func TestSessionSwitchActionTargetsSessionID(t *testing.T) {
	var got string
	defer withStub(&switchClientFn, func(_, _, target string) error { got = target; return nil })()
	mustResult(t, SessionSwitchAction(weirdContext(), Item{ID: "we:ird", Label: "we:ird"})())
	if got != "$3" {
		t.Fatalf("expected switch-client $3, got %q", got)
	}
}

func TestBuildTreeItemsUsesInternalIDs(t *testing.T) {
	ctx := weirdContext()
	items := NewTreeState(true).BuildTreeItems(TreeItemsInput{Sessions: ctx.Sessions, Windows: ctx.Windows, Panes: ctx.Panes})
	want := []string{
		"tree:s:$3",
		"tree:w:@6",
		"tree:w:@7",
		"tree:p:%9",
		"tree:p:%10",
		"tree:s:$4",
		"tree:w:@8",
		"tree:p:%11",
	}
	if len(items) != len(want) {
		t.Fatalf("expected %d items, got %d: %v", len(want), len(items), items)
	}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("item %d: expected %q, got %q", i, id, items[i].ID)
		}
	}
	if items[0].Label != "we:ird" || items[2].Label != "1: do.tted" {
		t.Fatalf("labels must stay human readable, got %q / %q", items[0].Label, items[2].Label)
	}
	if TreeSessionID("$3") != "tree:s:$3" || TreeWindowID("@7") != "tree:w:@7" || TreePaneID("%9") != "tree:p:%9" {
		t.Fatal("tree id helpers must wrap tmux ids")
	}
}

func TestFilterTreeItemsUsesInternalIDs(t *testing.T) {
	ctx := weirdContext()
	items := NewTreeState(false).FilterTreeItems(TreeItemsInput{Sessions: ctx.Sessions, Windows: ctx.Windows, Panes: ctx.Panes}, "vim")
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	joined := strings.Join(ids, " ")
	for _, want := range []string{"tree:s:$3", "tree:w:@7", "tree:p:%10", "tree:s:$4", "tree:w:@8"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("expected %q in filtered ids %v", want, ids)
		}
	}
}

func TestSessionTreeActionTargetsIDs(t *testing.T) {
	var switched, selectedWindow []string
	var paneRefs []tmux.PaneRef
	defer withStub(&switchClientFn, func(_, _, target string) error { switched = append(switched, target); return nil })()
	defer withStub(&selectWindowFn, func(_, target string) error { selectedWindow = append(selectedWindow, target); return nil })()
	defer withPaneStub(&switchPaneFn, func(_, _ string, ref tmux.PaneRef) error { paneRefs = append(paneRefs, ref); return nil })()

	ctx := weirdContext()
	mustResult(t, SessionTreeAction(ctx, Item{ID: "tree:s:$3", Label: "we:ird"})())
	if len(switched) != 1 || switched[0] != "$3" {
		t.Fatalf("session node: expected switch-client $3, got %v", switched)
	}

	switched = nil
	mustResult(t, SessionTreeAction(ctx, Item{ID: "tree:w:@7", Label: "1: do.tted"})())
	if len(switched) != 1 || switched[0] != "$3" || len(selectedWindow) != 1 || selectedWindow[0] != "@7" {
		t.Fatalf("window node: expected switch $3 + select @7, got %v %v", switched, selectedWindow)
	}

	switched = nil
	mustResult(t, SessionTreeAction(ctx, Item{ID: "tree:p:%10", Label: "1: vim"})())
	if len(switched) != 0 {
		t.Fatalf("pane node: expected no separate switch-client, got %v", switched)
	}
	want := tmux.PaneRef{SessionID: "$3", WindowID: "@7", PaneID: "%10"}
	if len(paneRefs) != 1 || paneRefs[0] != want {
		t.Fatalf("pane node: expected %+v, got %+v", want, paneRefs)
	}

	res := SessionTreeAction(ctx, Item{ID: "tree:w:@99"})().(ActionResult)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "unknown window") {
		t.Fatalf("expected unknown window error, got %v", res.Err)
	}
	res = SessionTreeAction(ctx, Item{ID: "tree:p:%99"})().(ActionResult)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "unknown pane") {
		t.Fatalf("expected unknown pane error, got %v", res.Err)
	}
}

func TestPaneBreakActionTargetsIDs(t *testing.T) {
	var gotSource, gotDest string
	defer withPaneStub(&breakPaneFn, func(_ string, source, dest string) error { gotSource, gotDest = source, dest; return nil })()
	mustResult(t, PaneBreakAction(weirdContext(), Item{ID: "we:ird:1.1", Label: "vim"})())
	if gotSource != "%10" || gotDest != "$3:2" {
		t.Fatalf("expected break-pane %%10 -> $3:2, got %q -> %q", gotSource, gotDest)
	}
}

func TestPaneBreakActionFallsBackToPaneSessionID(t *testing.T) {
	var gotDest string
	defer withPaneStub(&breakPaneFn, func(_ string, _, dest string) error { gotDest = dest; return nil })()
	ctx := weirdContext()
	ctx.CurrentWindowSession = ""
	mustResult(t, PaneBreakAction(ctx, Item{ID: "other:0.0", Label: "vim"})())
	if gotDest != "$4:1" {
		t.Fatalf("expected destination $4:1 from the pane's own session, got %q", gotDest)
	}
}

func TestWindowPullFromSessionActionTargetsIDs(t *testing.T) {
	var gotSource, gotSession string
	defer withStub(&moveWindowFn, func(_ string, source, target string) error { gotSource, gotSession = source, target; return nil })()
	mustResult(t, WindowPullFromSessionAction(weirdContext(), Item{ID: "tree:w:@8", Label: "0: vim"})())
	if gotSource != "@8" || gotSession != "$3" {
		t.Fatalf("expected move-window @8 -> $3, got %q -> %q", gotSource, gotSession)
	}
}

func TestWindowPushToSessionActionTargetsIDs(t *testing.T) {
	var gotSource, gotSession string
	defer withStub(&moveWindowFn, func(_ string, source, target string) error { gotSource, gotSession = source, target; return nil })()
	mustResult(t, WindowPushToSessionAction(weirdContext(), Item{ID: "other", Label: "other"})())
	if gotSource != "@7" || gotSession != "$4" {
		t.Fatalf("expected move-window @7 -> $4, got %q -> %q", gotSource, gotSession)
	}
}

func TestWindowKillActionTargetsInternalIDs(t *testing.T) {
	var got []string
	defer withStub(&unlinkWindowsFn, func(_ string, ids []string) error { got = ids; return nil })()
	mustResult(t, WindowKillAction(weirdContext(), Item{ID: "we:ird:0\nwe:ird:1", Label: "two"})())
	if len(got) != 2 || got[0] != "@7" || got[1] != "@6" {
		t.Fatalf("expected unlink [@7 @6], got %v", got)
	}
}

func TestWindowLinkActionTargetsIDs(t *testing.T) {
	var gotSource, gotSession string
	defer withStub(&linkWindowFn, func(_ string, source, target string) error { gotSource, gotSession = source, target; return nil })()
	mustResult(t, WindowLinkAction(weirdContext(), Item{ID: "other:0", Label: "vim"})())
	if gotSource != "@8" || gotSession != "$3" {
		t.Fatalf("expected link-window @8 -> $3, got %q -> %q", gotSource, gotSession)
	}
}

func TestWindowSwapCommandTargetsInternalIDs(t *testing.T) {
	var gotFirst, gotSecond string
	defer withStub(&swapWindowsFn, func(_ string, first, second string) error { gotFirst, gotSecond = first, second; return nil })()
	mustResult(t, WindowSwapCommand(weirdContext(), Item{ID: "we:ird:1", Label: "a"}, Item{ID: "other:0", Label: "b"})())
	if gotFirst != "@7" || gotSecond != "@8" {
		t.Fatalf("expected swap-window @7 @8, got %q %q", gotFirst, gotSecond)
	}
}

func TestWindowRenameCommandTargetsInternalID(t *testing.T) {
	var gotTarget string
	defer withStub(&renameWindowFn, func(_ string, target, _ string) error { gotTarget = target; return nil })()
	mustResult(t, WindowRenameCommand(RenameRequest{Context: weirdContext(), Target: "we:ird:1", Value: "renamed"})())
	if gotTarget != "@7" {
		t.Fatalf("expected rename-window @7, got %q", gotTarget)
	}
}

func TestPaneKillActionTargetsPaneIDs(t *testing.T) {
	var got []string
	defer withPaneStub(&killPanesFn, func(_ string, targets []string) error { got = targets; return nil })()
	mustResult(t, PaneKillAction(weirdContext(), Item{ID: "we:ird:1.0\nwe:ird:1.1", Label: "two"})())
	if len(got) != 2 || got[0] != "%10" || got[1] != "%9" {
		t.Fatalf("expected kill-pane [%%10 %%9], got %v", got)
	}
}

func TestPaneJoinActionTargetsPaneIDs(t *testing.T) {
	var gotSource, gotTarget string
	defer withPaneStub(&joinPaneFn, func(_ string, source, target string) error { gotSource, gotTarget = source, target; return nil })()
	mustResult(t, PaneJoinAction(weirdContext(), Item{ID: "we:ird:1.1", Label: "vim"})())
	if gotSource != "%10" || gotTarget != "%9" {
		t.Fatalf("expected join-pane %%10 -> %%9, got %q -> %q", gotSource, gotTarget)
	}
}

func TestPaneSwapCommandTargetsPaneIDs(t *testing.T) {
	var gotFirst, gotSecond string
	defer withPaneStub(&swapPanesFn, func(_ string, first, second string) error { gotFirst, gotSecond = first, second; return nil })()
	mustResult(t, PaneSwapCommand(weirdContext(), Item{ID: "we:ird:1.0", Label: "a"}, Item{ID: "other:0.0", Label: "b"})())
	if gotFirst != "%9" || gotSecond != "%11" {
		t.Fatalf("expected swap-pane %%9 %%11, got %q %q", gotFirst, gotSecond)
	}
}

func TestSessionKillActionTargetsSessionID(t *testing.T) {
	var got []string
	defer withStub(&killSessionsFn, func(_ string, targets []string) error { got = targets; return nil })()
	mustResult(t, SessionKillAction(weirdContext(), Item{ID: "we:ird", Label: "we:ird"})())
	if len(got) != 1 || got[0] != "$3" {
		t.Fatalf("expected kill-session [$3], got %v", got)
	}
}

func TestSessionDetachActionTargetsSessionID(t *testing.T) {
	var got []string
	defer withStub(&detachSessionsFn, func(_ string, targets []string) error { got = targets; return nil })()
	mustResult(t, SessionDetachAction(weirdContext(), Item{ID: "we:ird", Label: "we:ird"})())
	if len(got) != 1 || got[0] != "$3" {
		t.Fatalf("expected detach [$3], got %v", got)
	}
}

func TestSessionRenameCommandTargetsSessionID(t *testing.T) {
	var gotTarget, gotName string
	defer withStub(&renameSessionFn, func(_ string, target, name string) error { gotTarget, gotName = target, name; return nil })()
	mustResult(t, SessionRenameCommand(SessionRequest{Context: weirdContext(), Action: "session:rename", Target: "we:ird", Value: "plain"})())
	if gotTarget != "$3" || gotName != "plain" {
		t.Fatalf("expected rename-session $3 -> plain, got %q -> %q", gotTarget, gotName)
	}
}

func TestSessionCreateCommandSwitchesBySessionID(t *testing.T) {
	var switched string
	defer withStub(&createSessionFn, func(_, name string) (string, error) { return "$8", nil })()
	defer withStub(&switchClientFn, func(_, _, target string) error { switched = target; return nil })()
	mustResult(t, SessionCreateCommand(SessionRequest{Context: weirdContext(), Action: "session:new", Value: "dotted.name"})())
	if switched != "$8" {
		t.Fatalf("expected switch-client $8 for the new session, got %q", switched)
	}
}

func TestEntriesFromTmuxCopyIDs(t *testing.T) {
	sessions := SessionEntriesFromTmux([]tmux.Session{{Name: "we:ird", ID: "$3"}})
	if len(sessions) != 1 || sessions[0].ID != "$3" {
		t.Fatalf("expected session id copied, got %#v", sessions)
	}
	windows := WindowEntriesFromTmux([]tmux.Window{{ID: "we:ird:1", Session: "we:ird", SessionID: "$3", Index: 1, InternalID: "@7"}})
	if len(windows) != 1 || windows[0].SessionID != "$3" || windows[0].InternalID != "@7" {
		t.Fatalf("expected window ids copied, got %#v", windows)
	}
	panes := PaneEntriesFromTmux([]tmux.Pane{{ID: "we:ird:1.0", PaneID: "%9", SessionID: "$3", WindowID: "@7"}})
	if len(panes) != 1 || panes[0].PaneID != "%9" || panes[0].SessionID != "$3" || panes[0].WindowID != "@7" {
		t.Fatalf("expected pane ids copied, got %#v", panes)
	}
}

func TestWindowOrderKeyDoesNotParseDisplayID(t *testing.T) {
	entry := WindowEntry{ID: "we:ird:10", Session: "we:ird", Index: 10}
	key := buildWindowOrderKey(entry)
	if key.session != "we:ird" || !key.hasIndex || key.index != 10 {
		t.Fatalf("unexpected order key %+v", key)
	}
}
