package cmdparse

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestParseSimpleCommand(t *testing.T) {
	s, err := ParseSynopsis("kill-server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "kill-server" {
		t.Errorf("name = %q, want %q", s.Name, "kill-server")
	}
	if s.Alias != "" {
		t.Errorf("alias = %q, want empty", s.Alias)
	}
	if len(s.BoolFlags) != 0 {
		t.Errorf("bool flags = %v, want empty", s.BoolFlags)
	}
	if len(s.ArgFlags) != 0 {
		t.Errorf("arg flags = %v, want empty", s.ArgFlags)
	}
	if len(s.Positionals) != 0 {
		t.Errorf("positionals = %v, want empty", s.Positionals)
	}
}

func TestParseCommandWithAlias(t *testing.T) {
	s, err := ParseSynopsis("attach-session (attach) [-dErx] [-c working-directory] [-f flags] [-t target-session]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "attach-session" {
		t.Errorf("name = %q, want %q", s.Name, "attach-session")
	}
	if s.Alias != "attach" {
		t.Errorf("alias = %q, want %q", s.Alias, "attach")
	}
	wantBool := []rune{'d', 'E', 'r', 'x'}
	if !runesEqual(s.BoolFlags, wantBool) {
		t.Errorf("bool flags = %v, want %v", string(s.BoolFlags), string(wantBool))
	}
	wantArgs := []ArgFlagDef{
		{'c', "working-directory"},
		{'f', "flags"},
		{'t', "target-session"},
	}
	if !argFlagsEqual(s.ArgFlags, wantArgs) {
		t.Errorf("arg flags = %v, want %v", s.ArgFlags, wantArgs)
	}
	if len(s.Positionals) != 0 {
		t.Errorf("positionals = %v, want empty", s.Positionals)
	}
}

func TestParseBoolOnlyFlags(t *testing.T) {
	s, err := ParseSynopsis("kill-session [-aC] [-t target-session]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "kill-session" {
		t.Errorf("name = %q, want %q", s.Name, "kill-session")
	}
	wantBool := []rune{'a', 'C'}
	if !runesEqual(s.BoolFlags, wantBool) {
		t.Errorf("bool flags = %v, want %v", string(s.BoolFlags), string(wantBool))
	}
	wantArgs := []ArgFlagDef{{'t', "target-session"}}
	if !argFlagsEqual(s.ArgFlags, wantArgs) {
		t.Errorf("arg flags = %v, want %v", s.ArgFlags, wantArgs)
	}
}

func TestParsePositionalArgs(t *testing.T) {
	s, err := ParseSynopsis("find-window (findw) [-CiNrTZ] [-t target-pane] match-string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "find-window" {
		t.Errorf("name = %q, want %q", s.Name, "find-window")
	}
	if s.Alias != "findw" {
		t.Errorf("alias = %q, want %q", s.Alias, "findw")
	}
	if len(s.Positionals) != 1 {
		t.Fatalf("positionals count = %d, want 1", len(s.Positionals))
	}
	p := s.Positionals[0]
	if p.Name != "match-string" || !p.Required || p.Variadic {
		t.Errorf("positional = %+v, want {Name:match-string Required:true Variadic:false}", p)
	}
}

func TestParseOptionalPositional(t *testing.T) {
	s, err := ParseSynopsis("choose-buffer [-NrZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Positionals) != 1 {
		t.Fatalf("positionals count = %d, want 1", len(s.Positionals))
	}
	p := s.Positionals[0]
	if p.Name != "template" || p.Required || p.Variadic {
		t.Errorf("positional = %+v, want {Name:template Required:false Variadic:false}", p)
	}
}

func TestParseVariadicPositional(t *testing.T) {
	s, err := ParseSynopsis("send-keys (send) [-FHlMNRX] [-c target-client] [-t target-pane] [key ...]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Positionals) != 1 {
		t.Fatalf("positionals count = %d, want 1", len(s.Positionals))
	}
	p := s.Positionals[0]
	if p.Name != "key" || p.Required || !p.Variadic {
		t.Errorf("positional = %+v, want {Name:key Required:false Variadic:true}", p)
	}
}

