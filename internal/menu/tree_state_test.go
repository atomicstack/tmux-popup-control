package menu

import "testing"

func TestBuildItemsCollapsed(t *testing.T) {
	sessions := []SessionEntry{
		{Name: "main", Windows: 2, Current: true},
		{Name: "work", Windows: 1},
	}
	windows := []WindowEntry{
		{ID: "@1", Label: "bash", Session: "main", Index: 0},
		{ID: "@2", Label: "vim", Session: "main", Index: 1},
		{ID: "@3", Label: "htop", Session: "work", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "main"},
		{ID: "%2", Label: "pane-2", WindowIdx: 1, Session: "main"},
		{ID: "%3", Label: "pane-3", WindowIdx: 0, Session: "work"},
	}

	state := NewTreeState(false)
	items := state.BuildTreeItems(sessions, windows, panes)

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
	sessions := []SessionEntry{{Name: "main", Windows: 2}}
	windows := []WindowEntry{
		{ID: "@1", Label: "bash", Session: "main", Index: 0},
		{ID: "@2", Label: "vim", Session: "main", Index: 1},
	}
	panes := []PaneEntry{{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "main"}}

	state := NewTreeState(false)
	state.SetExpanded("tree:s:main", true)
	items := state.BuildTreeItems(sessions, windows, panes)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[1].ID != "tree:w:main:0" {
		t.Errorf("expected tree:w:main:0, got %s", items[1].ID)
	}
	if items[2].ID != "tree:w:main:1" {
		t.Errorf("expected tree:w:main:1, got %s", items[2].ID)
	}
}

func TestBuildItemsFullyExpanded(t *testing.T) {
	sessions := []SessionEntry{{Name: "main", Windows: 1}}
	windows := []WindowEntry{{ID: "@1", Label: "bash", Session: "main", Index: 0}}
	panes := []PaneEntry{
		{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "main"},
		{ID: "%2", Label: "pane-2", WindowIdx: 0, Session: "main"},
	}

	state := NewTreeState(true)
	items := state.BuildTreeItems(sessions, windows, panes)

	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	if items[2].ID != "tree:p:main:0:%1" {
		t.Errorf("[2] expected tree:p:main:0:%%1, got %s", items[2].ID)
	}
	if items[3].ID != "tree:p:main:0:%2" {
		t.Errorf("[3] expected tree:p:main:0:%%2, got %s", items[3].ID)
	}
}

func TestFilterItemsPreservesAncestors(t *testing.T) {
	sessions := []SessionEntry{
		{Name: "main", Windows: 2},
		{Name: "work", Windows: 1},
	}
	windows := []WindowEntry{
		{ID: "@1", Label: "bash", Session: "main", Index: 0},
		{ID: "@2", Label: "vim", Session: "main", Index: 1},
		{ID: "@3", Label: "htop", Session: "work", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "main"},
		{ID: "%2", Label: "vim-pane", WindowIdx: 1, Session: "main"},
		{ID: "%3", Label: "htop-pane", WindowIdx: 0, Session: "work"},
	}

	state := NewTreeState(false)
	items := state.FilterTreeItems(sessions, windows, panes, "vim")

	if len(items) < 2 {
		var ids []string
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("expected at least 2 items, got %d: %v", len(items), ids)
	}

	found := false
	for _, item := range items {
		if item.ID == "tree:s:main" {
			found = true
		}
	}
	if !found {
		var ids []string
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		t.Errorf("expected ancestor session main to be preserved, got %v", ids)
	}
}

