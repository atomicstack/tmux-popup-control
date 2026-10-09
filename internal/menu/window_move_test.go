package menu

import (
	"fmt"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func windowMoveTestContext() Context {
	win := func(session, sid string, idx, id int, name string, current bool) WindowEntry {
		return WindowEntry{
			ID:         fmt.Sprintf("%s:%d", session, idx),
			Label:      fmt.Sprintf("%s:%d: %s", session, idx, name),
			Name:       name,
			Session:    session,
			SessionID:  sid,
			Index:      idx,
			InternalID: fmt.Sprintf("@%d", id),
			Current:    current,
		}
	}
	return Context{
		Sessions: []SessionEntry{
			{Name: "alpha", ID: "$0"},
			{Name: "beta", ID: "$1"},
		},
		Windows: []WindowEntry{
			win("alpha", "$0", 1, 1, "one", false),
			win("alpha", "$0", 2, 2, "two", true),
			win("alpha", "$0", 3, 3, "three", false),
			win("beta", "$1", 1, 4, "four", false),
			win("beta", "$1", 5, 5, "five", false),
		},
		CurrentWindowID:      "alpha:2",
		CurrentWindowSession: "alpha",
	}
}

func itemIDs(items []Item) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func TestWindowMoveStateStartsAtOrigin(t *testing.T) {
	s, ok := NewWindowMoveState(windowMoveTestContext())
	if !ok {
		t.Fatal("expected a move state")
	}
	if s.Moved() {
		t.Fatal("fresh state must not report a move")
	}
	want := []string{"tree:s:$0", "tree:w:@1", "tree:w:@2", "tree:w:@3", "tree:s:$1", "tree:w:@4", "tree:w:@5"}
	if got := itemIDs(s.Items()); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("items = %v, want %v", got, want)
	}
	if c := s.Cursor(); c != 2 {
		t.Fatalf("cursor = %d, want 2 (the pinned window)", c)
	}
	if label := s.Items()[2].Label; label != WindowMoveMarker+"2: two" {
		t.Fatalf("pinned label = %q", label)
	}
}

func TestWindowMoveStateNoCurrentWindow(t *testing.T) {
	ctx := windowMoveTestContext()
	ctx.CurrentWindowID = ""
	ctx.CurrentWindowSession = ""
	for i := range ctx.Windows {
		ctx.Windows[i].Current = false
	}
	if _, ok := NewWindowMoveState(ctx); ok {
		t.Fatal("expected no move state without a current window")
	}
}

func TestWindowMoveStateStepClampsAndCrossesSessions(t *testing.T) {
	s, _ := NewWindowMoveState(windowMoveTestContext())
	// alpha slots: before one, before three(origin), after three; beta: 3 slots.
	if !s.Step(-1) || s.Cursor() != 1 {
		t.Fatalf("up once: cursor = %d, want 1", s.Cursor())
	}
	if s.Step(-1) {
		t.Fatal("stepping above the first slot must not move")
	}
	s.StepEnd()
	want := []string{"tree:s:$0", "tree:w:@1", "tree:w:@3", "tree:s:$1", "tree:w:@4", "tree:w:@5", "tree:w:@2"}
	if got := itemIDs(s.Items()); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("items at end = %v, want %v", got, want)
	}
	if s.Cursor() != 6 {
		t.Fatalf("cursor at end = %d, want 6", s.Cursor())
	}
	if s.Step(1) {
		t.Fatal("stepping past the last slot must not move")
	}
	s.StepHome()
	s.Step(1)
	if s.Moved() {
		t.Fatal("returning to the origin slot must not report a move")
	}
}

func TestWindowMoveStatePlan(t *testing.T) {
	cases := []struct {
		name  string
		steps int
		want  WindowMovePlan
		label string
	}{
		{
			name:  "top of own session inserts before first window",
			steps: -1,
			want:  WindowMovePlan{Source: "@2", Target: "@1", Placement: tmux.PlaceBefore, TargetSession: "alpha", Index: 1},
			label: WindowMoveMarker + "1: two",
		},
		{
			name:  "after last window of own session uses free index",
			steps: 1,
			want:  WindowMovePlan{Source: "@2", Target: "$0:4", Placement: tmux.PlaceAt, TargetSession: "alpha", Index: 4},
			label: WindowMoveMarker + "4: two",
		},
		{
			name:  "top of other session inserts before its first window",
			steps: 2,
			want:  WindowMovePlan{Source: "@2", Target: "@4", Placement: tmux.PlaceBefore, TargetSession: "beta", Index: 1},
			label: WindowMoveMarker + "1: two",
		},
		{
			name:  "gap in other session uses free index",
			steps: 3,
			want:  WindowMovePlan{Source: "@2", Target: "$1:2", Placement: tmux.PlaceAt, TargetSession: "beta", Index: 2},
			label: WindowMoveMarker + "2: two",
		},
		{
			name:  "after last window of other session",
			steps: 4,
			want:  WindowMovePlan{Source: "@2", Target: "$1:6", Placement: tmux.PlaceAt, TargetSession: "beta", Index: 6},
			label: WindowMoveMarker + "6: two",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := NewWindowMoveState(windowMoveTestContext())
			s.Step(tc.steps)
			if !s.Moved() {
				t.Fatal("expected a move")
			}
			if got := s.Plan(); got != tc.want {
				t.Fatalf("plan = %+v, want %+v", got, tc.want)
			}
			if got := s.Items()[s.Cursor()].Label; got != tc.label {
				t.Fatalf("pinned label = %q, want %q", got, tc.label)
			}
		})
	}
}

