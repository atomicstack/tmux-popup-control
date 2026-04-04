## Current state

Here’s what’s happened so far:

- Command argument tab completion (completed on `feature/cmd-completion`):
  - Task 14 complete and feature implementation finished.
  - Design spec written: `docs/superpowers/specs/2026-04-02-command-argument-completion-design.md`
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
