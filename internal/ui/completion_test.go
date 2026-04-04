package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCompletionFilter(t *testing.T) {
	cs := newCompletionState([]string{"main", "work", "scratch"}, "target-session", "target-session", 5)
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
	cs := newCompletionState([]string{"a", "b", "c"}, "", "", 0)
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
	cs := newCompletionState([]string{"main", "work"}, "", "", 0)
	if cs.selected() != "main" {
		t.Fatalf("expected 'main', got %q", cs.selected())
	}
	cs.moveDown()
	if cs.selected() != "work" {
		t.Fatalf("expected 'work', got %q", cs.selected())
	}
}

func TestCompletionSelectedEmpty(t *testing.T) {
	cs := newCompletionState(nil, "", "", 0)
	if cs.selected() != "" {
		t.Fatalf("expected empty, got %q", cs.selected())
	}
}

func TestCompletionCursorWrapsUpFromFirstItem(t *testing.T) {
	cs := newCompletionState([]string{"a", "b", "c"}, "", "", 0)
	cs.moveUp()
	if cs.cursor != 2 {
		t.Fatalf("expected cursor to wrap to last item, got %d", cs.cursor)
	}
}

func TestCompletionCursorWrapsDownFromLastItem(t *testing.T) {
	cs := newCompletionState([]string{"a", "b", "c"}, "", "", 0)
	cs.cursor = 2
	cs.moveDown()
	if cs.cursor != 0 {
		t.Fatalf("expected cursor to wrap to first item, got %d", cs.cursor)
	}
}

func TestCompletionGhostHintNoInput(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	ghost := cs.ghostHint("")
	if ghost != "main" {
		t.Fatalf("expected ghost 'main', got %q", ghost)
	}
}

func TestCompletionGhostHintWithSelection(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	cs.moveDown()
	ghost := cs.ghostHint("")
	if ghost != "work" {
		t.Fatalf("expected ghost 'work', got %q", ghost)
	}
}

func TestCompletionGhostHintUniquePrefix(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	cs.applyFilter("ma")
	ghost := cs.ghostHint("ma")
	if ghost != "in" {
		t.Fatalf("expected ghost 'in', got %q", ghost)
	}
}

func TestCompletionGhostHintMultipleMatches(t *testing.T) {
	cs := newCompletionState([]string{"main", "master"}, "target-session", "target-session", 0)
	cs.applyFilter("ma")
	ghost := cs.ghostHint("ma")
	if ghost != "in" {
		t.Fatalf("expected ghost 'in' (from selected 'main'), got %q", ghost)
	}
}

func TestCompletionGhostHintNoMatch(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	cs.applyFilter("xyz")
	ghost := cs.ghostHint("xyz")
	if ghost != "" {
		t.Fatalf("expected empty ghost, got %q", ghost)
	}
}

func TestCompletionView(t *testing.T) {
	cs := newCompletionState([]string{"main", "work", "scratch"}, "target-session", "target-session", 0)
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
