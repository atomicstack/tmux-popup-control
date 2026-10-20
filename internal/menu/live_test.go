package menu

import (
	"slices"
	"testing"
)

func TestSessionSwitchMenuItemsFiltersCurrent(t *testing.T) {
	ctx := Context{
		IncludeCurrent: false,
		Sessions: []SessionEntry{
			{Name: "one", Label: "one", Current: true},
			{Name: "two", Label: "two"},
		},
	}
	items := SessionSwitchMenuItems(ctx)
	if len(items) != 1 || items[0].ID != "two" {
		t.Fatalf("expected only non-current session, got %#v", items)
	}
}

func TestCurrentWindowMenuItem(t *testing.T) {
	ctx := Context{CurrentWindowID: "s:1", CurrentWindowLabel: "s:1 main"}
	item, ok := CurrentWindowMenuItem(ctx)
	if !ok {
		t.Fatalf("expected item")
	}
	if item.ID != "s:1" || item.Label != "[current] s:1 main" {
		t.Fatalf("unexpected item: %#v", item)
	}
	if _, ok := CurrentWindowMenuItem(Context{CurrentWindowID: "  "}); ok {
		t.Fatalf("expected no item for blank window id")
	}
}

func TestCurrentPaneMenuItemFallsBackToID(t *testing.T) {
	item, ok := CurrentPaneMenuItem(Context{CurrentPaneID: " s:0.1 "})
	if !ok || item.ID != "s:0.1" || item.Label != "[current] s:0.1" {
		t.Fatalf("unexpected item: %#v ok=%v", item, ok)
	}
	if _, ok := CurrentPaneMenuItem(Context{}); ok {
		t.Fatalf("expected no item without a current pane")
	}
}

func TestWindowPushToSessionItemsSkipsCurrentWindowSession(t *testing.T) {
	ctx := Context{
		CurrentWindowSession: "a",
		Sessions:             []SessionEntry{{Name: "a", Label: "A"}, {Name: "b", Label: "B"}},
	}
	items := WindowPushToSessionItems(ctx)
	if len(items) != 1 || items[0].ID != "b" || items[0].Label != "B" {
		t.Fatalf("unexpected items: %#v", items)
	}
}

func TestWindowSwapItemsPrependsCurrent(t *testing.T) {
	ctx := Context{
		CurrentWindowID:    "@1",
		CurrentWindowLabel: "main",
		Windows:            []WindowEntry{{ID: "s:0", Label: "zero"}, {ID: "s:1", Label: "one"}},
	}
	got := itemIDs(WindowSwapItems(ctx))
	want := []string{"@1", "s:0", "s:1"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := itemIDs(WindowSwapItems(Context{Windows: ctx.Windows})); !slices.Equal(got, want[1:]) {
		t.Fatalf("without current: got %v", got)
	}
}

func paneCtx() Context {
	return Context{
		CurrentPaneID:    "s:0.0",
		CurrentPaneLabel: "cur",
		Panes: []PaneEntry{
			{ID: "s:0.0", Label: "p0", Current: true},
			{ID: "s:0.1", Label: "p1"},
		},
	}
}

func TestPaneItemBuilders(t *testing.T) {
	ctx := paneCtx()
	withCurrent := []string{"s:0.0", "s:0.0", "s:0.1"}
	cases := []struct {
		name string
		fn   func(Context) []Item
		want []string
	}{
		{"PaneItems", PaneItems, []string{"s:0.0", "s:0.1"}},
		{"PaneSwitchItems", PaneSwitchItems, []string{"s:0.1"}},
		{"PaneJoinItems", PaneJoinItems, []string{"s:0.1"}},
		{"PaneBreakItems", PaneBreakItems, withCurrent},
		{"PaneSwapItems", PaneSwapItems, withCurrent},
		{"PaneKillItems", PaneKillItems, withCurrent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := itemIDs(tc.fn(ctx)); !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	if items := PaneKillItems(ctx); items[0].Label != "[current] cur" {
		t.Fatalf("expected [current] prefix, got %q", items[0].Label)
	}
}

func TestPaneSwitchItemsIncludeCurrent(t *testing.T) {
	ctx := paneCtx()
	ctx.PaneIncludeCurrent = true
	if got := itemIDs(PaneSwitchItems(ctx)); !slices.Equal(got, []string{"s:0.0", "s:0.1"}) {
		t.Fatalf("got %v", got)
	}
}

func TestLiveLevelsTable(t *testing.T) {
	want := map[LiveSource][]string{
		LiveSessions: {"session:switch", "session:rename", "session:detach", "session:kill", "window:push-to-session"},
		LiveWindows:  {"window:switch", "window:swap", "window:kill"},
		LivePanes:    {"pane:switch", "pane:break", "pane:join", "pane:swap", "pane:kill", "pane:rename"},
	}
	reg := BuildRegistry()
	for src, ids := range want {
		var got []string
		for _, l := range LiveLevels(src) {
			if l.Items == nil {
				t.Fatalf("%s has nil builder", l.ID)
			}
			if _, ok := reg.Find(l.ID); !ok {
				t.Fatalf("%s is not a registered menu node", l.ID)
			}
			got = append(got, l.ID)
		}
		if !slices.Equal(got, ids) {
			t.Fatalf("source %d: got %v, want %v", src, got, ids)
		}
	}
	if fn, ok := LiveLevelItems("pane:kill"); !ok || fn == nil {
		t.Fatalf("expected builder for pane:kill")
	}
	for _, id := range []string{"session:tree", "window:pull-from-session", "nope"} {
		if _, ok := LiveLevelItems(id); ok {
			t.Fatalf("%s must not be table-driven", id)
		}
	}
}
