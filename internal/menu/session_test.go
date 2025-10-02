package menu

import (
	"fmt"
	"strings"
	"testing"
)

func TestSessionRenameItemsFormatsTable(t *testing.T) {
	entries := []SessionEntry{
		{Name: "alpha", Windows: 3, Attached: true, Clients: []string{"c1"}},
		{Name: "beta", Windows: 12, Attached: false},
		{Name: "gamma", Windows: 7, Attached: true, Clients: []string{"c1", "c2"}, Current: true},
	}
	items := SessionRenameItems(entries)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	ordered := orderSessionsForRename(entries)
	format := sessionTableFormatForEntries(ordered)
	for i, entry := range ordered {
		name := entry.Name
		windows := fmt.Sprintf("%d windows", entry.Windows)
		status := sessionStatus(entry)
		current := ""
		if entry.Current {
			current = "current"
		}
		expected := fmt.Sprintf(format, name, windows, status, current)
		if items[i].ID != entry.Name {
			t.Fatalf("unexpected id at %d: got %s want %s", i, items[i].ID, entry.Name)
		}
		if items[i].Label != expected {
			t.Fatalf("unexpected label at %d: got %q want %q", i, items[i].Label, expected)
		}
	}

	last := strings.TrimRight(items[len(items)-1].Label, " ")
	if !strings.HasSuffix(last, "current") {
		t.Fatalf("expected trailing current marker in %q", items[len(items)-1].Label)
	}
	if !containsSubstring(items[len(items)-1].Label, "attached (2)") {
		t.Fatalf("expected attached client count in %q", items[len(items)-1].Label)
	}
	for _, item := range items {
		if strings.Contains(item.Label, "detached") {
			t.Fatalf("detached status should be empty, got %q", item.Label)
		}
	}
}

func TestSessionSwitchMenuItemsFormatsTable(t *testing.T) {
	ctx := Context{
		IncludeCurrent: true,
		Sessions: []SessionEntry{
			{Name: "one", Windows: 2, Attached: true, Clients: []string{"c1"}},
			{Name: "two", Windows: 8, Attached: false},
			{Name: "three", Windows: 5, Attached: true, Clients: []string{"c1", "c2"}, Current: true},
		},
	}
	items := SessionSwitchMenuItems(ctx)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	format := sessionTableFormatForEntries(ctx.Sessions)
	for i, entry := range ctx.Sessions {
		name := entry.Name
		windows := fmt.Sprintf("%d windows", entry.Windows)
		status := sessionStatus(entry)
		current := ""
		if entry.Current {
			current = "current"
		}
		expected := fmt.Sprintf(format, name, windows, status, current)
		if items[i].ID != entry.Name {
			t.Fatalf("unexpected id at %d: got %s want %s", i, items[i].ID, entry.Name)
		}
		if items[i].Label != expected {
			t.Fatalf("unexpected label at %d: got %q want %q", i, items[i].Label, expected)
		}
	}
	last := strings.TrimRight(items[len(items)-1].Label, " ")
	if !strings.HasSuffix(last, "current") {
		t.Fatalf("expected trailing current marker in %q", items[len(items)-1].Label)
	}
	for _, item := range items {
		if strings.Contains(item.Label, "detached") {
			t.Fatalf("detached status should be empty, got %q", item.Label)
		}
	}
}

func orderSessionsForRename(entries []SessionEntry) []SessionEntry {
	ordered := make([]SessionEntry, 0, len(entries))
	var current *SessionEntry
	for _, entry := range entries {
		if entry.Current {
			copy := entry
			current = &copy
			continue
		}
		ordered = append(ordered, entry)
	}
	if current != nil {
		ordered = append(ordered, *current)
	}
	return ordered
}

func sessionTableFormatForEntries(entries []SessionEntry) string {
	nameWidth := 0
	windowWidth := 0
	statusWidth := 0
	currentWidth := len("current")
	for _, entry := range entries {
		name := entry.Name
		if len(name) > nameWidth {
			nameWidth = len(name)
		}
		windows := fmt.Sprintf("%d windows", entry.Windows)
		if len(windows) > windowWidth {
			windowWidth = len(windows)
		}
		status := sessionStatus(entry)
		if len(status) > statusWidth {
			statusWidth = len(status)
		}
	}
	return fmt.Sprintf("%%-%ds  %%%ds  %%-%ds  %%-%ds", nameWidth, windowWidth, statusWidth, currentWidth)
}

func containsSubstring(s, substr string) bool {
	return indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
