package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/charmbracelet/x/ansi"
)

func altA() tea.KeyPressMsg { return tea.KeyPressMsg{Code: 'a', Mod: tea.ModAlt} }

func typeText(h *Harness, text string) {
	for _, r := range text {
		h.Send(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// newFindHarness opens rootMenu directly over three panes in two windows,
// with separate viewport and history texts per pane id.
func newFindHarness(t *testing.T, rootMenu string, visible, history map[string]string) *Harness {
	t.Helper()
	t.Cleanup(menu.SetFindCaptureForTest(
		func(_, target string) (string, error) { return visible[target], nil },
		func(_, target string) (string, error) { return history[target], nil },
	))
	m := NewModel(ModelConfig{Width: 120, Height: 30, RootMenu: rootMenu, SocketPath: "test.sock"})
	m.windows.SetEntries([]menu.WindowEntry{
		{ID: "main:0", Name: "editor", InternalID: "@1", Label: "editor"},
		{ID: "main:1", Name: "logs", InternalID: "@2", Label: "logs"},
	})
	m.panes.SetEntries([]menu.PaneEntry{
		{ID: "main:0.0", PaneID: "%1", WindowID: "@1", Label: "one"},
		{ID: "main:0.1", PaneID: "%2", WindowID: "@1", Label: "two"},
		{ID: "main:1.0", PaneID: "%3", WindowID: "@2", Label: "three"},
	})
	return NewHarness(m)
}

func itemIDs(l *level) []string {
	ids := make([]string, 0, len(l.Items))
	for _, it := range l.Items {
		ids = append(ids, it.ID)
	}
	return ids
}

func TestFindPaneLevelStartsEmptyWithUncheckedHistoryBox(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, nil, nil)
	lvl := h.Model().currentLevel()
	if lvl == nil || lvl.ID != menu.PaneFindID {
		t.Fatalf("expected the find level to be current, got %+v", lvl)
	}
	if len(lvl.Items) != 0 {
		t.Fatalf("find level must open with no results, got %v", itemIDs(lvl))
	}
	view := ansi.Strip(h.View())
	if !strings.Contains(view, "□ all history <alt+a>") {
		t.Fatalf("view should show the unchecked history tick box:\n%s", view)
	}
	if !strings.Contains(view, "(type to search pane contents)") {
		t.Fatalf("an empty find level should prompt for a search:\n%s", view)
	}
	if strings.Contains(view, "No entries found") {
		t.Fatalf("an unsearched find level must not report missing entries:\n%s", view)
	}
}

func TestFindPaneTypingSearchesVisibleContent(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID,
		map[string]string{"%1": "vim main.go", "%2": "go test: FAIL TestThing", "%3": "tail -f app.log"},
		map[string]string{"%1": "earlier: fail whale\nvim main.go"},
	)
	typeText(h, "fail")
	lvl := h.Model().currentLevel()
	if got := itemIDs(lvl); len(got) != 1 || got[0] != "main:0.1" {
		t.Fatalf("results = %v, want only main:0.1 (history must not be searched)", got)
	}
	if lvl.Filter != "fail" {
		t.Fatalf("filter = %q, want the typed query", lvl.Filter)
	}
	view := ansi.Strip(h.View())
	for _, want := range []string{"go test: FAIL TestThing", "1 pane matches"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestFindPaneAltAEnablesHistorySearch(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID,
		map[string]string{"%1": "vim main.go", "%2": "go test: FAIL TestThing"},
		map[string]string{"%1": "earlier: fail whale\nvim main.go", "%2": "go test: FAIL TestThing"},
	)
	typeText(h, "fail")
	h.Send(altA())
	lvl := h.Model().currentLevel()
	if got := itemIDs(lvl); len(got) != 2 || got[0] != "main:0.0" || got[1] != "main:0.1" {
		t.Fatalf("results with history = %v, want main:0.0 and main:0.1", got)
	}
	if lvl.Filter != "fail" {
		t.Fatalf("alt+a must not edit the query, filter = %q", lvl.Filter)
	}
	if view := ansi.Strip(h.View()); !strings.Contains(view, "■ all history <alt+a>") || !strings.Contains(view, "2 panes match") {
		t.Fatalf("view should show the checked box and the match count:\n%s", view)
	}

	h.Send(altA())
	if got := itemIDs(h.Model().currentLevel()); len(got) != 1 || got[0] != "main:0.1" {
		t.Fatalf("results after unchecking = %v, want only main:0.1", got)
	}
}

func TestFindResultsAreNotFilteredByLabel(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, map[string]string{"%1": "needle"}, nil)
	typeText(h, "needle")
	lvl := h.Model().currentLevel()
	// A row whose label lacks the query would vanish under the normal fuzzy
	// filter; on a search level every result the search returned stays.
	lvl.UpdateItems([]menu.Item{{ID: "main:0.0", Label: "unrelated label"}})
	if len(lvl.Items) != 1 {
		t.Fatalf("content-search level filtered its results by label: %v", itemIDs(lvl))
	}
}

func TestFindClearingQueryClearsResults(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, map[string]string{"%1": "needle"}, nil)
	typeText(h, "needle")
	if len(h.Model().currentLevel().Items) != 1 {
		t.Fatal("expected one result before clearing")
	}
	h.Send(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if got := itemIDs(h.Model().currentLevel()); len(got) != 0 {
		t.Fatalf("results after clearing the query = %v, want none", got)
	}
}

func TestFindNoMatchesIsReported(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, map[string]string{"%1": "hay"}, nil)
	typeText(h, "needle")
	if view := ansi.Strip(h.View()); !strings.Contains(view, `no matches for "needle"`) {
		t.Fatalf("view should say there are no matches:\n%s", view)
	}
}