func TestWindowMoveStatePlanAfterOccupiedIndexInserts(t *testing.T) {
	ctx := windowMoveTestContext()
	// Make the current window alpha:3 so moving it up lands between 1 and 2,
	// where index 2 is taken: tmux must shuffle with -a.
	ctx.Windows[1].Current = false
	ctx.Windows[2].Current = true
	ctx.CurrentWindowID = "alpha:3"
	s, _ := NewWindowMoveState(ctx)
	s.Step(-1)
	want := WindowMovePlan{Source: "@3", Target: "@1", Placement: tmux.PlaceAfter, TargetSession: "alpha", Index: 2}
	if got := s.Plan(); got != want {
		t.Fatalf("plan = %+v, want %+v", got, want)
	}
}

func TestWindowMoveCommandRunsPlan(t *testing.T) {
	var gotSource, gotTarget string
	var gotPlacement tmux.WindowPlacement
	orig := moveWindowToFn
	moveWindowToFn = func(_, source, target string, placement tmux.WindowPlacement) error {
		gotSource, gotTarget, gotPlacement = source, target, placement
		return nil
	}
	t.Cleanup(func() { moveWindowToFn = orig })

	plan := WindowMovePlan{Source: "@2", Target: "@4", Placement: tmux.PlaceBefore, TargetSession: "beta", Index: 1}
	msg := WindowMoveCommand("sock", plan)()
	res, ok := msg.(ActionResult)
	if !ok || res.Err != nil {
		t.Fatalf("unexpected result %#v", msg)
	}
	if gotSource != "@2" || gotTarget != "@4" || gotPlacement != tmux.PlaceBefore {
		t.Fatalf("move called with %s %s %d", gotSource, gotTarget, gotPlacement)
	}
	if res.Info != "Moved window to beta:1" {
		t.Fatalf("info = %q", res.Info)
	}
}

func TestWindowMoveStateShowsShuffledIndices(t *testing.T) {
	// alpha: 0 editor, 1 logs, 3 runner(current), 5 shell. One step up puts
	// the runner after editor with -a: it lands on 1 and logs shuffles to 2.
	ctx := Context{
		Sessions: []SessionEntry{{Name: "alpha", ID: "$0"}},
		Windows: []WindowEntry{
			{ID: "alpha:0", Label: "alpha:0: editor", Session: "alpha", Index: 0, InternalID: "@0"},
			{ID: "alpha:1", Label: "alpha:1: logs", Session: "alpha", Index: 1, InternalID: "@1"},
			{ID: "alpha:3", Label: "alpha:3: runner", Session: "alpha", Index: 3, InternalID: "@3", Current: true},
			{ID: "alpha:5", Label: "alpha:5: shell", Session: "alpha", Index: 5, InternalID: "@5"},
		},
		CurrentWindowID:      "alpha:3",
		CurrentWindowSession: "alpha",
	}
	labels := func(s *WindowMoveState) []string {
		var out []string
		for _, it := range s.Items()[1:] {
			out = append(out, it.Label)
		}
		return out
	}
	s, _ := NewWindowMoveState(ctx)
	s.Step(-1)
	want := []string{"0: editor", WindowMoveMarker + "1: runner", "2: logs", "5: shell"}
	if got := labels(s); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("labels = %q, want %q", got, want)
	}
	// Top of the session: -b before editor shuffles the contiguous run 0,1
	// up; 3 (the source itself) stops the run at 2.
	s.Step(-1)
	want = []string{WindowMoveMarker + "0: runner", "1: editor", "2: logs", "5: shell"}
	if got := labels(s); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("labels = %q, want %q", got, want)
	}
}
