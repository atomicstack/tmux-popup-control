package menu

import (
	"strings"
	"testing"
)

func TestLoadPluginsMenu_NoSocket(t *testing.T) {
	// Point plugin dir to an empty temp dir so we don't pick up real plugins.
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", t.TempDir())

	// Without a socket path, no declared plugins are fetched.
	// Only action items should appear.
	items, err := loadPluginsMenu(Context{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"install", "update", "uninstall", "tidy"}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, item := range items {
		if item.ID != want[i] {
			t.Errorf("items[%d].ID = %q, want %q", i, item.ID, want[i])
		}
	}
}

func TestLoadPluginsMenu_ActionItemsAlwaysPresent(t *testing.T) {
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", t.TempDir())

	items, err := loadPluginsMenu(Context{})
	if err != nil {
		t.Fatal(err)
	}
	actionIDs := make(map[string]bool)
	for _, item := range items {
		if !strings.HasPrefix(item.ID, "__") {
			actionIDs[item.ID] = true
		}
	}
	for _, want := range []string{"install", "update", "uninstall", "tidy"} {
		if !actionIDs[want] {
			t.Errorf("missing action item %q", want)
		}
	}
}

func TestParseMultiSelectIDs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"foo", []string{"foo"}},
		{"foo\nbar\nbaz", []string{"foo", "bar", "baz"}},
		{"foo\n\nbar\n", []string{"foo", "bar"}},
	}
	for _, tt := range tests {
		got := parseMultiSelectIDs(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseMultiSelectIDs(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseMultiSelectIDs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
