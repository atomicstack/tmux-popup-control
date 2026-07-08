package extract

import "testing"

func TestGrabAreaString(t *testing.T) {
	cases := map[GrabArea]string{
		Viewport:      "viewport",
		PaneHistory:   "pane-history",
		Window:        "window",
		WindowHistory: "window-history",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("GrabArea(%d).String() = %q, want %q", a, got, want)
		}
	}

	// out-of-range value falls back to viewport.
	if got := GrabArea(99).String(); got != "viewport" {
		t.Errorf("GrabArea(99).String() = %q, want %q", got, "viewport")
	}
}

func TestGrabAreaNextWraps(t *testing.T) {
	order := []GrabArea{Viewport, PaneHistory, Window, WindowHistory, Viewport}
	got := Viewport
	for i := 1; i < len(order); i++ {
		got = got.Next()
		if got != order[i] {
			t.Fatalf("Next() step %d = %v, want %v", i, got, order[i])
		}
	}
}

func TestDefaultGrabArea(t *testing.T) {
	if DefaultGrabArea != Viewport {
		t.Fatalf("DefaultGrabArea = %v, want Viewport", DefaultGrabArea)
	}
}

// TestGrabAreasMatchesCycle ensures GrabAreas() is the single source of truth
// for the grab-area cycle order, and that callers cannot mutate package
// state through the returned slice.
func TestGrabAreasMatchesCycle(t *testing.T) {
	want := []GrabArea{Viewport, PaneHistory, Window, WindowHistory}
	got := GrabAreas()
	if len(got) != len(want) {
		t.Fatalf("GrabAreas() = %v, want %v", got, want)
	}
	for i, a := range want {
		if got[i] != a {
			t.Fatalf("GrabAreas()[%d] = %v, want %v", i, got[i], a)
		}
	}

	// mutating the returned slice must not affect a second call.
	got[0] = WindowHistory
	second := GrabAreas()
	if second[0] != Viewport {
		t.Fatalf("GrabAreas() copy semantics broken: mutating first result changed second call: %v", second)
	}
}
