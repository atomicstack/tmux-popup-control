package state

import (
	"reflect"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestSetFilterTracksCursorAndRestoresPosition(t *testing.T) {
	level := newTestLevel("one", "two", "three")
	level.Cursor = 2
	level.SetFilter("two", len("two"))

	if level.Filter != "two" {
		t.Fatalf("expected filter persisted, got %q", level.Filter)
	}
	if level.FilterCursor != len("two") {
		t.Fatalf("expected cursor at end, got %d", level.FilterCursor)
	}
	if level.Cursor != 0 {
		t.Fatalf("expected filtered cursor at 0, got %d", level.Cursor)
	}
	if len(level.Items) != 1 || level.Items[0].ID != "two" {
		t.Fatalf("expected filtered items to contain only 'two', got %#v", level.Items)
	}

	level.SetFilter("", 0)
	if level.Cursor != 2 {
		t.Fatalf("expected cursor restored to 2, got %d", level.Cursor)
	}
	if level.LastCursor != -1 {
		t.Fatalf("expected last cursor reset, got %d", level.LastCursor)
	}
}

func TestInsertAndDeleteFilterText(t *testing.T) {
	level := newTestLevel("alpha")

	if !level.InsertFilterText("ab") {
		t.Fatal("expected insert to succeed")
	}
	if level.Filter != "ab" || level.FilterCursor != 2 {
		t.Fatalf("unexpected filter state %q/%d", level.Filter, level.FilterCursor)
	}

	level.FilterCursor = 1
	if !level.InsertFilterText("z") {
		t.Fatal("expected insert in middle to succeed")
	}
	if level.Filter != "azb" {
		t.Fatalf("expected insert into middle, got %q", level.Filter)
	}
	if level.FilterCursor != 2 {
		t.Fatalf("expected cursor 2 after insert, got %d", level.FilterCursor)
	}

	if !level.DeleteFilterRuneBackward() {
		t.Fatal("expected rune deletion to succeed")
	}
	if level.Filter != "ab" || level.FilterCursor != 1 {
		t.Fatalf("unexpected filter state after delete %q/%d", level.Filter, level.FilterCursor)
	}

	level.SetFilter("abc def", len("abc def"))
	if !level.DeleteFilterWordBackward() {
		t.Fatal("expected word deletion to succeed")
	}
	if level.Filter != "abc " {
		t.Fatalf("expected trailing word removed, got %q", level.Filter)
	}

	level.SetFilter("abc", 0)
	if level.DeleteFilterRuneBackward() {
		t.Fatal("expected delete at start to fail")
	}
}

func TestFilterCursorNavigation(t *testing.T) {
	level := newTestLevel("one", "two")
	level.SetFilter("one two", len("one two"))

	if !level.MoveFilterCursorWordBackward() {
		t.Fatal("expected word backward movement")
	}
	if level.FilterCursor != 4 {
		t.Fatalf("expected cursor at 4, got %d", level.FilterCursor)
	}
	if !level.MoveFilterCursorWordForward() {
		t.Fatal("expected word forward movement")
	}
	if level.FilterCursor != len("one two") {
		t.Fatalf("expected cursor restored to end, got %d", level.FilterCursor)
	}

	if !level.MoveFilterCursorRuneBackward() {
		t.Fatal("expected rune backward movement")
	}
	if level.FilterCursor != len("one two")-1 {
		t.Fatalf("expected cursor len-1, got %d", level.FilterCursor)
	}
	if !level.MoveFilterCursorRuneForward() {
		t.Fatal("expected rune forward movement")
	}
	if level.FilterCursor != len("one two") {
		t.Fatalf("expected cursor at end, got %d", level.FilterCursor)
	}
	if !level.MoveFilterCursorStart() {
		t.Fatal("expected move to start")
	}
	if level.FilterCursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", level.FilterCursor)
	}
	if !level.MoveFilterCursorEnd() {
		t.Fatal("expected move back to end")
	}
}

func TestFilterItemsAndClone(t *testing.T) {
	items := []menu.Item{{ID: "1", Label: "Alpha"}, {ID: "2", Label: "Beta"}}
	filtered := FilterItems(items, "alp")
	if len(filtered) != 1 || filtered[0].Label != "Alpha" {
		t.Fatalf("unexpected filtered results %#v", filtered)
	}
	filtered = FilterItems(items, "ta")
	if len(filtered) != 1 || filtered[0].Label != "Beta" {
		t.Fatalf("expected contains match for Beta, got %#v", filtered)
	}

	clone := CloneItems(items)
	if &clone[0] == &items[0] {
		t.Fatal("expected clone to allocate new backing array")
	}

	filtered[0].Label = "changed"
	if items[1].Label != "Beta" {
		t.Fatal("expected original slice to remain unchanged")
	}

	if len(FilterItems(items, "nomatch")) != 0 {
		t.Fatal("expected empty results when nothing matches")
	}
}

func TestBestMatchIndex(t *testing.T) {
	items := []menu.Item{
		{ID: "one", Label: "First"},
		{ID: "two", Label: "Second"},
		{ID: "three", Label: "Third"},
	}

	if idx := BestMatchIndex(items, "Second"); idx != 1 {
		t.Fatalf("expected exact label match index 1, got %d", idx)
	}
	if idx := BestMatchIndex(items, "two"); idx != 1 {
		t.Fatalf("expected ID match index 1, got %d", idx)
	}
	if idx := BestMatchIndex(items, "th"); idx != 2 {
		t.Fatalf("expected prefix match index 2, got %d", idx)
	}
	if idx := BestMatchIndex(items, "zzz"); idx != 0 {
		t.Fatalf("expected fallback index 0, got %d", idx)
	}
	if idx := BestMatchIndex(nil, "anything"); idx != -1 {
		t.Fatalf("expected -1 for empty slice, got %d", idx)
	}
}

func TestSetFilterSelectsFuzzyMatch(t *testing.T) {
	items := []menu.Item{{ID: "1", Label: "Alpha"}, {ID: "2", Label: "Beta"}}
	level := NewLevel("id", "title", items, nil)
	level.SetFilter("alp", 3)
	if level.Cursor != 0 {
		t.Fatalf("expected fuzzy match to select first item, got %d", level.Cursor)
	}
	if !reflect.DeepEqual(level.Items, []menu.Item{{ID: "1", Label: "Alpha"}}) {
		t.Fatalf("expected filtered items to contain Alpha, got %#v", level.Items)
	}
}
