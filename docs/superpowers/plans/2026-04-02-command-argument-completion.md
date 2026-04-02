# Command Argument Tab Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add intelligent tab completion for tmux command arguments (flags, flag values, positional args) with a dropdown popup and contextual ghost hints.

**Architecture:** New `internal/cmdparse/` package handles synopsis parsing, input analysis, and value resolution — no UI dependencies. The UI gets a `completionState` struct managing the dropdown overlay and ghost hints, with key routing changes when the dropdown is visible. The existing command preload path builds a schema registry alongside the item cache.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, lithammer/fuzzysearch (existing deps). No new dependencies.

**Spec:** `docs/superpowers/specs/2026-04-02-command-argument-completion-design.md`

---

## File Structure

### New files

| File | Responsibility |
|---|---|
| `internal/cmdparse/schema.go` | Type definitions: `CommandSchema`, `ArgFlagDef`, `PositionalDef`, `CompletionContext`, `ContextKind` |
| `internal/cmdparse/parse.go` | Parse `list-commands` synopsis lines into `CommandSchema`; `BuildRegistry` to index by name+alias |
| `internal/cmdparse/parse_test.go` | Unit tests for parser: individual commands, edge cases, golden file for all ~90 commands |
| `internal/cmdparse/analyse.go` | `Analyse(schema, input)` → `CompletionContext` determining what to complete at cursor position |
| `internal/cmdparse/analyse_test.go` | Table-driven tests for all context kinds at various cursor positions |
| `internal/cmdparse/resolve.go` | `Resolver` interface + `StoreResolver` mapping arg types to live tmux data from state stores |
| `internal/cmdparse/resolve_test.go` | Tests with mock store data |
| `internal/cmdparse/testdata/golden_schemas.txt` | Golden file for parsed schemas |
| `internal/ui/completion.go` | `completionState` struct: dropdown state, filtering, selection, ghost hints, rendering |
| `internal/ui/completion_test.go` | Unit tests for dropdown state management and ghost hint logic |

### Modified files

| File | What changes |
|---|---|
| `internal/ui/model.go` | Add `completion *completionState` and `commandSchemas map[string]*cmdparse.CommandSchema` fields to `Model` |
| `internal/ui/commands.go` | Build schema registry in `handleCommandPreloadMsg` |
| `internal/ui/input.go` | Extend `autoCompleteGhost()` for argument hints; extend `handleTextInput()` to trigger completion analysis |
| `internal/ui/navigation.go` | Route keys through completion state when dropdown visible (arrows, tab, escape, enter) |
| `internal/ui/view.go` | Render dropdown overlay above prompt line in `viewVertical` and `viewSideBySide` |
| `internal/theme/theme.go` | Add `CompletionBorder`, `CompletionItem`, `CompletionSelected` styles |

---

## Task 1: Schema Types

**Files:**
- Create: `internal/cmdparse/schema.go`

- [ ] **Step 1: Create the cmdparse package with type definitions**

```go
// internal/cmdparse/schema.go
package cmdparse

// CommandSchema is the parsed representation of a tmux command synopsis.
type CommandSchema struct {
	Name        string
	Alias       string
	BoolFlags   []rune
	ArgFlags    []ArgFlagDef
	Positionals []PositionalDef
}

// ArgFlagDef is a flag that expects a typed argument value.
type ArgFlagDef struct {
	Short   rune
	ArgType string
}

// PositionalDef is a positional argument.
type PositionalDef struct {
	Name     string
	Required bool
	Variadic bool
}

// CompletionContext describes what kind of completion is available at the
// current cursor position in the command input.
type CompletionContext struct {
	Kind      ContextKind
	ArgType   string // for FlagValue/PositionalValue: the argument type name
	TypeLabel string // display label for ghost hint (e.g. "src-window")
	Prefix    string // text already typed for the current token
	FlagsUsed []rune // flags already present in the input
}

// ContextKind identifies the type of completion point.
type ContextKind int

const (
	ContextNone           ContextKind = iota
	ContextCommandName                // first token: completing a command name
	ContextFlagName                   // expecting a flag (e.g. after space following completed arg)
	ContextFlagValue                  // expecting a value for a flag (e.g. after "-t ")
	ContextPositionalValue            // expecting a positional argument value
)
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go build ./internal/cmdparse/`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add internal/cmdparse/schema.go
git commit -m "feat(cmdparse): add schema types for command argument completion"
```

---

## Task 2: Synopsis Parser

**Files:**
- Create: `internal/cmdparse/parse.go`
- Create: `internal/cmdparse/parse_test.go`
- Create: `internal/cmdparse/testdata/golden_schemas.txt`

- [ ] **Step 1: Write the failing test for basic parsing**

```go
// internal/cmdparse/parse_test.go
package cmdparse

import "testing"

func TestParseSimpleCommand(t *testing.T) {
	schema, err := ParseSynopsis("kill-server")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema.Name != "kill-server" {
		t.Fatalf("expected name %q, got %q", "kill-server", schema.Name)
	}
	if schema.Alias != "" {
		t.Fatalf("expected no alias, got %q", schema.Alias)
	}
	if len(schema.BoolFlags) != 0 {
		t.Fatalf("expected no bool flags, got %v", schema.BoolFlags)
	}
	if len(schema.ArgFlags) != 0 {
		t.Fatalf("expected no arg flags, got %v", schema.ArgFlags)
	}
	if len(schema.Positionals) != 0 {
		t.Fatalf("expected no positionals, got %v", schema.Positionals)
	}
}

func TestParseCommandWithAlias(t *testing.T) {
	schema, err := ParseSynopsis("attach-session (attach) [-dErx] [-c working-directory] [-f flags] [-t target-session]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if schema.Name != "attach-session" {
		t.Fatalf("expected name %q, got %q", "attach-session", schema.Name)
	}
	if schema.Alias != "attach" {
		t.Fatalf("expected alias %q, got %q", "attach", schema.Alias)
	}
	expectedBool := []rune{'d', 'E', 'r', 'x'}
	if len(schema.BoolFlags) != len(expectedBool) {
		t.Fatalf("expected %d bool flags, got %d: %v", len(expectedBool), len(schema.BoolFlags), schema.BoolFlags)
	}
	for i, r := range expectedBool {
		if schema.BoolFlags[i] != r {
			t.Fatalf("bool flag %d: expected %c, got %c", i, r, schema.BoolFlags[i])
		}
	}
	expectedArg := []ArgFlagDef{
		{Short: 'c', ArgType: "working-directory"},
		{Short: 'f', ArgType: "flags"},
		{Short: 't', ArgType: "target-session"},
	}
	if len(schema.ArgFlags) != len(expectedArg) {
		t.Fatalf("expected %d arg flags, got %d: %v", len(expectedArg), len(schema.ArgFlags), schema.ArgFlags)
	}
	for i, want := range expectedArg {
		got := schema.ArgFlags[i]
		if got.Short != want.Short || got.ArgType != want.ArgType {
			t.Fatalf("arg flag %d: expected %+v, got %+v", i, want, got)
		}
	}
}

func TestParseBoolOnlyFlags(t *testing.T) {
	schema, err := ParseSynopsis("kill-session [-aC] [-t target-session]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schema.BoolFlags) != 2 || schema.BoolFlags[0] != 'a' || schema.BoolFlags[1] != 'C' {
		t.Fatalf("expected bool flags [a C], got %v", schema.BoolFlags)
	}
	if len(schema.ArgFlags) != 1 || schema.ArgFlags[0].Short != 't' {
		t.Fatalf("expected 1 arg flag -t, got %v", schema.ArgFlags)
	}
}

func TestParsePositionalArgs(t *testing.T) {
	schema, err := ParseSynopsis("find-window (findw) [-CiNrTZ] [-t target-pane] match-string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schema.Positionals) != 1 {
		t.Fatalf("expected 1 positional, got %d", len(schema.Positionals))
	}
	if schema.Positionals[0].Name != "match-string" {
		t.Fatalf("expected positional %q, got %q", "match-string", schema.Positionals[0].Name)
	}
	if !schema.Positionals[0].Required {
		t.Fatal("expected positional to be required")
	}
	if schema.Positionals[0].Variadic {
		t.Fatal("expected positional to not be variadic")
	}
}

func TestParseOptionalPositional(t *testing.T) {
	schema, err := ParseSynopsis("choose-buffer [-NrZ] [-F format] [-f filter] [-K key-format] [-O sort-order] [-t target-pane] [template]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, p := range schema.Positionals {
		if p.Name == "template" {
			found = true
			if p.Required {
				t.Fatal("template should be optional")
			}
		}
	}
	if !found {
		t.Fatalf("expected positional 'template', got %v", schema.Positionals)
	}
}

func TestParseVariadicPositional(t *testing.T) {
	schema, err := ParseSynopsis("send-keys (send) [-FHlMNRX] [-c target-client] [-t target-pane] [key ...]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, p := range schema.Positionals {
		if p.Name == "key" {
			found = true
			if !p.Variadic {
				t.Fatal("key should be variadic")
			}
			if p.Required {
				t.Fatal("key should be optional")
			}
		}
	}
	if !found {
		t.Fatalf("expected positional 'key', got %v", schema.Positionals)
	}
}

