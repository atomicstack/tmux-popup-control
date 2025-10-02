package menu

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func withPaneStub[T any](restore *T, value T) func() {
	original := *restore
	*restore = value
	return func() { *restore = original }
}

func TestLoadPaneSwitchMenu(t *testing.T) {
	ctx := Context{
		PaneIncludeCurrent: false,
		Panes: []PaneEntry{
			{ID: "s:1.0", Label: "s:1.0", Current: true},
			{ID: "s:1.1", Label: "s:1.1"},
		},
	}
	items, err := loadPaneSwitchMenu(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "s:1.1" {
		t.Fatalf("unexpected items %#v", items)
	}
}

func TestPaneEntriesFromTmux(t *testing.T) {
	panes := []tmux.Pane{{ID: "s:1.0", PaneID: "%1", Label: "pane", Session: "s", Window: "win", WindowIdx: 1, Index: 0}}
	entries := PaneEntriesFromTmux(panes)
	if len(entries) != 1 || entries[0].ID != "s:1.0" || entries[0].PaneID != "%1" {
		t.Fatalf("unexpected entries %#v", entries)
	}
}

func TestPaneKillActionUsesStub(t *testing.T) {
	var got []string
	restore := withPaneStub(&killPanesFn, func(_ string, targets []string) error {
		got = append([]string(nil), targets...)
		return nil
	})
	defer restore()

	ctx := Context{SocketPath: "sock"}
	res := PaneKillAction(ctx, Item{ID: "b\na", Label: " "})()
	if res.(ActionResult).Err != nil {
		t.Fatalf("unexpected error: %v", res.(ActionResult).Err)
	}
	if len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("unexpected targets %v", got)
	}
}

func TestPaneSwapActionReturnsPrompt(t *testing.T) {
	ctx := Context{}
	msg := PaneSwapAction(ctx, Item{ID: "s:1.0", Label: "x"})()
	if _, ok := msg.(PaneSwapPrompt); !ok {
		t.Fatalf("expected PaneSwapPrompt, got %T", msg)
	}
}

func TestPaneResizeLeftAction(t *testing.T) {
	var dir string
	var amt int
	restore := withPaneStub(&resizePaneFn, func(_ string, direction string, amount int) error {
		dir = direction
		amt = amount
		return nil
	})
	defer restore()
	ctx := Context{SocketPath: "sock"}
	res := PaneResizeLeftAction(ctx, Item{ID: "5"})()
	if res.(ActionResult).Err != nil {
		t.Fatalf("unexpected error: %v", res.(ActionResult).Err)
	}
	if dir != "left" || amt != 5 {
		t.Fatalf("unexpected args %s %d", dir, amt)
	}
}

func TestPaneJoinActionUsesMovePane(t *testing.T) {
	var calls []string
	restore := withPaneStub(&movePaneFn, func(_ string, source, target string) error {
		calls = append(calls, source)
		if target != "" {
			t.Fatalf("expected empty target, got %s", target)
		}
		return nil
	})
	defer restore()
	ctx := Context{SocketPath: "sock", CurrentPaneID: "s:1.0"}
	res := PaneJoinAction(ctx, Item{ID: "s:1.2\ns:1.1", Label: ""})()
	if res.(ActionResult).Err != nil {
		t.Fatalf("unexpected error: %v", res.(ActionResult).Err)
	}
	if len(calls) != 2 || calls[0] != "s:1.2" || calls[1] != "s:1.1" {
		t.Fatalf("unexpected move sequence %v", calls)
	}
}

func TestPaneBreakActionComputesDestination(t *testing.T) {
	var gotSource, gotDest string
	restore := withPaneStub(&breakPaneFn, func(_ string, source, dest string) error {
		gotSource, gotDest = source, dest
		return nil
	})
	defer restore()
	ctx := Context{
		SocketPath:           "sock",
		CurrentWindowSession: "sess",
		Windows: []WindowEntry{
			{Session: "sess", Index: 0},
			{Session: "sess", Index: 2},
		},
	}
	res := PaneBreakAction(ctx, Item{ID: "sess:0.1", Label: "pane"})()
	if res.(ActionResult).Err != nil {
		t.Fatalf("unexpected error: %v", res.(ActionResult).Err)
	}
	if gotSource != "sess:0.1" || gotDest != "sess:3" {
		t.Fatalf("unexpected break args %s %s", gotSource, gotDest)
	}
}
