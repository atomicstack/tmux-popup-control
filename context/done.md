## Current state

Here’s what’s happened so far:

- Test-overhaul follow-up (uncommitted on `codex-test-overhaul-2026-04-12`):
  - strengthened weak assertions in `internal/cmdhelp` and `internal/tmuxopts`
  - added direct unit coverage for `internal/app`, `internal/backend`, `internal/data/dispatcher`, `internal/format/table`, `internal/shquote`, `internal/state`, `internal/ui/command`, and extra `main.go` startup paths
  - fixed two test-exposed issues: `internal/format/table.Format` no longer panics on ragged rows, and `state.SessionStore` now deep-clones nested client slices

- Command argument tab completion (completed on `feature/cmd-completion`):
  - Task 14 complete and feature implementation finished.
  - Follow-up user-testing fixes landed after the main feature commit:
    - `c0ddb85` — dropdown now renders below the prompt when there is insufficient room above it.
    - `6877b8b` — backend refreshes no longer reset the completion selection or re-open an `Esc`-dismissed dropdown; dismissal now persists until text changes.
    - `d8942e2` — command-menu filtering now matches only the command token, so arguments do not empty the command list, and `Tab` replaces the current command token under the cursor.
  - Follow-up help-text work landed after the completion feature:
    - `a03e194` — added a follow-up spec and implementation plan for checked-in command help text and popup descriptions.
    - `c76301e` — added `cmd/gen_command_help` and generated `internal/cmdhelp/data.go` from `~/git_tree/tmux/command-summary.md`.
    - `9317f12` — wired command summaries into the prompt view and rendered aligned description columns for command flag completion rows while keeping live value candidates plain.
    - Additional uncommitted verification fix: `move-window -r -t` now completes sessions instead of window labels, and exact-match value completions dismiss the dropdown so `Enter` can execute the typed command in integration flows.
  - Design spec written: `docs/superpowers/specs/2026-04-02-command-argument-completion-design.md`
  - Follow-up help-text spec written: `docs/superpowers/specs/2026-04-04-command-help-text-design.md`
  - Follow-up help-text plan written: `docs/superpowers/plans/2026-04-04-command-help-text.md`
  - Implementation plan written: `docs/superpowers/plans/2026-04-02-command-argument-completion.md` (14 tasks)
  - Task 1 complete: `internal/cmdparse/schema.go` — type definitions (CommandSchema, ArgFlagDef, PositionalDef, CompletionContext, ContextKind)
  - Task 2 complete: `internal/cmdparse/parse.go` + `parse_test.go` + golden file — synopsis parser that handles bool flag clusters, arg flags, positional args (required/optional/variadic), aliases, nested optionals. BuildRegistry indexes by name+alias.
  - Task 3 complete: `internal/cmdparse/analyse.go` + `analyse_test.go` — input analyser walks tokens to determine completion context (command name, flag name, flag value, positional value), tracks used flags.
  - Task 4 complete: `internal/cmdparse/resolve.go` + `resolve_test.go` — resolver for sessions/windows/panes/commands plus unused-flag candidate generation.
  - Task 5 complete: `internal/theme/theme.go` — added CompletionBorder, CompletionItem, CompletionSelected styles.
  - Task 6 complete: `internal/ui/completion.go` + `completion_test.go` — dropdown state, filtering, selection, labeled rendering, ghost hint helpers.
  - Task 7 complete: `internal/ui/model.go` + `internal/ui/commands.go` — command schema registry and completion state wired into the model/preload path.
  - Task 8 complete: `internal/ui/completion_datasource.go` — Model-backed data source adapter for completion resolution.
  - Task 9 complete: `internal/ui/input.go` + `internal/ui/navigation.go` — per-keystroke completion analysis, dropdown open/close, accept, and key routing.
  - Task 10 complete: `internal/ui/input.go` — argument-aware ghost hints layered on top of existing command-name ghost completion.
  - Task 11 complete: `internal/ui/view.go` + `internal/ui/view_test.go` — dropdown overlay rendered above the prompt in both layout modes.
  - Task 12 complete: `internal/ui/completion_harness_test.go` — harness coverage for trigger, filtering, navigation, tab accept, escape dismiss, and resize behavior.
  - Task 13 complete: `internal/ui/backend.go` + `internal/ui/input_test.go` — resize dismissal, command-name regression coverage, and backend-driven dropdown refresh when live data arrives after typing starts.
  - Task 14 complete: `internal/ui/completion_integration_test.go` — live tmux test verifying dropdown candidates appear from real session data and tab inserts the resolved target.
