# Command Prompt Help Text Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add checked-in tmux command help metadata and use it to show a summary under the command prompt plus aligned flag/parameter descriptions in the completion popup.

**Architecture:** A new `internal/cmdhelp` package will hold native Go help metadata generated from `~/git_tree/tmux/command-summary.md` by a repo-local tool. The UI will consume only that Go package: the prompt view will render the current command summary under the input line, and command argument completion rows will render a second aligned description column when metadata exists.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, go/format, standard library only.

**Spec:** `docs/superpowers/specs/2026-04-04-command-help-text-design.md`

---

## File Structure

### New files

| File | Responsibility |
|---|---|
| `internal/cmdhelp/data.go` | Checked-in native Go help dataset keyed by command name |
| `internal/cmdhelp/data_test.go` | Lookup and structure tests for the checked-in dataset |
| `cmd/gen_command_help/main.go` | One-shot generator that parses `command-summary.md` and emits `internal/cmdhelp/data.go` |
| `cmd/gen_command_help/main_test.go` | Parser and deterministic generation tests |

### Modified files

| File | What changes |
|---|---|
| `internal/ui/model.go` | Add command-help metadata to the model |
| `internal/ui/commands.go` | Wire help lookup alongside command preload |
| `internal/ui/completion.go` | Support optional per-item descriptions and two-column rendering |
| `internal/ui/input.go` | Expose current command summary lookup for prompt rendering |
| `internal/ui/view.go` | Render the summary line under the prompt |
| `internal/ui/completion_test.go` | Add popup rendering tests with descriptions and plain value rows |
| `internal/ui/input_test.go` | Add current-command summary tests |
| `internal/ui/view_test.go` | Add prompt help line rendering tests |

---

### Task 1: Add The Checked-In Help Dataset And Generator

**Files:**
- Create: `cmd/gen_command_help/main.go`
- Create: `cmd/gen_command_help/main_test.go`
- Create: `internal/cmdhelp/data.go`
- Create: `internal/cmdhelp/data_test.go`

- [ ] **Step 1: Write the failing parser test**

```go
func TestParseCommandSummaryMarkdown(t *testing.T) {
	input := strings.TrimSpace(`
command: move-window
command-summary: move a window to another position
command-args:
-r renumber windows
-a insert after target window
-s source window
-t destination window
`)

	commands, err := parseCommandSummary(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	help := commands["move-window"]
	if help.Summary != "move a window to another position" {
		t.Fatalf("expected summary, got %q", help.Summary)
	}
	if len(help.Args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(help.Args))
	}
	if help.Args[2].Name != "-s" || help.Args[2].Description != "source window" {
		t.Fatalf("unexpected arg help: %+v", help.Args[2])
	}
}
```

- [ ] **Step 2: Run the parser test to verify it fails**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go test ./cmd/gen_command_help -run TestParseCommandSummaryMarkdown -v`
Expected: FAIL because `parseCommandSummary` does not exist yet

- [ ] **Step 3: Write the minimal parser and generator**

Implement:
- `type CommandHelp struct { Summary string; Args []ArgHelp }`
- `type ArgHelp struct { Name string; Description string }`
- `parseCommandSummary(io.Reader) (map[string]CommandHelp, error)`
- deterministic Go emission for `internal/cmdhelp/data.go`

- [ ] **Step 4: Add the checked-in dataset test**

```go
func TestMoveWindowHelpExists(t *testing.T) {
	help, ok := Commands["move-window"]
	if !ok {
		t.Fatal("expected move-window help")
	}
	if help.Summary == "" {
		t.Fatal("expected move-window summary")
	}
}
```

- [ ] **Step 5: Run generator and verify tests pass**

Run:
- `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go test ./cmd/gen_command_help ./internal/cmdhelp -v`
- `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go run ./cmd/gen_command_help`

Expected:
- tests PASS
- `internal/cmdhelp/data.go` contains deterministic Go data for the tmux commands

- [ ] **Step 6: Commit**

```bash
git add cmd/gen_command_help/main.go cmd/gen_command_help/main_test.go internal/cmdhelp/data.go internal/cmdhelp/data_test.go
git commit -m "feat(cmdhelp): add generated tmux command help data"
```

---

### Task 2: Render Prompt Summary Help

**Files:**
- Modify: `internal/ui/model.go`
- Modify: `internal/ui/commands.go`
- Modify: `internal/ui/input.go`
- Modify: `internal/ui/view.go`
- Modify: `internal/ui/input_test.go`
- Modify: `internal/ui/view_test.go`

- [ ] **Step 1: Write the failing prompt help test**

```go
func TestCurrentCommandSummaryUsesResolvedCommand(t *testing.T) {
	m := NewModel(ModelConfig{})
	node, _ := m.registry.Find("command")
	lvl := newLevel("command", "command", []menu.Item{
		{ID: "move-window", Label: "move-window [-adpr] [-s src-window] [-t dst-window]"},
	}, node)
	lvl.SetFilter("move-window -t ", len([]rune("move-window -t ")))
	m.stack = []*level{lvl}

	if got := m.currentCommandSummary(); got == "" {
		t.Fatal("expected summary for move-window")
	}
}
```

- [ ] **Step 2: Run the prompt help test to verify it fails**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go test ./internal/ui -run 'TestCurrentCommandSummaryUsesResolvedCommand|TestViewShowsCommandSummaryBelowPrompt' -v`
Expected: FAIL because summary lookup and rendering do not exist yet

