package menu

import (
	"slices"
	"testing"
)

// These tests characterize the behaviour of the unified multi-select ID
// splitter (splitSelectionIDs). Prior to unification there were three separate
// implementations with divergent delimiters:
//   - splitWindowIDs:      split on '\n' and ',' with a raw-string fallback
//   - splitPaneIDs:        split on '\n', ',' and ' ' with no fallback
//   - parseMultiSelectIDs: split on '\n' only
//
// The UI joins marked selections with "\n" (see internal/ui/navigation.go),
// so the only delimiter that is actually reachable from the UI is '\n'.
// Comma/space splitting was dead for real IDs and is actively harmful because
// tmux targets may legitimately contain spaces (session names). The unified
// splitter therefore splits on '\n' only, trims each segment, drops blanks,
// and de-duplicates while preserving order.

func TestSplitSelectionIDs_NewlineOnly(t *testing.T) {
	got := splitSelectionIDs("win1\nwin2\nwin3\nwin2")
	want := []string{"win1", "win2", "win3"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSplitSelectionIDs_Empty(t *testing.T) {
	if got := splitSelectionIDs(""); got != nil {
		t.Fatalf("expected nil for empty input, got %v", got)
	}
	if got := splitSelectionIDs("   "); got != nil {
		t.Fatalf("expected nil for blank input, got %v", got)
	}
}

func TestSplitSelectionIDs_PreservesSpacesAndCommasWithinIDs(t *testing.T) {
	// An ID containing a comma or space must survive intact — only '\n'
	// separates distinct selections.
	got := splitSelectionIDs("my session\nwith,comma")
	want := []string{"my session", "with,comma"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSplitSelectionIDs_TrimsSegments(t *testing.T) {
	got := splitSelectionIDs("  a  \n b \n")
	want := []string{"a", "b"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSplitSelectionIDs_SingleID(t *testing.T) {
	got := splitSelectionIDs("session:0")
	want := []string{"session:0"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
