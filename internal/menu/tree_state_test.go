package menu

import "testing"

func sampleTreeItemsInput() TreeItemsInput {
	return TreeItemsInput{
		Sessions: []SessionEntry{
			{Name: "main", Windows: 2, Current: true},
			{Name: "work", Windows: 1},
		},
		Windows: []WindowEntry{
			{ID: "@1", Label: "bash", Name: "bash", Session: "main", Index: 0},
			{ID: "@2", Label: "vim", Name: "vim", Session: "main", Index: 1},
			{ID: "@3", Label: "htop", Name: "htop", Session: "work", Index: 0},
		},
		Panes: []PaneEntry{
			{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "main", Window: "bash", Command: "bash"},
			{ID: "%2", Label: "vim-pane", WindowIdx: 1, Session: "main", Window: "vim", Command: "vim"},
			{ID: "%3", Label: "htop-pane", WindowIdx: 0, Session: "work", Window: "htop", Command: "htop"},
		},
	}
}

func TestBuildItemsCollapsed(t *testing.T) {
	state := NewTreeState(false)
	items := state.BuildTreeItems(sampleTreeItemsInput())

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
	items := state.BuildTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes})

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
	items := state.BuildTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes})

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
	state := NewTreeState(false)
	items := state.FilterTreeItems(sampleTreeItemsInput(), "vim")

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
	windows := []WindowEntry{{ID: "@1", Label: "bash", Name: "bash", Session: "main", Index: 0}}
	panes := []PaneEntry{{ID: "%1", Label: "pane-1", WindowIdx: 0, Session: "main", Window: "bash", Command: "bash"}}

	state := NewTreeState(false)
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "zzzznotfound")
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestFilterItemsEmptyQueryReturnsNormal(t *testing.T) {
	sessions := []SessionEntry{{Name: "main", Windows: 1}}
	windows := []WindowEntry{{ID: "@1", Label: "bash", Session: "main", Index: 0}}
	panes := []PaneEntry{}

	state := NewTreeState(false)
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "")
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

func TestFilterItemsSessionMatchDoesNotIncludeChildren(t *testing.T) {
	sessions := []SessionEntry{
		{Name: "staging", Windows: 2},
		{Name: "production", Windows: 1},
	}
	// Labels include session:index prefix like real tmux data.
	windows := []WindowEntry{
		{ID: "@1", Label: "staging:0: cron", Name: "cron", Session: "staging", Index: 0},
		{ID: "@2", Label: "staging:1: nginx", Name: "nginx", Session: "staging", Index: 1},
		{ID: "@3", Label: "production:0: web", Name: "web", Session: "production", Index: 0},
	}
	panes := []PaneEntry{}

	state := NewTreeState(false)
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "staging")

	// Only the session should appear — window tree labels (with the
	// session prefix stripped) don't contain "staging".
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (session only), got %d: %v", len(items), ids)
	}
	if items[0].ID != "tree:s:staging" {
		t.Errorf("expected tree:s:staging, got %s", items[0].ID)
	}
}

