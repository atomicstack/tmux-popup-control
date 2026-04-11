package ui

import "testing"

func TestRenderScrollbarHiddenWhenFits(t *testing.T) {
	cases := []struct {
		total, visible, start int
	}{
		{0, 10, 0},
		{5, 10, 0},
		{10, 10, 0},
	}
	for _, tc := range cases {
		if got := renderScrollbar(tc.total, tc.visible, tc.start); got != nil {
			t.Errorf("renderScrollbar(%d,%d,%d) = %q, want nil",
				tc.total, tc.visible, tc.start, string(got))
		}
	}
}

func TestRenderScrollbarTop(t *testing.T) {
	// thumb length = 10*10/20 = 5, at top of 10-row window.
	cells := renderScrollbar(20, 10, 0)
	want := "│││││     "
	if string(cells) != want {
		t.Errorf("top: got %q, want %q", string(cells), want)
	}
}

func TestRenderScrollbarBottom(t *testing.T) {
	cells := renderScrollbar(20, 10, 10)
	want := "     │││││"
	if string(cells) != want {
		t.Errorf("bottom: got %q, want %q", string(cells), want)
	}
}

func TestRenderScrollbarMiddle(t *testing.T) {
	// start=5, thumbLen=5, maxThumbStart=5, thumbStart = 5*5/10 = 2
	cells := renderScrollbar(20, 10, 5)
	want := "  │││││   "
	if string(cells) != want {
		t.Errorf("middle: got %q, want %q", string(cells), want)
	}
}

func TestRenderScrollbarThumbClampedToOne(t *testing.T) {
	// total=100, visible=5 → thumbLen = 5*5/100 = 0 → clamped to 1
	cells := renderScrollbar(100, 5, 0)
	if len(cells) != 5 {
		t.Fatalf("expected 5 cells, got %d", len(cells))
	}
	thumbs := 0
	for _, c := range cells {
		if c == '│' {
			thumbs++
		}
	}
	if thumbs != 1 {
		t.Errorf("expected 1 thumb cell, got %d (%q)", thumbs, string(cells))
	}
}

func TestRenderScrollbarThumbMovesWithStart(t *testing.T) {
	cells := renderScrollbar(100, 5, 99) // clamped to maxStart=95
	// thumbLen=1, maxThumbStart=4, thumbStart = 95*4/95 = 4 → last row
	if cells[4] != '│' {
		t.Errorf("expected thumb at bottom row, got %q", string(cells))
	}
	for i := 0; i < 4; i++ {
		if cells[i] != ' ' {
			t.Errorf("cell %d: expected track (space), got %q", i, cells[i])
		}
	}
}

func TestRenderScrollbarClampsNegativeStart(t *testing.T) {
	cells := renderScrollbar(20, 10, -5)
	if cells[0] != '│' {
		t.Errorf("expected thumb at top when start<0, got %q", string(cells))
	}
}