func TestFilterItemsNoMatchReturnsEmpty(t *testing.T) {
	sessions := []SessionEntry{{Name: "main", Windows: 1}}
	windows := []WindowEntry{{ID: "@1", Label: "bash", Session: "main", Index: 0}}
	panes := []PaneEntry{{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "main"}}

	state := NewTreeState(false)
	items := state.FilterTreeItems(sessions, windows, panes, "zzzznotfound")
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestFilterItemsEmptyQueryReturnsNormal(t *testing.T) {
	sessions := []SessionEntry{{Name: "main", Windows: 1}}
	windows := []WindowEntry{{ID: "@1", Label: "bash", Session: "main", Index: 0}}
	panes := []PaneEntry{}

	state := NewTreeState(false)
	items := state.FilterTreeItems(sessions, windows, panes, "")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestTreeToggle(t *testing.T) {
	state := NewTreeState(false)
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

func TestTreeItemKind(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"tree:s:main", "session"},
		{"tree:w:main:0", "window"},
		{"tree:p:main:0:%1", "pane"},
		{"other", ""},
	}
	for _, tt := range tests {
		if got := TreeItemKind(tt.id); got != tt.want {
			t.Errorf("TreeItemKind(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestFilterItemsMultiWordCrossHierarchy(t *testing.T) {
	sessions := []SessionEntry{
		{Name: "staging", Windows: 1},
		{Name: "production", Windows: 1},
	}
	windows := []WindowEntry{
		{ID: "@1", Label: "cron", Session: "staging", Index: 0},
		{ID: "@2", Label: "nginx", Session: "production", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "staging"},
		{ID: "%2", Label: "pane-2", WindowIdx: 0, Session: "production"},
	}

	state := NewTreeState(false)

	// "cron staging" — words match across session+window hierarchy.
	items := state.FilterTreeItems(sessions, windows, panes, "cron staging")
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) == 0 {
		t.Fatal("expected results for 'cron staging', got none")
	}
	foundSession := false
	foundWindow := false
	for _, it := range items {
		if it.ID == "tree:s:staging" {
			foundSession = true
		}
		if it.ID == "tree:w:staging:0" {
			foundWindow = true
		}
	}
	if !foundSession {
		t.Errorf("expected session 'staging' in results, got %v", ids)
	}
	if !foundWindow {
		t.Errorf("expected window 'cron' in results, got %v", ids)
	}

	// production session should NOT appear — "cron" doesn't match it.
	for _, it := range items {
		if it.ID == "tree:s:production" {
			t.Errorf("production should not match 'cron staging', got %v", ids)
		}
	}
}

func TestFilterItemsMultiWordReversedOrder(t *testing.T) {
	sessions := []SessionEntry{{Name: "staging", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "cron", Session: "staging", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "staging"},
	}

	state := NewTreeState(false)

	// "staging cron" — reversed word order should also match.
	items := state.FilterTreeItems(sessions, windows, panes, "staging cron")
	if len(items) == 0 {
		t.Fatal("expected results for 'staging cron', got none")
	}
	foundWindow := false
	for _, it := range items {
		if it.ID == "tree:w:staging:0" {
			foundWindow = true
		}
	}
	if !foundWindow {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Errorf("expected window 'cron' in results, got %v", ids)
	}
}

func TestFilterItemsMultiWordNoMatch(t *testing.T) {
	sessions := []SessionEntry{{Name: "staging", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "cron", Session: "staging", Index: 0},
	}
	panes := []PaneEntry{}

	state := NewTreeState(false)

	// "cron zzznope" — one word doesn't match anything in the path.
	items := state.FilterTreeItems(sessions, windows, panes, "cron zzznope")
	if len(items) != 0 {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("expected 0 items for 'cron zzznope', got %d: %v", len(items), ids)
	}
}

func TestFilterItemsMultiWordPaneContext(t *testing.T) {
	sessions := []SessionEntry{{Name: "dev", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "editor", Session: "dev", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "vim-main", WindowIdx: 0, Session: "dev"},
		{ID: "%2", Label: "shell", WindowIdx: 0, Session: "dev"},
	}

	state := NewTreeState(false)

	// "vim dev" — "vim" matches pane label, "dev" matches session name.
	items := state.FilterTreeItems(sessions, windows, panes, "vim dev")
	if len(items) == 0 {
		t.Fatal("expected results for 'vim dev', got none")
	}
	foundPane := false
	for _, it := range items {
		if it.ID == "tree:p:dev:0:%1" {
			foundPane = true
		}
	}
	if !foundPane {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Errorf("expected pane 'vim-main' in results, got %v", ids)
	}

	// The "shell" pane should NOT appear — "vim" doesn't match it.
	for _, it := range items {
		if it.ID == "tree:p:dev:0:%2" {
			var ids []string
			for _, it := range items {
				ids = append(ids, it.ID)
			}
			t.Errorf("pane 'shell' should not match 'vim dev', got %v", ids)
		}
	}
}

func TestTreeIsExpandable(t *testing.T) {
	if !TreeIsExpandable("tree:s:main") {
		t.Error("session should be expandable")
	}
	if !TreeIsExpandable("tree:w:main:0") {
		t.Error("window should be expandable")
	}
	if TreeIsExpandable("tree:p:main:0:%1") {
		t.Error("pane should not be expandable")
	}
}