func TestFilterItemsChildMatchShowsAncestorOnly(t *testing.T) {
	sessions := []SessionEntry{
		{Name: "dev", Windows: 2},
		{Name: "work", Windows: 1},
	}
	windows := []WindowEntry{
		{ID: "@1", Label: "dev:0: vim", Name: "vim", Session: "dev", Index: 0},
		{ID: "@2", Label: "dev:1: bash", Name: "bash", Session: "dev", Index: 1},
		{ID: "@3", Label: "work:0: htop", Name: "htop", Session: "work", Index: 0},
	}
	panes := []PaneEntry{}

	state := NewTreeState(false)
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "vim")

	// "vim" matches window dev:0 — session "dev" shown as ancestor,
	// but window "bash" (dev:1) should NOT appear.
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (session + matching window), got %d: %v", len(items), ids)
	}
	if items[0].ID != "tree:s:dev" {
		t.Errorf("[0] expected tree:s:dev, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:dev:0" {
		t.Errorf("[1] expected tree:w:dev:0, got %s", items[1].ID)
	}
}

func TestFilterItemsPaneMatchShowsAncestorsOnly(t *testing.T) {
	sessions := []SessionEntry{{Name: "dev", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "dev:0: editor", Name: "editor", Session: "dev", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "dev:0.0: vim-main", WindowIdx: 0, Session: "dev", Index: 0, Window: "editor", Command: "vim"},
		{ID: "%2", Label: "dev:0.1: shell", WindowIdx: 0, Session: "dev", Index: 1, Window: "editor", Command: "bash"},
	}

	state := NewTreeState(false)
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "vim")

	// "vim" matches pane "vim-main" — ancestors shown, but pane "shell" excluded.
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items (session + window + pane), got %d: %v", len(items), ids)
	}
	if items[0].ID != "tree:s:dev" {
		t.Errorf("[0] expected tree:s:dev, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:dev:0" {
		t.Errorf("[1] expected tree:w:dev:0, got %s", items[1].ID)
	}
	if items[2].ID != "tree:p:dev:0:%1" {
		t.Errorf("[2] expected tree:p:dev:0:%%1, got %s", items[2].ID)
	}
}

func TestFilterItemsWindowMatchExcludesNonMatchingPanes(t *testing.T) {
	sessions := []SessionEntry{{Name: "dev", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "dev:0: vim", Name: "vim", Session: "dev", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "dev:0.0: pane-1", WindowIdx: 0, Session: "dev", Index: 0, Window: "vim", Command: "bash"},
		{ID: "%2", Label: "dev:0.1: pane-2", WindowIdx: 0, Session: "dev", Index: 1, Window: "vim", Command: "zsh"},
	}

	state := NewTreeState(false)
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "vim")

	// "vim" matches window "vim" — panes "pane-1" and "pane-2" do NOT
	// match "vim", so they must NOT appear.
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (session + window), got %d: %v", len(items), ids)
	}
	if items[0].ID != "tree:s:dev" {
		t.Errorf("[0] expected tree:s:dev, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:dev:0" {
		t.Errorf("[1] expected tree:w:dev:0, got %s", items[1].ID)
	}
}

func TestFilterItemsSessionMatchExcludesNonMatchingDescendants(t *testing.T) {
	sessions := []SessionEntry{{Name: "staging", Windows: 2}}
	windows := []WindowEntry{
		{ID: "@1", Label: "staging:0: cron", Name: "cron", Session: "staging", Index: 0},
		{ID: "@2", Label: "staging:1: nginx", Name: "nginx", Session: "staging", Index: 1},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "staging:0.0: worker", WindowIdx: 0, Session: "staging", Index: 0, Window: "cron", Command: "worker"},
		{ID: "%2", Label: "staging:1.0: logs", WindowIdx: 1, Session: "staging", Index: 0, Window: "nginx", Command: "tail"},
	}

	state := NewTreeState(false)
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "staging")

	// "staging" matches the session only — no window or pane contains
	// "staging", so none of the descendants should appear.
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (session only), got %d: %v", len(items), ids)
	}
	if items[0].ID != "tree:s:staging" {
		t.Errorf("expected tree:s:staging, got %s", items[0].ID)
	}
}

func TestFilterItemsMultiWordPerItem(t *testing.T) {
	sessions := []SessionEntry{
		{Name: "staging", Windows: 1},
		{Name: "production", Windows: 1},
	}
	windows := []WindowEntry{
		{ID: "@1", Label: "cron", Name: "cron", Session: "staging", Index: 0},
		{ID: "@2", Label: "nginx", Name: "nginx", Session: "production", Index: 0},
	}
	panes := []PaneEntry{}

	state := NewTreeState(false)

	// "cron staging" — each word must match within a single item's own metadata.
	// No single item contains both "cron" AND "staging", so no results.
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "cron staging")
	if len(items) != 0 {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("expected 0 items for 'cron staging' (no single item matches both words), got %d: %v", len(items), ids)
	}

	// "cron" alone matches the window, with session as ancestor.
	items = state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "cron")
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items for 'cron', got %d: %v", len(items), ids)
	}
	if items[0].ID != "tree:s:staging" {
		t.Errorf("[0] expected tree:s:staging, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:staging:0" {
		t.Errorf("[1] expected tree:w:staging:0, got %s", items[1].ID)
	}
}

func TestFilterItemsMultiWordWithinSingleItem(t *testing.T) {
	sessions := []SessionEntry{{Name: "dev", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "cron staging-sync", Name: "cron staging-sync", Session: "dev", Index: 0},
	}
	panes := []PaneEntry{}

	state := NewTreeState(false)

	// Both words exist in the window's own label — should match.
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "cron staging")
	if len(items) != 2 {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("expected 2 items for 'cron staging', got %d: %v", len(items), ids)
	}
	if items[0].ID != "tree:s:dev" {
		t.Errorf("[0] expected tree:s:dev, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:dev:0" {
		t.Errorf("[1] expected tree:w:dev:0, got %s", items[1].ID)
	}
}

func TestFilterItemsMultiWordNoMatch(t *testing.T) {
	sessions := []SessionEntry{{Name: "staging", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "cron", Name: "cron", Session: "staging", Index: 0},
	}
	panes := []PaneEntry{}

	state := NewTreeState(false)

	// "cron zzznope" — one word doesn't match anything in the path.
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "cron zzznope")
	if len(items) != 0 {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("expected 0 items for 'cron zzznope', got %d: %v", len(items), ids)
	}
}

func TestFilterItemsPaneMatchedByOwnMetadataOnly(t *testing.T) {
	sessions := []SessionEntry{{Name: "dev", Windows: 1}}
	windows := []WindowEntry{
		{ID: "@1", Label: "editor", Name: "editor", Session: "dev", Index: 0},
	}
	panes := []PaneEntry{
		{ID: "%1", Label: "vim-main", WindowIdx: 0, Session: "dev", Window: "editor", Command: "vim"},
		{ID: "%2", Label: "shell", WindowIdx: 0, Session: "dev", Window: "editor", Command: "bash"},
	}

	state := NewTreeState(false)

	// "vim" matches pane "vim-main" on its own label — ancestors shown.
	items := state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "vim")
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items (session + window + pane), got %d: %v", len(items), ids)
	}
	if items[2].ID != "tree:p:dev:0:%1" {
		t.Errorf("[2] expected tree:p:dev:0:%%1, got %s", items[2].ID)
	}

	// "shell" pane should not appear — "vim" doesn't match it.
	for _, it := range items {
		if it.ID == "tree:p:dev:0:%2" {
			t.Errorf("pane 'shell' should not match 'vim', got %v", ids)
		}
	}

	// "vim dev" — no single item contains both words, so no results.
	items = state.FilterTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes}, "vim dev")
	if len(items) != 0 {
		ids = nil
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("expected 0 items for 'vim dev' (cross-hierarchy), got %d: %v", len(items), ids)
	}
}