func TestParseBindKey(t *testing.T) {
	schema, err := ParseSynopsis("bind-key (bind) [-nr] [-T key-table] [-N note] key [command [argument ...]]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(schema.Positionals) < 2 {
		t.Fatalf("expected at least 2 positionals, got %d: %v", len(schema.Positionals), schema.Positionals)
	}
	if schema.Positionals[0].Name != "key" || !schema.Positionals[0].Required {
		t.Fatalf("first positional should be required 'key', got %+v", schema.Positionals[0])
	}
	if schema.Positionals[1].Name != "command" || schema.Positionals[1].Required {
		t.Fatalf("second positional should be optional 'command', got %+v", schema.Positionals[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/cmdparse/ -run TestParse -v`
Expected: FAIL — `ParseSynopsis` not defined

- [ ] **Step 3: Implement the synopsis parser**

```go
// internal/cmdparse/parse.go
package cmdparse

import (
	"fmt"
	"strings"
)

// ParseSynopsis parses a single tmux list-commands output line into a
// CommandSchema. The expected format is:
//
//	command-name (alias) [-boolflags] [-f arg-type] ... [positional] ...
func ParseSynopsis(line string) (*CommandSchema, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty synopsis")
	}

	tokens := tokenize(line)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty synopsis after tokenize")
	}

	schema := &CommandSchema{}
	i := 0

	// first token is always the command name
	schema.Name = tokens[i]
	i++

	// optional alias in parentheses
	if i < len(tokens) && strings.HasPrefix(tokens[i], "(") && strings.HasSuffix(tokens[i], ")") {
		schema.Alias = tokens[i][1 : len(tokens[i])-1]
		i++
	}

	// parse remaining tokens: flag groups, individual flags, positionals
	for i < len(tokens) {
		tok := tokens[i]

		if isBracketedFlagGroup(tok) {
			// e.g. "[-dErx]" — clustered boolean flags
			inner := tok[2 : len(tok)-1] // strip "[- ... ]"
			for _, r := range inner {
				schema.BoolFlags = append(schema.BoolFlags, r)
			}
			i++
			continue
		}

		if isBracketedArgFlag(tok, tokens, i) {
			// e.g. "[-f" "arg-type]" — flag with typed argument (spans 2 tokens)
			flag := rune(tok[2]) // "[-f" → 'f'
			argType := strings.TrimSuffix(tokens[i+1], "]")
			schema.ArgFlags = append(schema.ArgFlags, ArgFlagDef{Short: flag, ArgType: argType})
			i += 2
			continue
		}

		// everything remaining is positional arguments
		positionals := parsePositionals(tokens[i:])
		schema.Positionals = append(schema.Positionals, positionals...)
		break
	}

	return schema, nil
}

// tokenize splits a synopsis line into whitespace-separated tokens, but
// keeps bracketed groups like "[-dErx]" together when they don't contain
// spaces. Brackets that span spaces are split normally since each word
// is its own token.
func tokenize(line string) []string {
	return strings.Fields(line)
}

// isBracketedFlagGroup checks for "[-abc]" patterns — bracketed boolean
// flag clusters. These are exactly one token like "[-dErx]" with no space
// inside.
func isBracketedFlagGroup(tok string) bool {
	if !strings.HasPrefix(tok, "[-") || !strings.HasSuffix(tok, "]") {
		return false
	}
	// must be exactly "[-" + one or more flag chars + "]"
	// if inner length is > 1 it's a cluster; if 1 it could be a single bool
	inner := tok[2 : len(tok)-1]
	if len(inner) == 0 {
		return false
	}
	// a bool group has no space (already tokenized) and no lowercase-word pattern
	// that looks like an arg type. single char like "[-a]" is bool too.
	// to distinguish "[-f" (start of arg flag) we check if tok ends with "]"
	// AND the inner part is all single characters (no hyphen in a-z pattern).
	for _, r := range inner {
		if r == '-' || r == ' ' {
			return false
		}
	}
	return true
}

// isBracketedArgFlag checks if tokens[i] looks like "[-f" and tokens[i+1]
// looks like "arg-type]" — a flag with a typed argument split across two
// tokens.
func isBracketedArgFlag(tok string, tokens []string, i int) bool {
	if !strings.HasPrefix(tok, "[-") {
		return false
	}
	if strings.HasSuffix(tok, "]") {
		return false // it's a complete bracket group, not a split arg flag
	}
	// "[-f" should be exactly 3 chars
	if len(tok) != 3 {
		return false
	}
	if i+1 >= len(tokens) {
		return false
	}
	return strings.HasSuffix(tokens[i+1], "]")
}

// parsePositionals processes the remaining tokens after all flags as
// positional arguments. Handles:
//   - "word"           → required positional
//   - "[word]"         → optional positional
//   - "[word ...]"     → optional variadic
//   - "word ..."       → required variadic (rare)
//   - nested "[command [argument ...]]" → flattened to individual positionals
func parsePositionals(tokens []string) []PositionalDef {
	var result []PositionalDef
	i := 0
	bracketDepth := 0

	for i < len(tokens) {
		tok := tokens[i]

		// track bracket depth for optionality
		opens := strings.Count(tok, "[")
		closes := strings.Count(tok, "]")
		bracketDepth += opens

		// strip all brackets from the token to get the bare name
		name := strings.ReplaceAll(tok, "[", "")
		name = strings.ReplaceAll(name, "]", "")

		// skip "..." tokens (they modify the previous positional)
		if name == "..." {
			if len(result) > 0 {
				result[len(result)-1].Variadic = true
			}
			bracketDepth -= closes
			i++
			continue
		}

		// skip empty names (can happen from "[]" or bracket-only tokens)
		if name == "" {
			bracketDepth -= closes
			i++
			continue
		}

		// check if next token is "...]" or "...]" or "..."
		variadic := false
		if strings.HasSuffix(name, "...") {
			name = strings.TrimSuffix(name, "...")
			variadic = true
		}
		if i+1 < len(tokens) {
			next := strings.ReplaceAll(tokens[i+1], "[", "")
			next = strings.ReplaceAll(next, "]", "")
			if next == "..." {
				variadic = true
			}
		}

		if name != "" {
			result = append(result, PositionalDef{
				Name:     name,
				Required: bracketDepth == 0 && opens == 0,
				Variadic: variadic,
			})
		}

		bracketDepth -= closes
		if bracketDepth < 0 {
			bracketDepth = 0
		}
		i++
	}

	return result
}

// BuildRegistry parses a list of menu items (from list-commands output) into
// a map of command name/alias → *CommandSchema. Each item's Label field is
// expected to be the full synopsis line.
func BuildRegistry(labels []string) map[string]*CommandSchema {
	reg := make(map[string]*CommandSchema, len(labels)*2)
	for _, label := range labels {
		schema, err := ParseSynopsis(label)
		if err != nil {
			continue
		}
		reg[schema.Name] = schema
		if schema.Alias != "" {
			reg[schema.Alias] = schema
		}
	}
	return reg
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/cmdparse/ -run TestParse -v`
Expected: all PASS

- [ ] **Step 5: Write golden file test**

Add to `parse_test.go`:

```go
import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

func init() {
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		t := true
		updateGolden = &t
	}
}

func TestParseAllCommands(t *testing.T) {
	// run tmux list-commands to get real output
	out, err := exec.Command("tmux", "list-commands").Output()
	if err != nil {
		t.Skip("tmux not available:", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var buf strings.Builder
	var failures []string

	for _, line := range lines {
		if line == "" {
			continue
		}
		schema, err := ParseSynopsis(line)
		if err != nil {
			failures = append(failures, fmt.Sprintf("FAIL: %s: %v", line, err))
			continue
		}
		fmt.Fprintf(&buf, "command: %s\n", schema.Name)
		if schema.Alias != "" {
			fmt.Fprintf(&buf, "  alias: %s\n", schema.Alias)
		}
		if len(schema.BoolFlags) > 0 {
			flags := make([]string, len(schema.BoolFlags))
			for i, f := range schema.BoolFlags {
				flags[i] = string(f)
			}
			fmt.Fprintf(&buf, "  bool-flags: %s\n", strings.Join(flags, ""))
		}
		if len(schema.ArgFlags) > 0 {
			for _, af := range schema.ArgFlags {
				fmt.Fprintf(&buf, "  arg-flag: -%c %s\n", af.Short, af.ArgType)
			}
		}
		if len(schema.Positionals) > 0 {
			for _, p := range schema.Positionals {
				opt := ""
				if !p.Required {
					opt = " (optional)"
				}
				vari := ""
				if p.Variadic {
					vari = " (variadic)"
				}
				fmt.Fprintf(&buf, "  positional: %s%s%s\n", p.Name, opt, vari)
			}
		}
		buf.WriteString("\n")
	}

	if len(failures) > 0 {
		t.Errorf("failed to parse %d commands:\n%s", len(failures), strings.Join(failures, "\n"))
	}

	golden := filepath.Join("testdata", "golden_schemas.txt")
	if *updateGolden {
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(golden, []byte(buf.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("updated golden file")
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("golden file not found — run with UPDATE_GOLDEN=1 to create: %v", err)
	}
	if got := buf.String(); got != string(want) {
		t.Errorf("golden file mismatch — run with UPDATE_GOLDEN=1 to update.\ngot:\n%s", got)
	}

	// also verify the registry builder
	labels := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			labels = append(labels, line)
		}
	}
	reg := BuildRegistry(labels)
	if len(reg) == 0 {
		t.Fatal("registry is empty")
	}
	// spot check: "attach-session" and "attach" should both resolve
	if _, ok := reg["attach-session"]; !ok {
		t.Error("registry missing attach-session")
	}
	if _, ok := reg["attach"]; !ok {
		t.Error("registry missing alias 'attach'")
	}
}
```

- [ ] **Step 6: Generate the golden file**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off UPDATE_GOLDEN=1 go test ./internal/cmdparse/ -run TestParseAllCommands -v`
Expected: PASS, golden file created

- [ ] **Step 7: Run test again without UPDATE_GOLDEN to confirm golden match**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/cmdparse/ -run TestParseAllCommands -v`
Expected: PASS

- [ ] **Step 8: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add internal/cmdparse/parse.go internal/cmdparse/parse_test.go internal/cmdparse/testdata/golden_schemas.txt
git commit -m "feat(cmdparse): add synopsis parser with golden file tests

parses tmux list-commands output into CommandSchema structs. handles
bool flag clusters, arg flags, positional args (required, optional,
variadic), aliases, and edge cases like bind-key nested optionals."
```

---

## Task 3: Input Analyser

**Files:**
- Create: `internal/cmdparse/analyse.go`
- Create: `internal/cmdparse/analyse_test.go`

- [ ] **Step 1: Write failing tests for input analysis**

```go
// internal/cmdparse/analyse_test.go
package cmdparse

import "testing"

// helper to build a schema for attach-session
func attachSchema() *CommandSchema {
	return &CommandSchema{
		Name:      "attach-session",
		Alias:     "attach",
		BoolFlags: []rune{'d', 'E', 'r', 'x'},
		ArgFlags: []ArgFlagDef{
			{Short: 'c', ArgType: "working-directory"},
			{Short: 'f', ArgType: "flags"},
			{Short: 't', ArgType: "target-session"},
		},
	}
}

// helper to build a schema for bind-key
func bindKeySchema() *CommandSchema {
	return &CommandSchema{
		Name:      "bind-key",
		Alias:     "bind",
		BoolFlags: []rune{'n', 'r'},
		ArgFlags: []ArgFlagDef{
			{Short: 'T', ArgType: "key-table"},
			{Short: 'N', ArgType: "note"},
		},
		Positionals: []PositionalDef{
			{Name: "key", Required: true},
			{Name: "command", Required: false},
			{Name: "argument", Required: false, Variadic: true},
		},
	}
}

func TestAnalyseEmptyInput(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "")
	if ctx.Kind != ContextCommandName {
		t.Fatalf("expected ContextCommandName, got %v", ctx.Kind)
	}
}

func TestAnalysePartialCommandName(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "att")
	if ctx.Kind != ContextCommandName {
		t.Fatalf("expected ContextCommandName, got %v", ctx.Kind)
	}
	if ctx.Prefix != "att" {
		t.Fatalf("expected prefix %q, got %q", "att", ctx.Prefix)
	}
}

func TestAnalyseAfterCommandNameSpace(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "attach-session ")
	if ctx.Kind != ContextFlagName {
		t.Fatalf("expected ContextFlagName, got %v", ctx.Kind)
	}
}

func TestAnalysePartialFlag(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "attach-session -")
	if ctx.Kind != ContextFlagName {
		t.Fatalf("expected ContextFlagName, got %v", ctx.Kind)
	}
	if ctx.Prefix != "-" {
		t.Fatalf("expected prefix %q, got %q", "-", ctx.Prefix)
	}
}

func TestAnalyseAfterBoolFlag(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "attach-session -d ")
	if ctx.Kind != ContextFlagName {
		t.Fatalf("expected ContextFlagName, got %v", ctx.Kind)
	}
	// -d should be marked as used
	found := false
	for _, f := range ctx.FlagsUsed {
		if f == 'd' {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'd' in FlagsUsed, got %v", ctx.FlagsUsed)
	}
}

func TestAnalyseAfterArgFlagSpace(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "attach-session -t ")
	if ctx.Kind != ContextFlagValue {
		t.Fatalf("expected ContextFlagValue, got %v", ctx.Kind)
	}
	if ctx.ArgType != "target-session" {
		t.Fatalf("expected ArgType %q, got %q", "target-session", ctx.ArgType)
	}
	if ctx.TypeLabel != "target-session" {
		t.Fatalf("expected TypeLabel %q, got %q", "target-session", ctx.TypeLabel)
	}
	if ctx.Prefix != "" {
		t.Fatalf("expected empty prefix, got %q", ctx.Prefix)
	}
}

func TestAnalyseMidFlagValue(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "attach-session -t mys")
	if ctx.Kind != ContextFlagValue {
		t.Fatalf("expected ContextFlagValue, got %v", ctx.Kind)
	}
	if ctx.ArgType != "target-session" {
		t.Fatalf("expected ArgType %q, got %q", "target-session", ctx.ArgType)
	}
	if ctx.Prefix != "mys" {
		t.Fatalf("expected prefix %q, got %q", "mys", ctx.Prefix)
	}
}

func TestAnalyseAfterFlagValueCompleted(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "attach-session -t mysess ")
	if ctx.Kind != ContextFlagName {
		t.Fatalf("expected ContextFlagName, got %v", ctx.Kind)
	}
	// -t should be used
	found := false
	for _, f := range ctx.FlagsUsed {
		if f == 't' {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 't' in FlagsUsed, got %v", ctx.FlagsUsed)
	}
}

func TestAnalysePositionalArg(t *testing.T) {
	reg := map[string]*CommandSchema{"bind-key": bindKeySchema()}
	ctx := Analyse(reg, "bind-key -n ")
	// after -n (bool flag), next is positional "key" since it's required
	if ctx.Kind != ContextFlagName {
		t.Fatalf("expected ContextFlagName (flags still available), got %v", ctx.Kind)
	}
}

func TestAnalysePositionalAfterAllFlags(t *testing.T) {
	reg := map[string]*CommandSchema{"bind-key": bindKeySchema()}
	// after "-n -T root", all flags used except -N; next token could be a flag or the positional "key"
	ctx := Analyse(reg, "bind-key -n -T root C-a ")
	// "C-a" consumed the required "key" positional, so next is "command" positional
	if ctx.Kind != ContextPositionalValue {
		t.Fatalf("expected ContextPositionalValue, got %v", ctx.Kind)
	}
	if ctx.ArgType != "command" {
		t.Fatalf("expected ArgType %q, got %q", "command", ctx.ArgType)
	}
}

func TestAnalyseUnknownCommand(t *testing.T) {
	reg := map[string]*CommandSchema{"attach-session": attachSchema()}
	ctx := Analyse(reg, "nonexistent-cmd ")
	if ctx.Kind != ContextNone {
		t.Fatalf("expected ContextNone for unknown command, got %v", ctx.Kind)
	}
}

func TestAnalyseAlias(t *testing.T) {
	reg := map[string]*CommandSchema{
		"attach-session": attachSchema(),
		"attach":         attachSchema(),
	}
	ctx := Analyse(reg, "attach -t ")
	if ctx.Kind != ContextFlagValue {
		t.Fatalf("expected ContextFlagValue, got %v", ctx.Kind)
	}
	if ctx.ArgType != "target-session" {
		t.Fatalf("expected ArgType %q, got %q", "target-session", ctx.ArgType)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/cmdparse/ -run TestAnalyse -v`
Expected: FAIL — `Analyse` not defined

- [ ] **Step 3: Implement the analyser**

```go
// internal/cmdparse/analyse.go
package cmdparse

import "strings"

// Analyse examines the current command input text and determines what kind of
// completion is appropriate. It looks up the command name in the registry to
// find its schema, then walks the tokens to determine the cursor context.
func Analyse(registry map[string]*CommandSchema, input string) CompletionContext {
	if input == "" {
		return CompletionContext{Kind: ContextCommandName}
	}

	tokens := strings.Fields(input)
	trailingSpace := strings.HasSuffix(input, " ")

	// still typing the command name (no space yet)
	if len(tokens) == 1 && !trailingSpace {
		return CompletionContext{Kind: ContextCommandName, Prefix: tokens[0]}
	}

	// look up the command schema
	cmdName := tokens[0]
	schema, ok := registry[cmdName]
	if !ok {
		return CompletionContext{Kind: ContextNone}
	}

	// walk tokens after the command name to understand state
	args := tokens[1:]
	var flagsUsed []rune
	positionalIdx := 0 // which positional we're up to

	i := 0
	for i < len(args) {
		tok := args[i]

		if strings.HasPrefix(tok, "-") && len(tok) == 2 {
			flag := rune(tok[1])
			flagsUsed = append(flagsUsed, flag)

			if isArgFlag(schema, flag) {
				// this flag expects a value
				if i+1 < len(args) {
					// value is the next token
					i += 2

					// if this is the last token and there's no trailing space,
					// user is still typing the value
					if i == len(args) && !trailingSpace {
						return CompletionContext{
							Kind:      ContextFlagValue,
							ArgType:   argFlagType(schema, flag),
							TypeLabel: argFlagType(schema, flag),
							Prefix:    args[i-1],
							FlagsUsed: flagsUsed,
						}
					}
					continue
				}
				// no value yet — if trailing space, we're expecting the value
				if trailingSpace {
					return CompletionContext{
						Kind:      ContextFlagValue,
						ArgType:   argFlagType(schema, flag),
						TypeLabel: argFlagType(schema, flag),
						FlagsUsed: flagsUsed,
					}
				}
				// cursor is on the flag itself (no space after it)
				return CompletionContext{
					Kind:      ContextFlagName,
					Prefix:    tok,
					FlagsUsed: flagsUsed,
				}
			}
			// bool flag — just consume it
			i++
			continue
		}

		if strings.HasPrefix(tok, "-") && len(tok) > 2 {
			// clustered flags like "-dr" — treat each char as a bool flag
			for _, r := range tok[1:] {
				flagsUsed = append(flagsUsed, r)
			}
			i++
			continue
		}

		// not a flag — it's a positional value
		positionalIdx++
		i++
	}

	// we've consumed all tokens — determine what comes next
	if !trailingSpace && len(args) > 0 {
		// cursor is at the end of the last token (still typing it)
		lastTok := args[len(args)-1]

		if strings.HasPrefix(lastTok, "-") {
			return CompletionContext{
				Kind:      ContextFlagName,
				Prefix:    lastTok,
				FlagsUsed: flagsUsed,
			}
		}

		// check if last token is a positional value being typed
		if positionalIdx > 0 && positionalIdx-1 < len(schema.Positionals) {
			p := schema.Positionals[positionalIdx-1]
			return CompletionContext{
				Kind:      ContextPositionalValue,
				ArgType:   p.Name,
				TypeLabel: p.Name,
				Prefix:    lastTok,
				FlagsUsed: flagsUsed,
			}
		}

		return CompletionContext{Kind: ContextNone, FlagsUsed: flagsUsed}
	}

	// trailing space — cursor is after completed token, expecting next thing
	// if there are still unused flags or positionals available, offer flags
	if hasUnusedFlags(schema, flagsUsed) {
		// if we've consumed all required positionals up to the current index,
		// and there are still positionals to fill, decide based on context
		if positionalIdx < len(schema.Positionals) {
			p := schema.Positionals[positionalIdx]
			if p.Required {
				// required positional takes priority but flags are still valid
				return CompletionContext{
					Kind:      ContextFlagName,
					FlagsUsed: flagsUsed,
				}
			}
		}
		return CompletionContext{
			Kind:      ContextFlagName,
			FlagsUsed: flagsUsed,
		}
	}

	// no more flags — check positionals
	if positionalIdx < len(schema.Positionals) {
		p := schema.Positionals[positionalIdx]
		return CompletionContext{
			Kind:      ContextPositionalValue,
			ArgType:   p.Name,
			TypeLabel: p.Name,
			FlagsUsed: flagsUsed,
		}
	}

	// check if last positional is variadic
	if len(schema.Positionals) > 0 {
		last := schema.Positionals[len(schema.Positionals)-1]
		if last.Variadic {
			return CompletionContext{
				Kind:      ContextPositionalValue,
				ArgType:   last.Name,
				TypeLabel: last.Name,
				FlagsUsed: flagsUsed,
			}
		}
	}

	return CompletionContext{Kind: ContextNone, FlagsUsed: flagsUsed}
}

// isArgFlag reports whether the given flag rune is an argument-taking flag.
func isArgFlag(schema *CommandSchema, flag rune) bool {
	for _, af := range schema.ArgFlags {
		if af.Short == flag {
			return true
		}
	}
	return false
}

// argFlagType returns the argument type for a given flag rune.
func argFlagType(schema *CommandSchema, flag rune) string {
	for _, af := range schema.ArgFlags {
		if af.Short == flag {
			return af.ArgType
		}
	}
	return ""
}

// hasUnusedFlags reports whether there are flags not yet used in the input.
func hasUnusedFlags(schema *CommandSchema, used []rune) bool {
	usedSet := make(map[rune]bool, len(used))
	for _, r := range used {
		usedSet[r] = true
	}
	for _, r := range schema.BoolFlags {
		if !usedSet[r] {
			return true
		}
	}
	for _, af := range schema.ArgFlags {
		if !usedSet[af.Short] {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/cmdparse/ -run TestAnalyse -v`
Expected: all PASS

- [ ] **Step 5: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/cmdparse/analyse.go internal/cmdparse/analyse_test.go
git commit -m "feat(cmdparse): add input analyser for completion context detection

walks parsed tokens to determine whether the cursor is at a command
name, flag name, flag value, or positional value position. tracks
used flags to avoid re-suggesting them."
```

---

## Task 4: Value Resolver

**Files:**
- Create: `internal/cmdparse/resolve.go`
- Create: `internal/cmdparse/resolve_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/cmdparse/resolve_test.go
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
}

func (m *mockResolver) Sessions() []string { return m.sessions }
func (m *mockResolver) Windows() []string  { return m.windows }
func (m *mockResolver) Panes() []string    { return m.panes }
func (m *mockResolver) Clients() []string  { return m.clients }
func (m *mockResolver) Commands() []string { return m.commands }
func (m *mockResolver) Buffers() []string  { return nil }

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
	// should exclude -d, include -E, -r, -x (bool) and -c, -f, -t (arg)
	if len(got) != 6 {
		t.Fatalf("expected 6 flag candidates, got %d: %v", len(got), got)
	}
	// verify -d is not present
	for _, fc := range got {
		if fc.Flag == 'd' {
			t.Fatal("flag 'd' should be excluded (already used)")
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/cmdparse/ -run "TestResolve|TestResolveFlagCandidates" -v`
Expected: FAIL

- [ ] **Step 3: Implement the resolver**

```go
// internal/cmdparse/resolve.go
package cmdparse

import "fmt"

// DataSource provides raw data for resolving completion candidates.
// The UI layer implements this by reading from state stores and tmux.
type DataSource interface {
	Sessions() []string
	Windows() []string
	Panes() []string
	Clients() []string
	Commands() []string
	Buffers() []string
}

// StoreResolver resolves argument types to completion candidates using
// a DataSource for live data and hardcoded values for known enums.
type StoreResolver struct {
	src DataSource
}

// NewStoreResolver creates a resolver backed by the given data source.
func NewStoreResolver(src DataSource) *StoreResolver {
	return &StoreResolver{src: src}
}

// Resolve returns completion candidates for the given argument type.
// Returns nil for types that cannot be completed (free-form text).
func (r *StoreResolver) Resolve(argType string) []string {
	switch argType {
	case "target-session", "session-name":
		return r.src.Sessions()
	case "target-window", "window-name", "src-window", "dst-window":
		return r.src.Windows()
	case "target-pane", "src-pane", "dst-pane", "pane":
		return r.src.Panes()
	case "target-client":
		return r.src.Clients()
	case "command":
		return r.src.Commands()
	case "buffer-name", "new-buffer-name":
		return r.src.Buffers()
	case "key-table":
		return []string{"root", "prefix", "copy-mode", "copy-mode-vi"}
	case "layout-name":
		return []string{"even-horizontal", "even-vertical", "main-horizontal", "main-vertical", "tiled"}
	case "prompt-type":
		return []string{"command", "search", "target", "window-target"}
	default:
		return nil
	}
}

// FlagCandidate represents a flag available for completion.
type FlagCandidate struct {
	Flag    rune
	Label   string // display label, e.g. "-t target-session" or "-d"
	ArgType string // empty for bool flags
}

// FlagCandidates returns the flags from the schema that are not yet used,
// sorted with arg flags first, then bool flags.
func FlagCandidates(schema *CommandSchema, used []rune) []FlagCandidate {
	usedSet := make(map[rune]bool, len(used))
	for _, r := range used {
		usedSet[r] = true
	}

	var result []FlagCandidate

	// arg flags first (more useful for completion)
	for _, af := range schema.ArgFlags {
		if usedSet[af.Short] {
			continue
		}
		result = append(result, FlagCandidate{
			Flag:    af.Short,
			Label:   fmt.Sprintf("-%c %s", af.Short, af.ArgType),
			ArgType: af.ArgType,
		})
	}

	// then bool flags
	for _, bf := range schema.BoolFlags {
		if usedSet[bf] {
			continue
		}
		result = append(result, FlagCandidate{
			Flag:  bf,
			Label: fmt.Sprintf("-%c", bf),
		})
	}

	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/cmdparse/ -run "TestResolve|TestResolveFlagCandidates" -v`
Expected: all PASS

- [ ] **Step 5: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/cmdparse/resolve.go internal/cmdparse/resolve_test.go
git commit -m "feat(cmdparse): add value resolver for completion candidates

maps argument types to live data sources (sessions, windows, panes,
clients, buffers, commands) and hardcoded enums (key-table, layout,
prompt-type). FlagCandidates returns unused flags sorted by utility."
```

---

## Task 5: Completion Styles

**Files:**
- Modify: `internal/theme/theme.go`

- [ ] **Step 1: Read current theme file**

Read `internal/theme/theme.go` fully before editing.

- [ ] **Step 2: Add completion styles to the Styles struct**

Add three new fields to the `Styles` struct (after `ProgressEmptyBg`):

```go
CompletionBorder   *lipgloss.Style
CompletionItem     *lipgloss.Style
CompletionSelected *lipgloss.Style
```

- [ ] **Step 3: Add default style definitions**

In the `Default()` function (or wherever styles are initialised), add:

```go
completionBorder := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
completionItem := lipgloss.NewStyle().Foreground(lipgloss.Color("249"))
completionSelected := lipgloss.NewStyle().
	Foreground(lipgloss.Color("255")).
	Background(lipgloss.Color("33"))
```

And assign them:
```go
CompletionBorder:   &completionBorder,
CompletionItem:     &completionItem,
CompletionSelected: &completionSelected,
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make build`
Expected: builds successfully

- [ ] **Step 5: Commit**

```bash
git add internal/theme/theme.go
git commit -m "feat(theme): add completion dropdown styles

adds CompletionBorder, CompletionItem, CompletionSelected styles for
the argument completion dropdown popup."
```

---

## Task 6: Completion Dropdown Widget

**Files:**
- Create: `internal/ui/completion.go`
- Create: `internal/ui/completion_test.go`

- [ ] **Step 1: Write failing tests for completion state**

```go
// internal/ui/completion_test.go
package ui

import "testing"

func TestCompletionFilter(t *testing.T) {
	cs := newCompletionState([]string{"main", "work", "scratch"}, "target-session", "target-session", 5)
	if len(cs.filtered) != 3 {
		t.Fatalf("expected 3 filtered items, got %d", len(cs.filtered))
	}

	cs.applyFilter("ma")
	if len(cs.filtered) != 1 {
		t.Fatalf("expected 1 filtered item for 'ma', got %d: %v", len(cs.filtered), cs.filtered)
	}
	if cs.filtered[0] != "main" {
		t.Fatalf("expected 'main', got %q", cs.filtered[0])
	}
}

func TestCompletionCursorBounds(t *testing.T) {
	cs := newCompletionState([]string{"a", "b", "c"}, "", "", 0)
	cs.moveDown()
	if cs.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", cs.cursor)
	}
	cs.moveDown()
	cs.moveDown() // should clamp
	if cs.cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", cs.cursor)
	}
	cs.moveUp()
	if cs.cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", cs.cursor)
	}
	cs.moveUp()
	cs.moveUp() // should clamp
	if cs.cursor != 0 {
		t.Fatalf("expected cursor 0, got %d", cs.cursor)
	}
}

func TestCompletionSelected(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "", "", 0)
	if cs.selected() != "main" {
		t.Fatalf("expected 'main', got %q", cs.selected())
	}
	cs.moveDown()
	if cs.selected() != "work" {
		t.Fatalf("expected 'work', got %q", cs.selected())
	}
}

func TestCompletionSelectedEmpty(t *testing.T) {
	cs := newCompletionState(nil, "", "", 0)
	if cs.selected() != "" {
		t.Fatalf("expected empty, got %q", cs.selected())
	}
}

func TestCompletionGhostHintNoInput(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	ghost := cs.ghostHint("")
	// with no input, ghost is the selected item
	if ghost != "main" {
		t.Fatalf("expected ghost 'main', got %q", ghost)
	}
}

func TestCompletionGhostHintWithSelection(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	cs.moveDown()
	ghost := cs.ghostHint("")
	if ghost != "work" {
		t.Fatalf("expected ghost 'work', got %q", ghost)
	}
}

func TestCompletionGhostHintUniquePrefix(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	cs.applyFilter("ma")
	ghost := cs.ghostHint("ma")
	// unique match "main", so ghost is the suffix "in"
	if ghost != "in" {
		t.Fatalf("expected ghost 'in', got %q", ghost)
	}
}

func TestCompletionGhostHintMultipleMatches(t *testing.T) {
	cs := newCompletionState([]string{"main", "master"}, "target-session", "target-session", 0)
	cs.applyFilter("ma")
	ghost := cs.ghostHint("ma")
	// multiple matches — ghost shows the selected item's suffix
	if ghost != "in" {
		t.Fatalf("expected ghost 'in' (from selected 'main'), got %q", ghost)
	}
}

func TestCompletionGhostHintNoMatch(t *testing.T) {
	cs := newCompletionState([]string{"main", "work"}, "target-session", "target-session", 0)
	cs.applyFilter("xyz")
	ghost := cs.ghostHint("xyz")
	if ghost != "" {
		t.Fatalf("expected empty ghost, got %q", ghost)
	}
}

func TestCompletionView(t *testing.T) {
	cs := newCompletionState([]string{"main", "work", "scratch"}, "target-session", "target-session", 0)
	view := cs.view(30, 10)
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	// should contain all three items
	for _, name := range []string{"main", "work", "scratch"} {
		if !strings.Contains(view, name) {
			t.Errorf("view should contain %q", name)
		}
	}
}

// Note: use strings.Contains from the standard library for assertions.
// Import "strings" at the top of the test file.
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/ -run TestCompletion -v`
Expected: FAIL

- [ ] **Step 3: Implement the completion dropdown widget**

```go
// internal/ui/completion.go
package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const completionMaxVisible = 10

// completionState manages the dropdown popup for argument completion.
type completionState struct {
	visible   bool
	items     []string // all candidates
	filtered  []string // after prefix filter
	cursor    int      // index in filtered
	prefix    string   // typed text for current token
	anchorCol int      // prompt column where dropdown anchors
	argType   string   // argument type for resolution
	typeLabel string   // display label for ghost hint placeholder
}

// newCompletionState creates a completion state with the given candidates.
func newCompletionState(items []string, argType, typeLabel string, anchorCol int) *completionState {
	cs := &completionState{
		visible:   true,
		items:     items,
		argType:   argType,
		typeLabel: typeLabel,
		anchorCol: anchorCol,
	}
	cs.filtered = append([]string{}, items...)
	return cs
}

// applyFilter narrows the candidate list by prefix match and resets cursor.
func (cs *completionState) applyFilter(prefix string) {
	cs.prefix = prefix
	if prefix == "" {
		cs.filtered = append(cs.filtered[:0], cs.items...)
		cs.cursor = 0
		return
	}
	lower := strings.ToLower(prefix)
	cs.filtered = cs.filtered[:0]
	for _, item := range cs.items {
		if strings.HasPrefix(strings.ToLower(item), lower) {
			cs.filtered = append(cs.filtered, item)
		}
	}
	if cs.cursor >= len(cs.filtered) {
		cs.cursor = 0
	}
}

// moveDown moves the cursor down in the filtered list.
func (cs *completionState) moveDown() {
	if cs.cursor < len(cs.filtered)-1 {
		cs.cursor++
	}
}

// moveUp moves the cursor up in the filtered list.
func (cs *completionState) moveUp() {
	if cs.cursor > 0 {
		cs.cursor--
	}
}

// selected returns the currently highlighted candidate, or "" if empty.
func (cs *completionState) selected() string {
	if len(cs.filtered) == 0 || cs.cursor >= len(cs.filtered) {
		return ""
	}
	return cs.filtered[cs.cursor]
}

// ghostHint returns the ghost text to display after the typed prefix.
// Priority:
//  1. If filtered list has items and one is selected, show selected minus prefix
//  2. If no prefix and typeLabel set, return "" (caller uses typeLabel as placeholder)
//  3. Otherwise return ""
func (cs *completionState) ghostHint(typedPrefix string) string {
	if len(cs.filtered) == 0 {
		return ""
	}

	sel := cs.selected()
	if sel == "" {
		return ""
	}

	if typedPrefix == "" {
		return sel
	}

	// if the selected item starts with the prefix, return the suffix
	if strings.HasPrefix(strings.ToLower(sel), strings.ToLower(typedPrefix)) {
		return sel[len(typedPrefix):]
	}

	return ""
}

// view renders the dropdown box as a string. maxWidth and maxHeight constrain
// the rendering area.
func (cs *completionState) view(maxWidth, maxHeight int) string {
	if len(cs.filtered) == 0 {
		return ""
	}

	visible := cs.filtered
	startIdx := 0
	maxRows := completionMaxVisible
	if maxHeight > 0 && maxHeight < maxRows {
		maxRows = maxHeight
	}
	if len(visible) > maxRows {
		// scroll to keep cursor visible
		if cs.cursor >= startIdx+maxRows {
			startIdx = cs.cursor - maxRows + 1
		}
		if cs.cursor < startIdx {
			startIdx = cs.cursor
		}
		visible = visible[startIdx : startIdx+maxRows]
	}

	// determine max item width for consistent padding
	maxItemW := 0
	for _, item := range visible {
		if len(item) > maxItemW {
			maxItemW = len(item)
		}
	}
	if maxItemW > maxWidth-4 { // leave room for border + padding
		maxItemW = maxWidth - 4
	}
	if maxItemW < 1 {
		maxItemW = 1
	}

	itemStyle := styles.CompletionItem
	selectedStyle := styles.CompletionSelected
	if itemStyle == nil {
		plain := lipgloss.NewStyle()
		itemStyle = &plain
	}
	if selectedStyle == nil {
		sel := lipgloss.NewStyle().Reverse(true)
		selectedStyle = &sel
	}

	var lines []string
	for i, item := range visible {
		text := item
		if len(text) > maxItemW {
			text = text[:maxItemW]
		}
		padded := text + strings.Repeat(" ", maxItemW-len(text))
		if startIdx+i == cs.cursor {
			lines = append(lines, selectedStyle.Render(" "+padded+" "))
		} else {
			lines = append(lines, itemStyle.Render(" "+padded+" "))
		}
	}

	// add scroll indicators
	if startIdx > 0 {
		lines[0] = itemStyle.Render(" " + strings.Repeat(" ", maxItemW-1) + "^" + " ")
	}
	if startIdx+len(visible) < len(cs.filtered) {
		lines[len(lines)-1] = itemStyle.Render(" " + strings.Repeat(" ", maxItemW-1) + "v" + " ")
	}

	body := strings.Join(lines, "\n")

	borderStyle := styles.CompletionBorder
	if borderStyle == nil {
		bs := lipgloss.NewStyle()
		borderStyle = &bs
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderStyle.GetForeground()).
		Render(body)

	return box
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/ -run TestCompletion -v`
Expected: all PASS

- [ ] **Step 5: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/completion.go internal/ui/completion_test.go
git commit -m "feat(ui): add completion dropdown widget

manages dropdown state (candidates, filtering, cursor, selection),
renders bordered popup box with scroll indicators, and computes ghost
hints based on selection state and typed prefix."
```

---

## Task 7: Wire Schema Registry into Model

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/commands.go`

- [ ] **Step 1: Read current files**

Read `internal/ui/model.go` lines 78-135 and `internal/ui/commands.go` lines 47-79.

- [ ] **Step 2: Add fields to Model struct**

In `internal/ui/model.go`, add to the `Model` struct (after `commandItemsCache` on line 102):

```go
commandSchemas map[string]*cmdparse.CommandSchema
completion     *completionState
```

Add the import for `cmdparse`:
```go
"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
```

- [ ] **Step 3: Build schema registry on preload**

In `internal/ui/commands.go`, in `handleCommandPreloadMsg` (around line 77 where `m.commandItemsCache = preload.items`), add after that line:

```go
labels := make([]string, len(preload.items))
for i, item := range preload.items {
	labels[i] = item.Label
}
m.commandSchemas = cmdparse.BuildRegistry(labels)
```

Add the import for `cmdparse`:
```go
"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make build`
Expected: builds successfully

- [ ] **Step 5: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/ui/model.go internal/ui/commands.go
git commit -m "feat(ui): build command schema registry from preloaded items

parses list-commands output into CommandSchema structs on preload,
indexed by name and alias. adds completion state field to Model."
```

---

## Task 8: Data Source Implementation

**Files:**
- Create: `internal/ui/completion_datasource.go`

This connects the `cmdparse.DataSource` interface to the Model's state stores so the resolver can access live session/window/pane data.

- [ ] **Step 1: Implement the data source adapter**

```go
// internal/ui/completion_datasource.go
package ui

import (
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/state"
)

// modelDataSource adapts the Model's state stores to the cmdparse.DataSource
// interface for resolving completion candidates.
type modelDataSource struct {
	sessions state.SessionStore
	windows  state.WindowStore
	panes    state.PaneStore
	schemas  map[string]*cmdparse.CommandSchema
}

func (ds *modelDataSource) Sessions() []string {
	entries := ds.sessions.Entries()
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names
}

func (ds *modelDataSource) Windows() []string {
	entries := ds.windows.Entries()
	labels := make([]string, len(entries))
	for i, e := range entries {
		labels[i] = e.Session + ":" + e.Name
	}
	return labels
}

func (ds *modelDataSource) Panes() []string {
	entries := ds.panes.Entries()
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.PaneID
	}
	return ids
}

func (ds *modelDataSource) Clients() []string {
	// clients are not in state stores; return nil for now.
	// a future enhancement can fetch via tmux list-clients.
	return nil
}

func (ds *modelDataSource) Commands() []string {
	names := make([]string, 0, len(ds.schemas))
	seen := make(map[string]bool)
	for name, schema := range ds.schemas {
		if name == schema.Name && !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	return names
}

func (ds *modelDataSource) Buffers() []string {
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make build`
Expected: builds successfully

- [ ] **Step 3: Commit**

```bash
git add internal/ui/completion_datasource.go
git commit -m "feat(ui): add data source adapter for completion resolver

bridges Model's session/window/pane state stores to the
cmdparse.DataSource interface for resolving completion candidates."
```

---

## Task 9: Integration — Completion Triggering

**Files:**
- Modify: `internal/ui/input.go`
- Modify: `internal/ui/navigation.go`

This is the core wiring: triggering the analyser on input changes, opening/closing the dropdown, and routing keys when the dropdown is visible.

- [ ] **Step 1: Read current input.go and navigation.go fully**

Read `internal/ui/input.go` and `internal/ui/navigation.go` to understand the current flow before making changes.

- [ ] **Step 2: Add completion trigger method to Model**

Add to `internal/ui/input.go` (after `autoCompleteGhost`):

```go
// triggerCompletion analyses the current command input and opens/updates/closes
// the completion dropdown as appropriate.
func (m *Model) triggerCompletion() {
	current := m.currentLevel()
	if current == nil || current.Node == nil || !current.Node.FilterCommand {
		m.dismissCompletion()
		return
	}
	if m.commandSchemas == nil {
		return
	}

	ctx := cmdparse.Analyse(m.commandSchemas, current.Filter)

	switch ctx.Kind {
	case cmdparse.ContextFlagName:
		schema := m.lookupCommandSchema(current.Filter)
		if schema == nil {
			m.dismissCompletion()
			return
		}
		candidates := cmdparse.FlagCandidates(schema, ctx.FlagsUsed)
		if len(candidates) == 0 {
			m.dismissCompletion()
			return
		}
		labels := make([]string, len(candidates))
		for i, c := range candidates {
			labels[i] = c.Label
		}
		m.openCompletion(labels, "flag", "flag", ctx.Prefix)

	case cmdparse.ContextFlagValue, cmdparse.ContextPositionalValue:
		ds := &modelDataSource{
			sessions: m.sessions,
			windows:  m.windows,
			panes:    m.panes,
			schemas:  m.commandSchemas,
		}
		resolver := cmdparse.NewStoreResolver(ds)
		candidates := resolver.Resolve(ctx.ArgType)
		if candidates == nil || len(candidates) == 0 {
			// non-resolvable type — show ghost hint for type label only
			m.completion = &completionState{
				visible:   false,
				typeLabel: ctx.TypeLabel,
				argType:   ctx.ArgType,
				prefix:    ctx.Prefix,
			}
			return
		}
		m.openCompletion(candidates, ctx.ArgType, ctx.TypeLabel, ctx.Prefix)

	default:
		m.dismissCompletion()
	}
}

// openCompletion creates or updates the completion dropdown.
func (m *Model) openCompletion(items []string, argType, typeLabel, prefix string) {
	anchorCol := len(m.currentLevel().Filter) - len(prefix) + 3 // +3 for "» " prompt
	m.completion = newCompletionState(items, argType, typeLabel, anchorCol)
	if prefix != "" {
		m.completion.applyFilter(prefix)
	}
	if len(m.completion.filtered) == 0 {
		m.dismissCompletion()
	}
}

// dismissCompletion closes the completion dropdown.
func (m *Model) dismissCompletion() {
	m.completion = nil
}

// lookupCommandSchema extracts the command name from the input and looks it up.
func (m *Model) lookupCommandSchema(input string) *cmdparse.CommandSchema {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return nil
	}
	return m.commandSchemas[fields[0]]
}

// completionVisible reports whether the completion dropdown is currently shown.
func (m *Model) completionVisible() bool {
	return m.completion != nil && m.completion.visible && len(m.completion.filtered) > 0
}
```

- [ ] **Step 3: Add completion trigger calls to handleTextInput**

In `internal/ui/input.go`, in `handleTextInput()`, add a call to `m.triggerCompletion()` at each point where the filter text changes. Specifically, after each `current.SetFilter(...)` or `current.InsertFilterText(...)` call, add:

```go
m.triggerCompletion()
```

This applies to:
- After ctrl+u clears filter (dismiss completion)
- After ctrl+w deletes word (re-trigger)
- After backspace (re-trigger)
- After regular character input (re-trigger)
- After space input (re-trigger — this is the main trigger point for flags)

- [ ] **Step 4: Add completion key routing to handleKeyMsg**

In `internal/ui/navigation.go`, in `handleKeyMsg()`, add a completion routing block near the top (before the existing tab handler at line 328):

```go
// completion dropdown key routing
if m.completionVisible() {
	switch keyMsg.String() {
	case "up":
		m.completion.moveUp()
		return nil
	case "down":
		m.completion.moveDown()
		return nil
	case "tab":
		return m.acceptCompletion()
	case "escape":
		m.dismissCompletion()
		return nil
	case "enter":
		return m.acceptCompletion()
	}
}
```

Add the `acceptCompletion` method (can be in `input.go` or `navigation.go`):

```go
// acceptCompletion inserts the selected completion into the filter text and
// dismisses the dropdown.
func (m *Model) acceptCompletion() tea.Cmd {
	if m.completion == nil {
		return nil
	}
	sel := m.completion.selected()
	if sel == "" {
		m.dismissCompletion()
		return nil
	}

	current := m.currentLevel()
	if current == nil {
		m.dismissCompletion()
		return nil
	}

	// replace the current token prefix with the selected value
	filter := current.Filter
	prefix := m.completion.prefix
	if prefix != "" && strings.HasSuffix(filter, prefix) {
		filter = filter[:len(filter)-len(prefix)]
	}
	newFilter := filter + sel + " "
	before := current.FilterCursorPos()
	current.SetFilter(newFilter, len([]rune(newFilter)))
	m.noteFilterCursorChange(current, before)
	m.syncFilterViewport(current)

	m.dismissCompletion()
	m.triggerCompletion() // check for next completion point
	return nil
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make build`
Expected: builds successfully

- [ ] **Step 6: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/input.go internal/ui/navigation.go
git commit -m "feat(ui): wire completion triggering and key routing

triggerCompletion analyses input on every keystroke and opens/closes
the dropdown. key routing redirects arrows, tab, escape, and enter
through the completion state when the dropdown is visible."
```

---

## Task 10: Ghost Hint Extension

**Files:**
- Modify: `internal/ui/input.go`

- [ ] **Step 1: Read current autoCompleteGhost**

Read `internal/ui/input.go` lines 263-287 to understand the current ghost logic.

- [ ] **Step 2: Extend autoCompleteGhost for argument hints**

Replace the `autoCompleteGhost` function with an extended version that handles argument completion ghost hints in addition to command name ghosts:

```go
func (m *Model) autoCompleteGhost() string {
	current := m.currentLevel()
	if current == nil {
		return ""
	}
	if current.Node == nil || !current.Node.FilterCommand {
		return ""
	}
	if current.FilterCursorPos() != len([]rune(current.Filter)) {
		return ""
	}

	// priority 1: completion dropdown ghost
	if m.completion != nil {
		if m.completion.visible && len(m.completion.filtered) > 0 {
			return m.completion.ghostHint(m.completion.prefix)
		}
		// non-visible completion with a type label = placeholder hint
		if !m.completion.visible && m.completion.typeLabel != "" && m.completion.prefix == "" {
			return m.completion.typeLabel
		}
	}

	// priority 2: command name ghost (existing logic)
	if current.Filter == "" {
		return ""
	}
	if current.Cursor < 0 || current.Cursor >= len(current.Items) {
		return ""
	}
	// only apply command name ghost when we're still on the first token
	if strings.Contains(current.Filter, " ") {
		return ""
	}
	item := current.Items[current.Cursor]
	lower := strings.ToLower(current.Filter)
	idLower := strings.ToLower(item.ID)
	if !strings.HasPrefix(idLower, lower) {
		return ""
	}
	return item.ID[len(current.Filter):]
}
```

- [ ] **Step 3: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/ui/input.go
git commit -m "feat(ui): extend ghost hints for argument completion

ghost text now shows: dropdown selection suffix when dropdown is open,
type label placeholder when awaiting a value, and command name suffix
only for the first token."
```

---

## Task 11: Dropdown Overlay Rendering

**Files:**
- Modify: `internal/ui/view.go`

- [ ] **Step 1: Read current view.go rendering**

Read `internal/ui/view.go` lines 217-234 (viewVertical bottom bar) and lines 331-344 (viewSideBySide bottom bar).

- [ ] **Step 2: Add overlay helper**

Add a helper function to `view.go` that overlays the completion dropdown above the prompt line:

```go
// overlayCompletion renders the completion dropdown above the prompt line.
// It takes the fully rendered view string and overlays the dropdown box
// at the appropriate position.
func (m *Model) overlayCompletion(rendered string) string {
	if !m.completionVisible() {
		return rendered
	}

	// render the dropdown
	maxH := m.height - 4 // leave room for header, status, prompt
	if maxH < 3 {
		maxH = 3
	}
	maxW := m.width - m.completion.anchorCol
	if maxW < 20 {
		maxW = 20
	}
	dropdown := m.completion.view(maxW, maxH)
	if dropdown == "" {
		return rendered
	}

	// split the rendered view into lines
	lines := strings.Split(rendered, "\n")
	dropLines := strings.Split(dropdown, "\n")

	// the dropdown should appear above the last 2 lines (status + prompt)
	// so insert it ending at len(lines)-2
	insertEnd := len(lines) - 2
	insertStart := insertEnd - len(dropLines)
	if insertStart < 0 {
		insertStart = 0
	}

	// overlay each dropdown line at the anchor column
	for i, dLine := range dropLines {
		lineIdx := insertStart + i
		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}
		lines[lineIdx] = overlayAt(lines[lineIdx], dLine, m.completion.anchorCol, m.width)
	}

	return strings.Join(lines, "\n")
}

// overlayAt inserts overlayStr into baseLine starting at column col.
func overlayAt(baseLine, overlayStr string, col, maxWidth int) string {
	// use ANSI-aware width measurement
	baseW := lipgloss.Width(baseLine)

	// pad base line to at least col width
	if baseW < col {
		baseLine += strings.Repeat(" ", col-baseW)
	}

	// for simplicity, truncate base at col, append overlay, then pad
	// this overwrites whatever was at the overlay position
	baseRunes := []rune(ansi.Strip(baseLine))
	prefix := string(baseRunes[:min(col, len(baseRunes))])

	return prefix + overlayStr
}
```

Add `min` helper if not available (Go 1.21+ has built-in `min`).

- [ ] **Step 3: Apply overlay in viewVertical**

In `viewVertical`, before the `return renderLines(lines)` call (line 233), wrap the result:

```go
result := renderLines(lines)
return m.overlayCompletion(result)
```

- [ ] **Step 4: Apply overlay in viewSideBySide**

In `viewSideBySide`, before the final return (line 344), wrap the result:

```go
result := topSection + "\n" + bottomStr
return m.overlayCompletion(result)
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make build`
Expected: builds successfully

- [ ] **Step 6: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/view.go
git commit -m "feat(ui): render completion dropdown overlay above prompt

overlays the dropdown box above the status/prompt lines in both
viewVertical and viewSideBySide layouts, anchored at the completion
token's column position."
```

---

## Task 12: Harness Tests for End-to-End Completion

**Files:**
- Create: `internal/ui/completion_harness_test.go`

- [ ] **Step 1: Write harness tests for the completion flow**

```go
// internal/ui/completion_harness_test.go
package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/state"
)

// setupCommandHarness creates a test harness with the command menu active,
// preloaded schemas, and mock state stores.
func setupCommandHarness(t *testing.T) *Harness {
	t.Helper()

	sessions := state.NewSessionStore()
	sessions.SetEntries([]menu.SessionEntry{
		{Name: "main"},
		{Name: "work"},
		{Name: "scratch"},
	})

	windows := state.NewWindowStore()
	windows.SetEntries([]menu.WindowEntry{
		{Name: "0", Session: "main"},
		{Name: "1", Session: "main"},
		{Name: "0", Session: "work"},
	})

	panes := state.NewPaneStore()

	reg := menu.NewTestRegistry() // or however the test registry is constructed
	model := NewModel(ModelConfig{
		SocketPath: "/tmp/test.sock",
		Sessions:   sessions,
		Windows:    windows,
		Panes:      panes,
		Registry:   reg,
	})

	// inject command schemas
	model.commandSchemas = cmdparse.BuildRegistry([]string{
		"kill-session [-aC] [-t target-session]",
		"swap-window (swapw) [-d] [-s src-window] [-t target-window]",
		"bind-key (bind) [-nr] [-T key-table] [-N note] key [command [argument ...]]",
	})

	// inject command items cache and navigate to command level
	model.commandItemsCache = []menu.Item{
		{ID: "kill-session", Label: "kill-session [-aC] [-t target-session]"},
		{ID: "swap-window", Label: "swap-window (swapw) [-d] [-s src-window] [-t target-window]"},
		{ID: "bind-key", Label: "bind-key (bind) [-nr] [-T key-table] [-N note] key [command [argument ...]]"},
	}

	h := NewHarness(model)

	// navigate to command menu (this depends on how the test registry is set up)
	// if the command level needs to be pushed manually:
	node, _ := reg.Find("command")
	if node != nil {
		lvl := newLevel("command", "command", model.commandItemsCache, node)
		model.stack = append(model.stack, lvl)
	}

	return h
}

func TestCompletionTriggersOnFlagValue(t *testing.T) {
	h := setupCommandHarness(t)

	// type "kill-session -t "
	for _, ch := range "kill-session -t " {
		h.Send(tea.KeyMsg{Runes: []rune{ch}})
	}

	// completion should be visible with session names
	if !h.model.completionVisible() {
		t.Fatal("expected completion dropdown to be visible after '-t '")
	}

	sel := h.model.completion.selected()
	if sel == "" {
		t.Fatal("expected a selected completion candidate")
	}
}

func TestCompletionArrowNavigation(t *testing.T) {
	h := setupCommandHarness(t)

	for _, ch := range "kill-session -t " {
		h.Send(tea.KeyMsg{Runes: []rune{ch}})
	}

	first := h.model.completion.selected()
	h.Send(tea.KeyMsg{Type: tea.KeyDown})
	second := h.model.completion.selected()
	if first == second {
		t.Fatal("expected cursor to move to different item")
	}
	h.Send(tea.KeyMsg{Type: tea.KeyUp})
	back := h.model.completion.selected()
	if back != first {
		t.Fatalf("expected to return to %q, got %q", first, back)
	}
}

func TestCompletionTabAccepts(t *testing.T) {
	h := setupCommandHarness(t)

	for _, ch := range "kill-session -t " {
		h.Send(tea.KeyMsg{Runes: []rune{ch}})
	}

	sel := h.model.completion.selected()
	h.Send(tea.KeyMsg{Type: tea.KeyTab})

	current := h.model.currentLevel()
	if current == nil {
		t.Fatal("expected current level")
	}
	if !strings.Contains(current.Filter, sel) {
		t.Fatalf("expected filter to contain %q, got %q", sel, current.Filter)
	}
	// dropdown should be dismissed (or re-triggered for next point)
}

func TestCompletionEscapeDismisses(t *testing.T) {
	h := setupCommandHarness(t)

	for _, ch := range "kill-session -t " {
		h.Send(tea.KeyMsg{Runes: []rune{ch}})
	}

	if !h.model.completionVisible() {
		t.Fatal("expected dropdown visible")
	}

	h.Send(tea.KeyMsg{Type: tea.KeyEscape})
	if h.model.completionVisible() {
		t.Fatal("expected dropdown dismissed after escape")
	}
}

func TestCompletionTypeToFilter(t *testing.T) {
	h := setupCommandHarness(t)

	for _, ch := range "kill-session -t " {
		h.Send(tea.KeyMsg{Runes: []rune{ch}})
	}

	// type "ma" to filter to "main"
	h.Send(tea.KeyMsg{Runes: []rune{'m'}})
	h.Send(tea.KeyMsg{Runes: []rune{'a'}})

	if !h.model.completionVisible() {
		t.Fatal("expected dropdown still visible while filtering")
	}
	if len(h.model.completion.filtered) != 1 {
		t.Fatalf("expected 1 filtered candidate, got %d: %v", len(h.model.completion.filtered), h.model.completion.filtered)
	}
	if h.model.completion.filtered[0] != "main" {
		t.Fatalf("expected 'main', got %q", h.model.completion.filtered[0])
	}
}

func TestGhostHintShowsTypeLabel(t *testing.T) {
	h := setupCommandHarness(t)

	// type "kill-session -t" (no trailing space yet — after this the ghost
	// should not show the type label since we're mid-flag)
	for _, ch := range "kill-session -t" {
		h.Send(tea.KeyMsg{Runes: []rune{ch}})
	}

	// now add space to trigger flag value context
	h.Send(tea.KeyMsg{Runes: []rune{' '}})

	ghost := h.model.autoCompleteGhost()
	// ghost should show either the first candidate or the type label
	if ghost == "" {
		t.Fatal("expected non-empty ghost hint after '-t '")
	}
}
```

Note: the `setupCommandHarness` function will need adjustment based on the exact constructor signatures of `NewModel`, `state.NewSessionStore`, etc. The implementing agent should read the actual constructors and adapt. The key patterns to follow are in the existing harness tests in `internal/ui/`.

- [ ] **Step 2: Run tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/ -run "TestCompletion|TestGhostHint" -v`
Expected: all PASS (may require adjustments to setupCommandHarness based on actual API)

- [ ] **Step 3: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/ui/completion_harness_test.go
git commit -m "test(ui): add harness tests for completion flow

covers dropdown triggering on flag values, arrow navigation, tab
accept, escape dismiss, type-to-filter, and ghost hint display."
```

---

## Task 13: Polish and Edge Cases

**Files:**
- Modify: `internal/ui/input.go`
- Modify: `internal/ui/navigation.go`
- Modify: `internal/ui/completion.go`

- [ ] **Step 1: Handle backspace through completed values**

In `handleTextInput` in `input.go`, after the backspace handling section where the filter is modified, ensure `triggerCompletion()` is called. If the user backspaces past a flag value into the flag itself, the dropdown should re-open with flag options. This should already work from the triggerCompletion calls added in Task 9, but verify.

- [ ] **Step 2: Handle window resize**

In `model.go` where `tea.WindowSizeMsg` is handled, add:

```go
// dismiss completion on resize — position may be stale
m.dismissCompletion()
```

- [ ] **Step 3: Verify existing command name completion still works**

Run a quick manual check or write a test: type a partial command name (no space), confirm ghost text appears and tab completes. The extended `autoCompleteGhost` should fall through to the existing command-name logic when the input has no spaces.

- [ ] **Step 4: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 5: Build**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make build`
Expected: builds successfully

- [ ] **Step 6: Commit**

```bash
git add internal/ui/input.go internal/ui/navigation.go internal/ui/model.go internal/ui/completion.go
git commit -m "fix(ui): handle edge cases in completion dropdown

dismiss completion on window resize, ensure backspace re-triggers
analysis, verify command name ghost still works for first token."
```

---

## Review Checkpoint

After Task 13, all core functionality is implemented. Before proceeding to integration testing:

1. Run `make test` — all tests should pass
2. Run `make build` — binary should compile
3. Manual smoke test: run the binary against a real tmux session, navigate to the command menu, type a command with flags, verify the dropdown appears and works

---

## Task 14: Integration Test

**Files:**
- Modify: `internal/ui/integration_test.go` (or create `internal/ui/completion_integration_test.go`)

- [ ] **Step 1: Write integration test**

Add an integration test that exercises the full completion flow with a live tmux server. Follow the existing patterns in `internal/ui/integration_test.go`:

```go
func TestCommandCompletionIntegration(t *testing.T) {
	srv := testutil.StartTmuxServer(t)
	defer srv.KillServer()

	session := "test-completion"
	srv.NewSession(session)

	bin := testutil.BuildBinary(t)
	srv.LaunchBinary(t, bin, session, "command")

	// wait for command menu to render
	srv.WaitForContent(t, "type to search", 5*time.Second)

	// type a command that takes a session argument
	srv.SendText(t, "kill-session -t ")

	// wait for dropdown to appear with the session name
	srv.WaitForContent(t, session, 5*time.Second)

	// press tab to accept
	srv.SendKeys(t, "Tab")

	// verify the session name was inserted
	srv.WaitForContent(t, "kill-session -t "+session, 5*time.Second)
}
```

Note: adapt this to the actual `testutil` API. The key is: launch binary in command mode, type a command with a flag that takes a session target, wait for the dropdown to show the session name, tab-complete it.

- [ ] **Step 2: Run the integration test**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/ -run TestCommandCompletionIntegration -v -timeout 60s`
Expected: PASS

- [ ] **Step 3: Run all tests**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make test`
Expected: all PASS

- [ ] **Step 4: Build final binary**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && make build`
Expected: builds successfully

- [ ] **Step 5: Commit**

```bash
git add internal/ui/completion_integration_test.go
git commit -m "test(ui): add integration test for command argument completion

exercises full completion flow with a live tmux server: type command
with flag, verify dropdown shows session name, tab-complete it."
```