func TestFindWindowLevelListsWindows(t *testing.T) {
	h := newFindHarness(t, menu.WindowFindID,
		map[string]string{"%1": "needle a", "%2": "needle b", "%3": "nothing"},
		nil,
	)
	typeText(h, "needle")
	lvl := h.Model().currentLevel()
	if got := itemIDs(lvl); len(got) != 1 || got[0] != "main:0" {
		t.Fatalf("results = %v, want the single window main:0", got)
	}
	if view := ansi.Strip(h.View()); !strings.Contains(view, "1 window matches") {
		t.Fatalf("view should count windows:\n%s", view)
	}
}

// Searches are single-flight: typing while one is running must not start
// another, and the newest query is searched once the running one lands.
func TestFindSearchIsSingleFlight(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, map[string]string{"%1": "abc", "%2": "ab only"}, nil)
	first := h.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	st := findStateOf(h.Model().currentLevel())
	if !st.running || st.query != "a" {
		t.Fatalf("first key should start a search for %q, state %+v", "a", st)
	}
	h.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	h.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if st.query != "a" {
		t.Fatalf("a second search started while one was running (query %q)", st.query)
	}

	// Land the first search: its results apply, then "abc" is searched.
	h.processCmd(first)
	lvl := h.Model().currentLevel()
	if st.running || st.applied != "abc" {
		t.Fatalf("after the first search landed, want results for %q, state %+v", "abc", st)
	}
	if got := itemIDs(lvl); len(got) != 1 || got[0] != "main:0.0" {
		t.Fatalf("results = %v, want only main:0.0", got)
	}
}

func TestFindStaleResultFromEarlierVisitIsDropped(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, map[string]string{"%1": "needle"}, nil)
	stale := findResultMsg{
		state:   &findState{},
		query:   "old",
		results: []menu.FindResult{{Item: menu.Item{ID: "ghost:0.0", Label: "ghost"}, PaneID: "%9"}},
	}
	h.Send(stale)
	if got := itemIDs(h.Model().currentLevel()); len(got) != 0 {
		t.Fatalf("a result for another visit was applied: %v", got)
	}
}

func TestFindLaunchedDirectlyQuitsOnEscape(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, nil, nil)
	cmd := h.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("esc on a directly launched find level should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("esc should produce a quit, got %T", cmd())
	}
}

func TestFindPreviewHighlightsAndScrollsToMatch(t *testing.T) {
	text := "l0\nl1\nl2\nl3\nl4\nl5 has a Needle here\nl6\n"
	lines, row := findPreviewLines(text, "needle")
	if row != 5 {
		t.Fatalf("first match row = %d, want 5", row)
	}
	if len(lines) != 7 {
		t.Fatalf("got %d lines, want 7 (trailing blank dropped)", len(lines))
	}
	if got := ansi.Strip(lines[5]); got != "l5 has a Needle here" {
		t.Fatalf("highlighting changed the text: %q", got)
	}
	if !strings.Contains(lines[5], findMatchStyle.Render("Needle")) {
		t.Fatalf("matched text is not highlighted: %q", lines[5])
	}

	h := newFindHarness(t, menu.PaneFindID,
		map[string]string{"%1": text},
		nil,
	)
	typeText(h, "needle")
	preview := h.Model().activePreview()
	if preview == nil {
		t.Fatal("expected a preview for the matching pane")
	}
	if preview.target != "main:0.0" || preview.label != "main:0.0" {
		t.Fatalf("preview target/label = %q/%q, want the pane id", preview.target, preview.label)
	}
	if want := 5 - findPreviewContext; preview.scrollOffset != want {
		t.Fatalf("preview scrollOffset = %d, want %d (match row minus context)", preview.scrollOffset, want)
	}

	// A refresh for the same pane and query keeps wherever the user scrolled.
	preview.scrollOffset = 1
	h.Send(previewTickMsg{})
	if got := h.Model().activePreview().scrollOffset; got != 1 {
		t.Fatalf("refresh moved the preview to %d, want the user's offset 1", got)
	}
}
