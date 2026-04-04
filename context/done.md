## Current state

Here’s what’s happened so far:

- Command argument tab completion (in progress, feature/cmd-completion branch):
  - Design spec written: `docs/superpowers/specs/2026-04-02-command-argument-completion-design.md`
  - Implementation plan written: `docs/superpowers/plans/2026-04-02-command-argument-completion.md` (14 tasks)
  - Task 1 complete: `internal/cmdparse/schema.go` — type definitions (CommandSchema, ArgFlagDef, PositionalDef, CompletionContext, ContextKind)
  - Task 2 complete: `internal/cmdparse/parse.go` + `parse_test.go` + golden file — synopsis parser that handles bool flag clusters, arg flags, positional args (required/optional/variadic), aliases, nested optionals. BuildRegistry indexes by name+alias.
  - Task 3 complete: `internal/cmdparse/analyse.go` + `analyse_test.go` — input analyser walks tokens to determine completion context (command name, flag name, flag value, positional value), tracks used flags.
  - Task 5 complete: `internal/theme/theme.go` — added CompletionBorder, CompletionItem, CompletionSelected styles.
