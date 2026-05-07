package menu

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

func TestSessionMenuIncludesTree(t *testing.T) {
	items, err := loadSessionMenu(Context{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.ID == "tree" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected session menu to include 'tree' item")
	}
}

func TestSessionCommandForActionUsesRequest(t *testing.T) {
	cmd := SessionCommandForAction(SessionRequest{
		Action: "session:new",
		Context: Context{
			SocketPath: "sock",
			ClientID:   "client",
		},
		Value: "dev",
	})
	if cmd == nil {
		t.Fatal("expected command")
	}
}

func TestSessionRenameFormEnterReturnsCommand(t *testing.T) {
	form := NewSessionForm(SessionPrompt{
		Context: Context{SocketPath: "sock"},
		Action:  "session:rename",
		Target:  "main",
		Initial: "main",
	})
	form.input.SetValue("renamed")

	cmd, done, cancel := form.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cancel {
		t.Fatal("enter should not cancel")
	}
	if !done {
		t.Fatal("enter should submit")
	}
	if cmd == nil {
		t.Fatal("expected submit command from session rename form")
	}
}

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

func TestLoadSessionRestoreFromMenuShowsNameBeforeTypeAndColoredRows(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", dir)

	saves := []struct {
		name string
		kind resurrect.SaveKind
		ts   string
	}{
		{name: "manual-save", kind: resurrect.SaveKindManual, ts: "20260405T100000"},
		{name: "auto-2026-04-05T11-00-00", kind: resurrect.SaveKindAuto, ts: "20260405T110000"},
	}
	for _, save := range saves {
		path := dir + "/" + save.name + "_" + save.ts + ".json"
		sf := &resurrect.SaveFile{
			Version:   2,
			Timestamp: mustParseRFC3339(t, save.ts),
			Name:      save.name,
			Kind:      save.kind,
			Sessions:  []resurrect.Session{{Name: "main"}},
		}
		if err := resurrect.WriteSaveFile(path, sf); err != nil {
			t.Fatalf("WriteSaveFile(%s): %v", save.name, err)
		}
	}
	autoPath := filepath.Join(dir, saves[1].name+"_"+saves[1].ts+".json")
	autoInfo, err := os.Stat(autoPath)
	if err != nil {
		t.Fatalf("Stat(%s): %v", autoPath, err)
	}
	autoSize := humanizeSaveSize(autoInfo.Size())

	items, err := loadResurrectRestoreFromMenu(Context{})
	if err != nil {
		t.Fatalf("loadResurrectRestoreFromMenu: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected header + 2 items, got %d", len(items))
	}
	if !items[0].Header || !strings.Contains(items[0].Label, "name") || !strings.Contains(items[0].Label, "type") || !strings.Contains(items[0].Label, "size") {
		t.Fatalf("expected restore header with name and type columns, got %#v", items[0])
	}
	if nameIdx := indexOf(items[0].Label, "name"); nameIdx < 0 {
		t.Fatalf("expected name column in header, got %#v", items[0])
	} else if typeIdx := indexOf(items[0].Label, "type"); typeIdx < 0 {
		t.Fatalf("expected type column in header, got %#v", items[0])
	} else if nameIdx >= typeIdx {
		t.Fatalf("expected name column before type column, got %q", items[0].Label)
	}

	var sawManual, sawAuto bool
	for _, item := range items[1:] {
		if ansi.Strip(item.Label) == item.Label {
			if item.StyledLabel == "" {
				t.Fatalf("expected styled restore row for %q", item.Label)
			}
		}
		stripped := ansi.Strip(item.StyledLabel)
		switch {
		case strings.Contains(stripped, "manual"):
			sawManual = true
			if !strings.Contains(item.StyledLabel, "[38;5;33m") {
				t.Fatalf("expected manual row colour33 styling, got %q", item.StyledLabel)
			}
		case strings.Contains(stripped, "auto"):
			sawAuto = true
			if !strings.Contains(item.StyledLabel, "[38;5;93m") {
				t.Fatalf("expected auto row colour93 styling, got %q", item.StyledLabel)
			}
			if strings.Contains(stripped, saves[1].name) {
				t.Fatalf("expected auto save row to collapse redundant timestamped name, got %q", stripped)
			}
			if !strings.Contains(stripped, autoSize) {
				t.Fatalf("expected auto row to show size %q, got %q", autoSize, stripped)
			}
		}
	}
	if !sawManual || !sawAuto {
		t.Fatalf("expected both manual and auto rows, got %#v", items)
	}
}

func TestFormatRestoreRowsLeftAlignsHeader(t *testing.T) {
	header := []string{"name", "type", "age", "date", "time", "size", "info"}
	rows := [][]string{
		{"snapshot", "manual", "1m", "2026-04-08", "09:30", "12 KB", " 1s  1w  1p"},
	}
	alignments := []table.Alignment{
		table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignRight, table.AlignLeft,
	}

	got := formatRestoreRows(header, rows, alignments)
	wantHeader := formatRestoreRow(header, restoreColumnWidths(append([][]string{header}, rows...)), nil)
	if got[0] != wantHeader {
		t.Fatalf("expected left-aligned header %q, got %q", wantHeader, got[0])
	}
}

func TestHumanizeSaveSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1024, "1 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1 MB"},
		{5*1024*1024 + 512*1024, "5.5 MB"},
	}
	for _, tt := range tests {
		if got := humanizeSaveSize(tt.size); got != tt.want {
			t.Fatalf("humanizeSaveSize(%d) = %q, want %q", tt.size, got, tt.want)
		}
	}
}

func mustParseRFC3339(t *testing.T, basic string) (outTime time.Time) {
	t.Helper()
	outTime, err := time.Parse("20060102T150405", basic)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", basic, err)
	}
	return outTime
}