func TestTreeWindowLabel(t *testing.T) {
	tests := []struct {
		name  string
		entry WindowEntry
		want  string
	}{
		{
			name:  "strips session prefix",
			entry: WindowEntry{Label: "main:0: bash", Session: "main", Index: 0},
			want:  "0: bash",
		},
		{
			name:  "different session and index",
			entry: WindowEntry{Label: "work:2: vim", Session: "work", Index: 2},
			want:  "2: vim",
		},
		{
			name:  "no prefix match returns label unchanged",
			entry: WindowEntry{Label: "bash", Session: "main", Index: 0},
			want:  "bash",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TreeWindowLabel(tt.entry); got != tt.want {
				t.Errorf("TreeWindowLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTreePaneLabel(t *testing.T) {
	tests := []struct {
		name  string
		entry PaneEntry
		want  string
	}{
		{
			name: "strips prefix and swaps blocks",
			entry: PaneEntry{
				Label:     "dev:0.1: [bash:~] vim  [120x40] [history 500/10000, 12345 bytes] [active]",
				Session:   "dev",
				WindowIdx: 0,
				Index:     1,
			},
			want: "1: vim [bash:~]  [120x40] [history 500/10000, 12345 bytes] [active]",
		},
		{
			name: "no prefix match still swaps blocks",
			entry: PaneEntry{
				Label:     "[zsh:/tmp] top  [80x24]",
				Session:   "main",
				WindowIdx: 0,
				Index:     0,
			},
			want: "0: top [zsh:/tmp]  [80x24]",
		},
		{
			name: "plain label without brackets",
			entry: PaneEntry{
				Label:     "pane-1",
				Session:   "main",
				WindowIdx: 0,
				Index:     0,
			},
			want: "0: pane-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TreePaneLabel(tt.entry); got != tt.want {
				t.Errorf("TreePaneLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSwapLeadingBracketBlock(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"[bash:~] vim  [80x24]", "vim [bash:~]  [80x24]"},
		{"[name:title] cmd", "cmd [name:title]"},
		{"no brackets here", "no brackets here"},
		{"[only bracket]", "[only bracket]"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := swapLeadingBracketBlock(tt.input); got != tt.want {
				t.Errorf("swapLeadingBracketBlock(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildTreeItemsUsesCompactLabels(t *testing.T) {
	sessions := []SessionEntry{{Name: "dev", Windows: 1}}
	windows := []WindowEntry{{
		ID: "@1", Label: "dev:0: bash", Session: "dev", Index: 0,
	}}
	panes := []PaneEntry{{
		ID: "%1", Label: "dev:0.0: [bash:~] zsh  [80x24]", Session: "dev", WindowIdx: 0, Index: 0,
	}}

	state := NewTreeState(true)
	items := state.BuildTreeItems(TreeItemsInput{Sessions: sessions, Windows: windows, Panes: panes})

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[1].Label != "0: bash" {
		t.Errorf("window label = %q, want %q", items[1].Label, "0: bash")
	}
	if items[2].Label != "0: zsh [bash:~]  [80x24]" {
		t.Errorf("pane label = %q, want %q", items[2].Label, "0: zsh [bash:~]  [80x24]")
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