func TestParseBindKey(t *testing.T) {
	s, err := ParseSynopsis("bind-key (bind) [-nr] [-T key-table] [-N note] key [command [argument ...]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "bind-key" {
		t.Errorf("name = %q, want %q", s.Name, "bind-key")
	}
	if s.Alias != "bind" {
		t.Errorf("alias = %q, want %q", s.Alias, "bind")
	}
	wantBool := []rune{'n', 'r'}
	if !runesEqual(s.BoolFlags, wantBool) {
		t.Errorf("bool flags = %v, want %v", string(s.BoolFlags), string(wantBool))
	}
	wantArgs := []ArgFlagDef{
		{'T', "key-table"},
		{'N', "note"},
	}
	if !argFlagsEqual(s.ArgFlags, wantArgs) {
		t.Errorf("arg flags = %v, want %v", s.ArgFlags, wantArgs)
	}
	// bind-key has: key (required), command (optional), argument ... (optional variadic)
	wantPos := []PositionalDef{
		{Name: "key", Required: true, Variadic: false},
		{Name: "command", Required: false, Variadic: false},
		{Name: "argument", Required: false, Variadic: true},
	}
	if len(s.Positionals) != len(wantPos) {
		t.Fatalf("positionals = %+v, want %+v", s.Positionals, wantPos)
	}
	for i, want := range wantPos {
		got := s.Positionals[i]
		if got != want {
			t.Errorf("positional[%d] = %+v, want %+v", i, got, want)
		}
	}
}

func TestParseAllCommands(t *testing.T) {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux not available")
	}

	out, err := exec.Command(tmuxPath, "list-commands").Output()
	if err != nil {
		t.Fatalf("tmux list-commands failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var buf strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		s, parseErr := ParseSynopsis(line)
		if parseErr != nil {
			t.Errorf("failed to parse %q: %v", line, parseErr)
			continue
		}
		fmt.Fprintf(&buf, "command: %s\n", s.Name)
		if s.Alias != "" {
			fmt.Fprintf(&buf, "  alias: %s\n", s.Alias)
		}
		if len(s.BoolFlags) > 0 {
			fmt.Fprintf(&buf, "  bool-flags: %s\n", string(s.BoolFlags))
		}
		for _, af := range s.ArgFlags {
			fmt.Fprintf(&buf, "  arg-flag: -%c %s\n", af.Short, af.ArgType)
		}
		for _, p := range s.Positionals {
			marker := "required"
			if !p.Required {
				marker = "optional"
			}
			if p.Variadic {
				marker += ",variadic"
			}
			fmt.Fprintf(&buf, "  positional: %s (%s)\n", p.Name, marker)
		}
		buf.WriteString("\n")
	}

	golden := filepath.Join("testdata", "golden_schemas.txt")
	got := buf.String()

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("failed to write golden file: %v", err)
		}
		t.Logf("updated golden file %s", golden)
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("failed to read golden file (run with UPDATE_GOLDEN=1 to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, string(want))
	}

	// also test BuildRegistry
	reg := BuildRegistry(lines)
	if len(reg) == 0 {
		t.Fatal("BuildRegistry returned empty map")
	}
	// spot-check: attach-session should be reachable by both name and alias
	if _, ok := reg["attach-session"]; !ok {
		t.Error("registry missing attach-session")
	}
	if _, ok := reg["attach"]; !ok {
		t.Error("registry missing attach alias")
	}
	if reg["attach-session"] != reg["attach"] {
		t.Error("attach-session and attach should point to same schema")
	}
}

// helpers

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func argFlagsEqual(a, b []ArgFlagDef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sortedKeys returns sorted keys from a map for deterministic comparison.
var _ = sort.Strings
