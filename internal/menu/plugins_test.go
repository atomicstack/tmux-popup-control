package menu

import (
	"testing"
)

func TestLoadPluginsMenu(t *testing.T) {
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
