package menu

import (
	"errors"
	"strings"
	"testing"
)

func findTestContext() Context {
	return Context{
		SocketPath: "sock",
		Windows: []WindowEntry{
			{ID: "main:0", Name: "editor", InternalID: "@1"},
			{ID: "main:1", Name: "logs", InternalID: "@2"},
			{ID: "other:0", Name: "shell", InternalID: "@3"},
		},
		Panes: []PaneEntry{
			{ID: "main:0.0", PaneID: "%1", WindowID: "@1"},
			{ID: "main:0.1", PaneID: "%2", WindowID: "@1"},
			{ID: "main:1.0", PaneID: "%3", WindowID: "@2"},
			{ID: "other:0.0", PaneID: "%4", WindowID: "@3"},
		},
	}
}

// stubFindCaptures installs separate viewport and history texts per pane id
// and records which capture kind each pane was asked for.
func stubFindCaptures(t *testing.T, visible, history map[string]string) *[]string {
	t.Helper()
	var calls []string
	restore := SetFindCaptureForTest(
		func(socket, target string) (string, error) {
			calls = append(calls, "visible "+target)
			return visible[target], nil
		},
		func(socket, target string) (string, error) {
			calls = append(calls, "history "+target)
			return history[target], nil
		},
	)
	t.Cleanup(restore)
	return &calls
}

func TestFindPanesSearchesViewportOnlyByDefault(t *testing.T) {
	calls := stubFindCaptures(t,
		map[string]string{"%1": "nothing here", "%2": "build FAILED: exit 2", "%3": "tail -f", "%4": ""},
		map[string]string{"%1": "old failed run\nnothing here"},
	)
	got, err := FindPanes(findTestContext(), "failed", false)
	if err != nil {
		t.Fatalf("FindPanes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(got), got)
	}
	if got[0].Item.ID != "main:0.1" || got[0].PaneID != "%2" {
		t.Fatalf("result = %+v, want pane main:0.1 (%%2)", got[0])
	}
	if !strings.Contains(got[0].Item.Label, "main:0.1") || !strings.Contains(got[0].Item.Label, "build FAILED: exit 2") {
		t.Fatalf("label %q should carry the pane id and the matching line", got[0].Item.Label)
	}
	for _, c := range *calls {
		if strings.HasPrefix(c, "history") {
			t.Fatalf("viewport search must not capture history, got call %q", c)
		}
	}
}

func TestFindPanesSearchesHistoryWhenEnabled(t *testing.T) {
	stubFindCaptures(t,
		map[string]string{"%1": "nothing here"},
		map[string]string{"%1": "old failed run\nnothing here", "%4": "also Failed"},
	)
	got, err := FindPanes(findTestContext(), "failed", true)
	if err != nil {
		t.Fatalf("FindPanes: %v", err)
	}
	if len(got) != 2 || got[0].Item.ID != "main:0.0" || got[1].Item.ID != "other:0.0" {
		t.Fatalf("results = %+v, want main:0.0 then other:0.0", got)
	}
}

func TestFindPanesBlankQueryReturnsNothingWithoutCapturing(t *testing.T) {
	calls := stubFindCaptures(t, map[string]string{"%1": "anything"}, nil)
	got, err := FindPanes(findTestContext(), "  ", false)
	if err != nil || len(got) != 0 {
		t.Fatalf("FindPanes(blank) = %+v, %v; want none", got, err)
	}
	if len(*calls) != 0 {
		t.Fatalf("blank query captured panes: %v", *calls)
	}
}

func TestFindPanesSkipsPanesThatFailToCapture(t *testing.T) {
	restore := SetFindCaptureForTest(
		func(socket, target string) (string, error) {
			if target == "%1" {
				return "", errors.New("can't find pane: %1")
			}
			return "needle", nil
		},
		nil,
	)
	t.Cleanup(restore)
	got, err := FindPanes(findTestContext(), "needle", false)
	if err != nil {
		t.Fatalf("a vanished pane must not fail the search: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d results, want the 3 panes that captured: %+v", len(got), got)
	}
}

func TestFindPanesFailsWhenEveryCaptureFails(t *testing.T) {
	restore := SetFindCaptureForTest(
		func(string, string) (string, error) { return "", errors.New("no server") },
		nil,
	)
	t.Cleanup(restore)
	if _, err := FindPanes(findTestContext(), "needle", false); err == nil {
		t.Fatal("expected an error when no pane could be captured")
	}
}

func TestFindWindowsGroupsMatchingPanesByWindow(t *testing.T) {
	stubFindCaptures(t,
		map[string]string{"%1": "needle one", "%2": "needle two", "%3": "nope", "%4": "a Needle"},
		nil,
	)
	got, err := FindWindows(findTestContext(), "needle", false)
	if err != nil {
		t.Fatalf("FindWindows: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2 windows: %+v", len(got), got)
	}
	if got[0].Item.ID != "main:0" || got[0].PaneID != "%1" {
		t.Fatalf("first result = %+v, want window main:0 via its first matching pane %%1", got[0])
	}
	if got[1].Item.ID != "other:0" || got[1].PaneID != "%4" {
		t.Fatalf("second result = %+v, want window other:0 via pane %%4", got[1])
	}
	for _, want := range []string{"main:0", "editor", "needle one"} {
		if !strings.Contains(got[0].Item.Label, want) {
			t.Fatalf("label %q missing %q", got[0].Item.Label, want)
		}
	}
}

func TestFindLevelsAreRegistered(t *testing.T) {
	reg := BuildRegistry()
	for _, id := range []string{PaneFindID, WindowFindID} {
		node, ok := reg.Find(id)
		if !ok || node.Loader == nil || node.Action == nil {
			t.Fatalf("%s must have a loader and an action", id)
		}
		if !node.ContentSearch {
			t.Fatalf("%s must be marked as a content-search level", id)
		}
	}
	if _, ok := reg.Child("pane", "find-pane-by-content"); !ok {
		t.Fatal("pane menu is missing find-pane-by-content")
	}
	if _, ok := reg.Child("window", "find-window-by-content"); !ok {
		t.Fatal("window menu is missing find-window-by-content")
	}
	paneItems, _ := loadPaneMenu(Context{})
	windowItems, _ := loadWindowMenu(Context{})
	if !hasItem(paneItems, "find-pane-by-content") || !hasItem(windowItems, "find-window-by-content") {
		t.Fatal("find entries must be listed in the pane and window menus")
	}
}

func hasItem(items []Item, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}
