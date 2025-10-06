package menu

import (
	"errors"
	"strings"
	"testing"
)

func withStub[T any](restore *T, value T) func() {
	original := *restore
	*restore = value
	return func() { *restore = original }
}

func TestLoadWindowSwitchMenuFiltersCurrent(t *testing.T) {
	ctx := Context{
		WindowIncludeCurrent: false,
		Windows: []WindowEntry{
			{ID: "s1:1", Label: "s1:1 main", Current: true, InternalID: "@1"},
			{ID: "s1:2", Label: "s1:2 dev", InternalID: "@2"},
		},
	}
	items, err := loadWindowSwitchMenu(ctx)
	if err != nil {
		t.Fatalf("loadWindowSwitchMenu returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != "s1:2" {
		t.Fatalf("unexpected items: %#v", items)
	}
	if !strings.Contains(items[0].Label, "s1:2") {
		t.Fatalf("expected window id in label, got %q", items[0].Label)
	}
}

func TestLoadWindowKillMenuIncludesCurrentMarker(t *testing.T) {
	ctx := Context{
		CurrentWindowID:    "s1:1",
		CurrentWindowLabel: "s1:1 main",
		Windows:            []WindowEntry{{ID: "s1:2", Label: "s1:2 dev", InternalID: "@2"}},
	}
	items, err := loadWindowKillMenu(ctx)
	if err != nil {
		t.Fatalf("loadWindowKillMenu returned error: %v", err)
	}
	if len(items) == 0 || !strings.HasPrefix(items[0].Label, "[current]") {
		t.Fatalf("expected current entry first, got %#v", items)
	}
}

func TestWindowRenameMenuTabular(t *testing.T) {
	ctx := Context{
		Windows: []WindowEntry{
			{ID: "s1:1", Name: "main", Session: "s1", Index: 1, Current: true, Label: "s1:1 main", InternalID: "@1"},
			{ID: "s1:2", Name: "dev", Session: "s1", Index: 2, Label: "s1:2 dev", InternalID: "@2"},
		},
		CurrentWindowID:    "s1:1",
		CurrentWindowLabel: "s1:1 main",
	}
	items, err := loadWindowRenameMenu(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if strings.Contains(items[0].Label, "[current]") {
		t.Fatalf("unexpected legacy marker in label: %q", items[0].Label)
	}
	if !strings.Contains(items[0].Label, "current") {
		t.Fatalf("expected current marker column in %q", items[0].Label)
	}
	if !strings.Contains(items[0].Label, "s1:1") {
		t.Fatalf("expected window id in label, got %q", items[0].Label)
	}
	if !strings.Contains(items[0].Label, "@1") {
		t.Fatalf("expected internal window id in label, got %q", items[0].Label)
	}
	if strings.Count(items[0].Label, " ") < 3 {
		t.Fatalf("expected tabular columns in %q", items[0].Label)
	}
}

func TestWindowSwitchItemsTabular(t *testing.T) {
	ctx := Context{
		WindowIncludeCurrent: true,
		Windows: []WindowEntry{
			{ID: "s1:1", Name: "main", Label: "s1:1 main", Index: 1, Current: true, InternalID: "@1"},
			{ID: "s1:2", Name: "dev", Label: "s1:2 dev", Index: 2, InternalID: "@2"},
		},
	}
	items, err := loadWindowSwitchMenu(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if !strings.Contains(items[0].Label, "current") {
		t.Fatalf("expected current marker in first label, got %q", items[0].Label)
	}
	if !strings.Contains(items[0].Label, "@1") {
		t.Fatalf("expected internal id column in %q", items[0].Label)
	}
}

func TestWindowSwitchItemsSortedBySessionAndID(t *testing.T) {
	ctx := Context{
		WindowIncludeCurrent: false,
		Windows: []WindowEntry{
			{ID: "b:3", Name: "b-3", Session: "b", Index: 3, Label: "b:3", InternalID: "@13"},
			{ID: "a:10", Name: "a-10", Session: "a", Index: 10, Label: "a:10", InternalID: "@10"},
			{ID: "a:2", Name: "a-2", Session: "a", Index: 2, Label: "a:2", InternalID: "@2"},
			{ID: "a:3", Name: "a-3", Session: "a", Index: 3, Label: "a:3", InternalID: "@3"},
		},
	}
	items := WindowSwitchItems(ctx)
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	expected := []string{"a:2", "a:3", "a:10", "b:3"}
	for i, id := range expected {
		if items[i].ID != id {
			t.Fatalf("expected %s at position %d, got %s", id, i, items[i].ID)
		}
	}
}

func TestSplitWindowIDs(t *testing.T) {
	ids := splitWindowIDs("win1\nwin2,win3 \nwin2")
	if len(ids) != 3 {
		t.Fatalf("expected 3 ids, got %d (%v)", len(ids), ids)
	}
	expected := []string{"win1", "win2", "win3"}
	for i, id := range expected {
		if ids[i] != id {
			t.Fatalf("expected %s at %d, got %s", id, i, ids[i])
		}
	}
}

func TestWindowKillActionMultiSelect(t *testing.T) {
	called := false
	restore := withStub(&unlinkWindowsFn, func(_ string, ids []string) error {
		called = true
		if len(ids) != 2 || ids[0] != "w2" || ids[1] != "w1" {
			t.Fatalf("unexpected ids: %v", ids)
		}
		return nil
	})
	defer restore()

	ctx := Context{SocketPath: "sock"}
	cmd := WindowKillAction(ctx, Item{ID: "w1\nw2", Label: "items"})
	msg := cmd()
	res, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if !called {
		t.Fatalf("unlinkWindowsFn not called")
	}
	if !strings.Contains(res.Info, "2") {
		t.Fatalf("unexpected info message: %q", res.Info)
	}
}

func TestWindowLinkActionUsesCurrentSession(t *testing.T) {
	var gotSource, gotSession string
	restore := withStub(&linkWindowFn, func(_ string, source, targetSession string) error {
		gotSource, gotSession = source, targetSession
		return nil
	})
	defer restore()

	ctx := Context{SocketPath: "sock", CurrentWindowSession: "main"}
	cmd := WindowLinkAction(ctx, Item{ID: "sess:1", Label: "sess:1"})
	msg := cmd()
	res := msg.(ActionResult)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
	if gotSource != "sess:1" || gotSession != "main" {
		t.Fatalf("unexpected call args: %s, %s", gotSource, gotSession)
	}
}

func TestWindowSwapActionReturnsPrompt(t *testing.T) {
	ctx := Context{}
	cmd := WindowSwapAction(ctx, Item{ID: "s1:1", Label: "s1:1"})
	msg := cmd()
	prompt, ok := msg.(WindowSwapPrompt)
	if !ok {
		t.Fatalf("expected WindowSwapPrompt, got %T", msg)
	}
	if prompt.First.ID != "s1:1" {
		t.Fatalf("unexpected prompt first: %#v", prompt)
	}
}

func TestWindowRenameCommandUsesStub(t *testing.T) {
	restore := withStub(&renameWindowFn, func(_ string, target, name string) error {
		if target != "s1:1" || name != "new" {
			t.Fatalf("unexpected args %s %s", target, name)
		}
		return nil
	})
	defer restore()

	ctx := Context{SocketPath: "sock"}
	cmd := WindowRenameCommand(ctx, "s1:1", "new")
	msg := cmd()
	res := msg.(ActionResult)
	if res.Err != nil {
		t.Fatalf("unexpected error: %v", res.Err)
	}
}

func TestWindowKillActionPropagatesError(t *testing.T) {
	called := false
	restore := withStub(&unlinkWindowsFn, func(_ string, ids []string) error {
		called = true
		return errors.New("boom")
	})
	defer restore()

	ctx := Context{SocketPath: "sock"}
	msg := WindowKillAction(ctx, Item{ID: "w1"})()
	res := msg.(ActionResult)
	if res.Err == nil {
		t.Fatalf("expected error")
	}
	if !called {
		t.Fatalf("unlinkWindowsFn not called")
	}
}
