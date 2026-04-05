package cmdparse

import (
	"sort"
	"testing"
)

type mockResolver struct {
	sessions []string
	windows  []string
	panes    []string
	clients  []string
	commands []string
	buffers  []string
}

func (m *mockResolver) Sessions() []string { return m.sessions }
func (m *mockResolver) Windows() []string  { return m.windows }
func (m *mockResolver) Panes() []string    { return m.panes }
func (m *mockResolver) Clients() []string  { return m.clients }
func (m *mockResolver) Commands() []string { return m.commands }
func (m *mockResolver) Buffers() []string  { return m.buffers }

func TestResolveTargetSession(t *testing.T) {
	r := NewStoreResolver(&mockResolver{
		sessions: []string{"main", "work", "scratch"},
	})
	got := r.Resolve("target-session")
	if len(got) != 3 {
		t.Fatalf("expected 3 sessions, got %d: %v", len(got), got)
	}
}

func TestResolveSessionName(t *testing.T) {
	r := NewStoreResolver(&mockResolver{
		sessions: []string{"main"},
	})
	got := r.Resolve("session-name")
	if len(got) != 1 || got[0] != "main" {
		t.Fatalf("expected [main], got %v", got)
	}
}

func TestResolveTargetWindow(t *testing.T) {
	r := NewStoreResolver(&mockResolver{
		windows: []string{"main:0", "main:1", "work:0"},
	})
	got := r.Resolve("target-window")
	if len(got) != 3 {
		t.Fatalf("expected 3 windows, got %d: %v", len(got), got)
	}
}

func TestResolveTargetPane(t *testing.T) {
	r := NewStoreResolver(&mockResolver{
		panes: []string{"%0", "%1", "%2"},
	})
	got := r.Resolve("target-pane")
	if len(got) != 3 {
		t.Fatalf("expected 3 panes, got %d: %v", len(got), got)
	}
}

func TestResolveTargetClient(t *testing.T) {
	r := NewStoreResolver(&mockResolver{
		clients: []string{"/dev/ttys000", "/dev/ttys001"},
	})
	got := r.Resolve("target-client")
	if len(got) != 2 {
		t.Fatalf("expected 2 clients, got %d: %v", len(got), got)
	}
}

func TestResolveCommand(t *testing.T) {
	r := NewStoreResolver(&mockResolver{
		commands: []string{"attach-session", "bind-key", "kill-server"},
	})
	got := r.Resolve("command")
	if len(got) != 3 {
		t.Fatalf("expected 3 commands, got %d: %v", len(got), got)
	}
}

func TestResolveKeyTable(t *testing.T) {
	r := NewStoreResolver(&mockResolver{})
	got := r.Resolve("key-table")
	sort.Strings(got)
	if len(got) != 4 {
		t.Fatalf("expected 4 key tables, got %d: %v", len(got), got)
	}
}

func TestResolveLayoutName(t *testing.T) {
	r := NewStoreResolver(&mockResolver{})
	got := r.Resolve("layout-name")
	if len(got) != 5 {
		t.Fatalf("expected 5 layouts, got %d: %v", len(got), got)
	}
}

func TestResolveUnknownType(t *testing.T) {
	r := NewStoreResolver(&mockResolver{})
	got := r.Resolve("format")
	if got != nil {
		t.Fatalf("expected nil for unknown type, got %v", got)
	}
}

func TestResolveFlagCandidates(t *testing.T) {
	schema := attachSchema()
	used := []rune{'d'}
	got := FlagCandidates(schema, used)
	if len(got) != 6 {
		t.Fatalf("expected 6 flag candidates, got %d: %v", len(got), got)
	}
	for _, fc := range got {
		if fc.Flag == 'd' {
			t.Fatal("flag 'd' should be excluded (already used)")
		}
	}
}

func TestFlagCandidatesPreserveSynopsisOrder(t *testing.T) {
	schema, err := ParseSynopsis("attach-session (attach) [-dErx] [-c working-directory] [-f flags] [-t target-session]")
	if err != nil {
		t.Fatalf("ParseSynopsis failed: %v", err)
	}

	got := FlagCandidates(schema, nil)
	want := []rune{'d', 'E', 'r', 'x', 'c', 'f', 't'}
	if len(got) != len(want) {
		t.Fatalf("expected %d candidates, got %d", len(want), len(got))
	}
	for i, flag := range want {
		if got[i].Flag != flag {
			t.Fatalf("candidate[%d] = %q, want %q", i, string(got[i].Flag), string(flag))
		}
	}
}

func TestFlagCandidatesKeepRepeatableFlagsAvailable(t *testing.T) {
	schema, err := ParseSynopsis("new-window (neww) [-abdkPS] [-c start-directory] [-e environment] [-F format] [-n window-name] [-t target-window] [shell-command [argument ...]]")
	if err != nil {
		t.Fatalf("ParseSynopsis failed: %v", err)
	}

	got := FlagCandidates(schema, []rune{'a', 'b', 'd', 'k', 'P', 'S', 'c', 'e', 'F', 'n', 't'})
	if len(got) != 1 {
		t.Fatalf("expected only repeatable -e to remain, got %v", got)
	}
	if got[0].Flag != 'e' {
		t.Fatalf("expected repeatable flag -e, got %q", string(got[0].Flag))
	}
}