- [ ] **Step 3: Implement summary lookup and rendering**

Implement:
- model field for command help lookup
- helper that resolves the current command from the selected item or typed input
- prompt-adjacent summary line rendered under the input when available

- [ ] **Step 4: Run the UI tests and verify they pass**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go test ./internal/ui -run 'TestCurrentCommandSummaryUsesResolvedCommand|TestViewShowsCommandSummaryBelowPrompt' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/commands.go internal/ui/input.go internal/ui/view.go internal/ui/input_test.go internal/ui/view_test.go
git commit -m "feat(ui): show command summary under prompt"
```

---

### Task 3: Render Described Command Arguments In The Completion Popup

**Files:**
- Modify: `internal/ui/completion.go`
- Modify: `internal/ui/completion_test.go`
- Modify: `internal/ui/input.go`

- [ ] **Step 1: Write the failing completion rendering test**

```go
func TestCompletionViewAlignsDescriptions(t *testing.T) {
	cs := newCompletionStateWithDetails([]completionItem{
		{Value: "-a", Label: "-a", Description: "insert after target window"},
		{Value: "-s", Label: "-s <src-window>", Description: "source window"},
		{Value: "-t", Label: "-t <dst-window>", Description: "destination window"},
	}, "", "", 0)

	view := ansi.Strip(cs.view(80, 10))
	if !strings.Contains(view, "-s <src-window>") {
		t.Fatalf("expected left column in view, got:\n%s", view)
	}
	if !strings.Contains(view, "destination window") {
		t.Fatalf("expected description in view, got:\n%s", view)
	}
}
```

- [ ] **Step 2: Run the completion rendering test to verify it fails**

Run: `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go test ./internal/ui -run 'TestCompletionViewAlignsDescriptions|TestCompletionViewLeavesPlainValuesUnchanged' -v`
Expected: FAIL because completion rows do not carry descriptions yet

- [ ] **Step 3: Implement described completion items**

Implement:
- a completion item type with `Value`, `Label`, and optional `Description`
- two-column popup rendering when any visible row has a description
- wiring from command flag/positional completion paths to the new descriptions
- plain rendering for live runtime candidates with no descriptions

- [ ] **Step 4: Run focused UI tests and then the package test suite**

Run:
- `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go test ./internal/ui -run 'TestCompletionViewAlignsDescriptions|TestCompletionViewLeavesPlainValuesUnchanged|TestCurrentCommandSummaryUsesResolvedCommand|TestViewShowsCommandSummaryBelowPrompt' -v`
- `cd /Users/matt/git_tree/tmux-popup-control/.worktrees/cmd-completion && GOCACHE=$(pwd)/.gocache go test ./internal/ui/... ./internal/cmdhelp ./cmd/gen_command_help -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/completion.go internal/ui/completion_test.go internal/ui/input.go
git commit -m "feat(ui): show command help in completion popup"
```

---

## Self-Review

- Spec coverage: the plan covers checked-in Go help data, prompt summary rendering, described command-argument popup rows, and tests for plain live value rows.
- Placeholder scan: no `TODO`, `TBD`, or task references without concrete files and commands remain.
- Type consistency: the same `CommandHelp`, `ArgHelp`, and described completion item structure is used across dataset generation and UI tasks.
