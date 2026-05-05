package cmdparse

import (
	"os/exec"
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

// TestParseAllCommands feeds every synopsis line from `tmux list-commands`
// through the parser and asserts:
//   - no line errors out
//   - BuildRegistry produces a registry with the commands the rest of the
//     codebase relies on for completion, including alias resolution
//   - those known commands have a sensible shape (correct alias, expected
//     flag/positional presence)
//
// The test deliberately does not pin the full command catalogue to a golden
// file: the catalogue grows with each tmux release and the codebase only
// consumes a stable subset.
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
	if len(lines) == 0 {
		t.Fatal("tmux list-commands produced no output")
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, parseErr := ParseSynopsis(line); parseErr != nil {
			t.Errorf("failed to parse %q: %v", line, parseErr)
		}
	}

	reg := BuildRegistry(lines)
	if len(reg) == 0 {
		t.Fatal("BuildRegistry returned empty map")
	}

	// Spot-check commands the UI uses for completion. These have stable
	// shapes across tmux releases — flag-letter additions are tolerated, but
	// the named flag/positional must be present (and an alias when expected).
	type expectation struct {
		name        string
		alias       string
		argFlags    []rune
		positionals []string
	}
	expectations := []expectation{
		{name: "attach-session", alias: "attach", argFlags: []rune{'c', 'f', 't'}},
		{name: "kill-session", argFlags: []rune{'t'}},
		{name: "rename-session", alias: "rename", positionals: []string{"new-name"}},
		{name: "new-window", alias: "neww"},
		{name: "kill-window", alias: "killw", argFlags: []rune{'t'}},
		{name: "send-keys", alias: "send", argFlags: []rune{'t'}, positionals: []string{"key"}},
		{name: "find-window", alias: "findw", positionals: []string{"match-string"}},
		{name: "split-window", alias: "splitw"},
		{name: "select-pane", alias: "selectp"},
		{name: "switch-client", alias: "switchc", argFlags: []rune{'t'}},
		{name: "list-commands", alias: "lscm"},
	}

	for _, want := range expectations {
		schema, ok := reg[want.name]
		if !ok {
			t.Errorf("registry missing %q", want.name)
			continue
		}
		if want.alias != "" {
			if schema.Alias != want.alias {
				t.Errorf("%q: alias = %q, want %q", want.name, schema.Alias, want.alias)
			}
			if reg[want.alias] != schema {
				t.Errorf("%q: alias %q does not resolve to same schema", want.name, want.alias)
			}
		}
		for _, flag := range want.argFlags {
			if !SchemaHasArgFlag(schema, flag) {
				t.Errorf("%q: missing arg-flag -%c", want.name, flag)
			}
		}
		for _, posName := range want.positionals {
			found := false
			for _, p := range schema.Positionals {
				if p.Name == posName {
					found = true
					break
				}
			}
			if !found {
				names := make([]string, 0, len(schema.Positionals))
				for _, p := range schema.Positionals {
					names = append(names, p.Name)
				}
				t.Errorf("%q: missing positional %q (have %v)", want.name, posName, names)
			}
		}
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

