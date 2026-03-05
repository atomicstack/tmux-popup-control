package tree

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestBuildItemsCollapsed(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 2, Current: true},
		{Name: "work", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", Session: "main"},
		{ID: "@2", Label: "vim", Session: "main"},
		{ID: "@3", Label: "htop", Session: "work"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", Window: "@1", Session: "main"},
		{ID: "%2", Label: "pane-2", Window: "@2", Session: "main"},
		{ID: "%3", Label: "pane-3", Window: "@3", Session: "work"},
	}

	state := NewState(false) // all collapsed
	items := state.BuildItems(sessions, windows, panes)

	if len(items) != 2 {
		t.Fatalf("expected 2 items (sessions only), got %d", len(items))
	}
	if items[0].ID != "tree:s:main" {
		t.Errorf("expected tree:s:main, got %s", items[0].ID)
	}
	if items[1].ID != "tree:s:work" {
		t.Errorf("expected tree:s:work, got %s", items[1].ID)
	}
}

func TestBuildItemsExpanded(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 2},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", Session: "main"},
		{ID: "@2", Label: "vim", Session: "main"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", Window: "@1", Session: "main"},
	}

	state := NewState(false)
	state.SetExpanded("tree:s:main", true)
	items := state.BuildItems(sessions, windows, panes)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != "tree:s:main" {
		t.Errorf("expected tree:s:main, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:main:@1" {
		t.Errorf("expected tree:w:main:@1, got %s", items[1].ID)
	}
	if items[2].ID != "tree:w:main:@2" {
		t.Errorf("expected tree:w:main:@2, got %s", items[2].ID)
	}
}

func TestBuildItemsFullyExpanded(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", Session: "main"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", Window: "@1", Session: "main"},
		{ID: "%2", Label: "pane-2", Window: "@1", Session: "main"},
	}

	state := NewState(true) // all expanded
	items := state.BuildItems(sessions, windows, panes)

	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	if items[0].ID != "tree:s:main" {
		t.Errorf("[0] expected tree:s:main, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:main:@1" {
		t.Errorf("[1] expected tree:w:main:@1, got %s", items[1].ID)
	}
	if items[2].ID != "tree:p:main:@1:%1" {
		t.Errorf("[2] expected tree:p:main:@1:%%1, got %s", items[2].ID)
	}
	if items[3].ID != "tree:p:main:@1:%2" {
		t.Errorf("[3] expected tree:p:main:@1:%%2, got %s", items[3].ID)
	}
}

func TestFilterItemsPreservesAncestors(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 2},
		{Name: "work", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", Session: "main"},
		{ID: "@2", Label: "vim", Session: "main"},
		{ID: "@3", Label: "htop", Session: "work"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", Window: "@1", Session: "main"},
		{ID: "%2", Label: "vim-pane", Window: "@2", Session: "main"},
		{ID: "%3", Label: "htop-pane", Window: "@3", Session: "work"},
	}

	state := NewState(false)
	items := state.FilterItems(sessions, windows, panes, "vim")

	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	if len(items) < 2 {
		t.Fatalf("expected at least 2 items, got %d: %v", len(items), ids)
	}

	found := false
	for _, item := range items {
		if item.ID == "tree:s:main" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ancestor session main to be preserved, got %v", ids)
	}
}

func TestFilterItemsNoMatchReturnsEmpty(t *testing.T) {
	sessions := []menu.SessionEntry{{Name: "main", Windows: 1}}
	windows := []menu.WindowEntry{{ID: "@1", Label: "bash", Session: "main"}}
	panes := []menu.PaneEntry{{ID: "%1", Label: "pane-1", Window: "@1", Session: "main"}}

	state := NewState(false)
	items := state.FilterItems(sessions, windows, panes, "zzzznotfound")
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestFilterItemsEmptyQueryReturnsNormal(t *testing.T) {
	sessions := []menu.SessionEntry{{Name: "main", Windows: 1}}
	windows := []menu.WindowEntry{{ID: "@1", Label: "bash", Session: "main"}}
	panes := []menu.PaneEntry{}

	state := NewState(false)
	items := state.FilterItems(sessions, windows, panes, "")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestToggle(t *testing.T) {
	state := NewState(false)
	id := "tree:s:main"

	if state.IsExpanded(id) {
		t.Fatal("expected collapsed by default")
	}
	state.Toggle(id)
	if !state.IsExpanded(id) {
		t.Fatal("expected expanded after toggle")
	}
	state.Toggle(id)
	if state.IsExpanded(id) {
		t.Fatal("expected collapsed after second toggle")
	}
}

func TestItemKind(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"tree:s:main", "session"},
		{"tree:w:main:@1", "window"},
		{"tree:p:main:@1:%1", "pane"},
		{"other", ""},
	}
	for _, tt := range tests {
		if got := ItemKind(tt.id); got != tt.want {
			t.Errorf("ItemKind(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestIsExpandable(t *testing.T) {
	if !IsExpandable("tree:s:main") {
		t.Error("session should be expandable")
	}
	if !IsExpandable("tree:w:main:@1") {
		t.Error("window should be expandable")
	}
	if IsExpandable("tree:p:main:@1:%1") {
		t.Error("pane should not be expandable")
	}
}
