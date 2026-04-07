package menu

import (
	"testing"

	tea "charm.land/bubbletea/v2"

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

func TestPaneCaptureActionReturnsPrompt(t *testing.T) {
	ctx := Context{SocketPath: "sock", CurrentPaneID: "%3"}
	msg := PaneCaptureAction(ctx, Item{ID: "pane:capture", Label: "capture"})()
	prompt, ok := msg.(PaneCapturePrompt)
	if !ok {
		t.Fatalf("expected PaneCapturePrompt, got %T", msg)
	}
	if prompt.Context.CurrentPaneID != "%3" {
		t.Errorf("pane ID = %q, want %%3", prompt.Context.CurrentPaneID)
	}
	if prompt.Template == "" {
		t.Error("template should not be empty")
	}
}

func TestPaneCaptureActionEmptyPaneID(t *testing.T) {
	ctx := Context{SocketPath: "sock", CurrentPaneID: ""}
	msg := PaneCaptureAction(ctx, Item{ID: "pane:capture"})()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected error for empty pane ID")
	}
}

func TestPaneRenameCommandUsesStub(t *testing.T) {
	restore := withPaneStub(&renamePaneFn, func(_ string, target, title string) error {
		if target != "%1" || title != "new-title" {
			t.Fatalf("unexpected args %s %s", target, title)
		}
		return nil
	})
	defer restore()

	ctx := Context{
		SocketPath: "sock",
		Panes: []PaneEntry{
			{ID: "s:1.0", PaneID: "%1", Label: "pane-label"},
		},
	}
	cmd := PaneRenameCommand(RenameRequest{Context: ctx, Target: "s:1.0", Value: "new-title"})
	msg := cmd()
	res := msg.(ActionResult)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
}

func TestPaneRenameFormEnterReturnsCommand(t *testing.T) {
	form := NewPaneRenameForm(RenamePrompt{
		Context: Context{
			SocketPath: "sock",
			Panes: []PaneEntry{
				{ID: "s:1.0", PaneID: "%1", Label: "pane-label"},
			},
		},
		Target:  "s:1.0",
		Initial: "old-title",
	})
	cmd, done, cancel := form.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cancel {
		t.Fatal("enter should not cancel")
	}
	if !done {
		t.Fatal("enter should submit")
	}
	if cmd == nil {
		t.Fatal("expected submit command from pane rename form")
	}
}

func TestPaneCaptureFormToggleEscSeqs(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	if form.EscSeqs() {
		t.Fatal("escSeqs should default to false")
	}
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !form.EscSeqs() {
		t.Fatal("escSeqs should be true after tab")
	}
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if form.EscSeqs() {
		t.Fatal("escSeqs should be false after second tab")
	}
}

func TestPaneCaptureFormEscCancels(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	_, done, cancel := form.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !cancel {
		t.Fatal("esc should cancel")
	}
	if done {
		t.Fatal("esc should not signal done")
	}
}

func TestPaneCaptureFormEnterSubmits(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	_, done, cancel := form.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !done {
		t.Fatal("enter should signal done")
	}
	if cancel {
		t.Fatal("enter should not cancel")
	}
	if form.Value() != "test.log" {
		t.Errorf("Value() = %q, want %q", form.Value(), "test.log")
	}
}

func TestPaneCaptureFormCtrlUClears(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	form.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if form.Value() != "" {
		t.Errorf("Value() = %q after ctrl+u, want empty", form.Value())
	}
}

func TestPaneCaptureFormSeqIncrementsOnInput(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	seq0 := form.Seq()
	form.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if form.Seq() <= seq0 {
		t.Error("seq should increment on input change")
	}
}

func TestLoadPaneMenuIncludesCapture(t *testing.T) {
	items, err := loadPaneMenu(Context{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.ID == "capture" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("pane menu should include 'capture' item")
	}
}
