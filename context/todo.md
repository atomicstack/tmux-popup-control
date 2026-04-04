## Command argument tab completion (feature/cmd-completion branch)

Remaining tasks from `docs/superpowers/plans/2026-04-02-command-argument-completion.md`:

- **Task 4**: Value Resolver — create `internal/cmdparse/resolve.go` + `resolve_test.go`. DataSource interface, StoreResolver mapping arg types to live data, FlagCandidates for unused flag listing. (not started)
- **Task 6**: Completion Dropdown Widget — create `internal/ui/completion.go` + `completion_test.go`. completionState struct with filtering, cursor, selection, ghost hints, dropdown rendering. (not started)
- **Task 7**: Wire Schema Registry into Model — add `commandSchemas` and `completion` fields to Model, build registry in `handleCommandPreloadMsg`. (not started)
- **Task 8**: Data Source Implementation — create `internal/ui/completion_datasource.go` adapting state stores to the cmdparse.DataSource interface. (not started)
- **Task 9**: Completion Triggering — modify `input.go` + `navigation.go` to trigger completion analysis on keystrokes and route keys through dropdown when visible. (not started)
- **Task 10**: Ghost Hint Extension — extend `autoCompleteGhost()` in `input.go` for argument hints (dropdown selection, type label, unique prefix match). (not started)
- **Task 11**: Dropdown Overlay Rendering — modify `view.go` to render dropdown above prompt in both layout modes. (not started)
- **Task 12**: Harness Tests — end-to-end key sequences through the test harness. (not started)
- **Task 13**: Polish and Edge Cases — backspace through completed values, window resize, regression check. (not started)
- **Task 14**: Integration Test — live tmux test of full completion flow. (not started)

Completed tasks: 1 (schema types), 2 (synopsis parser), 3 (input analyser), 5 (completion styles).
Full spec: `docs/superpowers/specs/2026-04-02-command-argument-completion-design.md`
