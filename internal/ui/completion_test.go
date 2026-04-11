package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestCompletionViewWidthStableWhileScrolling pins the rendered popup width
// against the widest item in the full filtered set, so scrolling (which shifts
// which items are visible) must not change the popup's width.
func TestCompletionViewWidthStableWhileScrolling(t *testing.T) {
	// The widest row is the first item; the rest are short. With a 10-row
	// viewport, scrolling to the bottom of the list pushes the widest row
	// out of view, which is where the old "measure visible only" logic
	// reflowed the popup narrower.
	items := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaa", // widest
		"b",
		"c",
		"d",
		"e",
		"f",
		"g",
		"h",
		"i",
		"j",
		"k",
		"l",
		"m",
		"n",
		"o",
	}
	cs := newCompletionState(CompletionOptions{Items: items})

	widthAtCursor := func(cursor int) int {
		cs.cursor = cursor
		out := cs.view(200, 20)
		longest := 0
		for line := range strings.SplitSeq(out, "\n") {
			if w := ansi.StringWidth(line); w > longest {
				longest = w
			}
		}
		return longest
	}

	base := widthAtCursor(0)
	for cursor := 1; cursor < len(items); cursor++ {
		if got := widthAtCursor(cursor); got != base {
			t.Errorf("popup width at cursor %d = %d, want %d (scrolling should not reflow the popup)", cursor, got, base)
		}
	}
}

// TestCompletionViewWidthCapped pins the popup width cap at 50 visible columns
// so a single absurdly wide candidate cannot blow the popup across the screen.
func TestCompletionViewWidthCapped(t *testing.T) {
	items := []string{strings.Repeat("x", 120)}
	cs := newCompletionState(CompletionOptions{Items: items})
	out := cs.view(200, 20)
	longest := 0
	for line := range strings.SplitSeq(out, "\n") {
		if w := ansi.StringWidth(line); w > longest {
			longest = w
		}
	}
	// Popup = border (2) + scrollbar (0, only one row) + padding (2) + content.
	// With content capped at 50 visible cols, total visible width is 54.
	const maxPopupWidth = 54
	if longest > maxPopupWidth {
		t.Errorf("popup width = %d, expected ≤ %d", longest, maxPopupWidth)
	}
}

func TestCompletionFilter(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "work", "scratch"}, ArgType: "target-session", TypeLabel: "target-session", AnchorCol: 5})
	if len(cs.filtered) != 3 {
		t.Fatalf("expected 3 filtered items, got %d", len(cs.filtered))
	}

	cs.applyFilter("ma")
	if len(cs.filtered) != 1 {
		t.Fatalf("expected 1 filtered item for 'ma', got %d: %v", len(cs.filtered), cs.filtered)
	}
	if cs.filtered[0].Value != "main" {
		t.Fatalf("expected 'main', got %q", cs.filtered[0].Value)
	}
}

func TestCompletionCursorBounds(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"a", "b", "c"}})
	cs.moveDown()
	if cs.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", cs.cursor)
	}
	cs.moveDown()
	if cs.cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", cs.cursor)
	}
	cs.moveUp()
	if cs.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", cs.cursor)
	}
	cs.moveUp()
	if cs.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", cs.cursor)
	}
}

func TestCompletionSelected(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "work"}})
	if cs.selected() != "main" {
		t.Fatalf("expected 'main', got %q", cs.selected())
	}
	cs.moveDown()
	if cs.selected() != "work" {
		t.Fatalf("expected 'work', got %q", cs.selected())
	}
}

func TestCompletionSelectedEmpty(t *testing.T) {
	cs := newCompletionState(CompletionOptions{})
	if cs.selected() != "" {
		t.Fatalf("expected empty, got %q", cs.selected())
	}
}

func TestCompletionCursorWrapsUpFromFirstItem(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"a", "b", "c"}})
	cs.moveUp()
	if cs.cursor != 2 {
		t.Fatalf("expected cursor to wrap to last item, got %d", cs.cursor)
	}
}

func TestCompletionCursorWrapsDownFromLastItem(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"a", "b", "c"}})
	cs.cursor = 2
	cs.moveDown()
	if cs.cursor != 0 {
		t.Fatalf("expected cursor to wrap to first item, got %d", cs.cursor)
	}
}

func TestCompletionGhostHintNoInput(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "work"}, ArgType: "target-session", TypeLabel: "target-session"})
	ghost := cs.ghostHint("")
	if ghost != "main" {
		t.Fatalf("expected ghost 'main', got %q", ghost)
	}
}

func TestCompletionGhostHintWithSelection(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "work"}, ArgType: "target-session", TypeLabel: "target-session"})
	cs.moveDown()
	ghost := cs.ghostHint("")
	if ghost != "work" {
		t.Fatalf("expected ghost 'work', got %q", ghost)
	}
}

func TestCompletionGhostHintUniquePrefix(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "work"}, ArgType: "target-session", TypeLabel: "target-session"})
	cs.applyFilter("ma")
	ghost := cs.ghostHint("ma")
	if ghost != "in" {
		t.Fatalf("expected ghost 'in', got %q", ghost)
	}
}

func TestCompletionGhostHintMultipleMatches(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "master"}, ArgType: "target-session", TypeLabel: "target-session"})
	cs.applyFilter("ma")
	ghost := cs.ghostHint("ma")
	if ghost != "in" {
		t.Fatalf("expected ghost 'in' (from selected 'main'), got %q", ghost)
	}
}

func TestCompletionGhostHintNoMatch(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "work"}, ArgType: "target-session", TypeLabel: "target-session"})
	cs.applyFilter("xyz")
	ghost := cs.ghostHint("xyz")
	if ghost != "" {
		t.Fatalf("expected empty ghost, got %q", ghost)
	}
}

func TestCompletionView(t *testing.T) {
	cs := newCompletionState(CompletionOptions{Items: []string{"main", "work", "scratch"}, ArgType: "target-session", TypeLabel: "target-session"})
	view := cs.view(30, 10)
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	for _, name := range []string{"main", "work", "scratch"} {
		if !strings.Contains(view, name) {
			t.Errorf("view should contain %q", name)
		}
	}
}

func TestCompletionViewAlignsDescriptions(t *testing.T) {
	cs := newCompletionStateWithItems([]completionItem{
		{Value: "-a", Label: "-a", Description: "insert after target window"},
		{Value: "-s", Label: "-s <src-window>", Description: "source window"},
		{Value: "-t", Label: "-t <dst-window>", Description: "destination window"},
	}, "", "", 0)

	view := ansi.Strip(cs.view(80, 10))
	if !strings.Contains(view, "-s <src-window>") {
		t.Fatalf("expected left column in view, got:\n%s", view)
	}
	if !strings.Contains(view, "destination window") {
		t.Fatalf("expected description in view, got:\n%s", view)
	}
}

func TestCompletionViewLeavesPlainValuesUnchanged(t *testing.T) {
	cs := newCompletionStateWithItems([]completionItem{
		{Value: "main:0", Label: "main:0"},
		{Value: "work:1", Label: "work:1"},
	}, "", "", 0)

	view := ansi.Strip(cs.view(40, 10))
	if !strings.Contains(view, "main:0") || !strings.Contains(view, "work:1") {
		t.Fatalf("expected plain completion values in view, got:\n%s", view)
	}
	if strings.Contains(view, "destination window") {
		t.Fatalf("did not expect description text in plain value view, got:\n%s", view)
	}
}
