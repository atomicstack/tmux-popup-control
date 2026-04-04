## Command argument tab completion (feature/cmd-completion branch)

Remaining tasks from `docs/superpowers/plans/2026-04-02-command-argument-completion.md`:

- None. Tasks 1 through 14 are complete on `feature/cmd-completion`, and the immediate post-implementation bugfixes from user testing are also merged.
- None. The follow-up help-text/spec work from `docs/superpowers/specs/2026-04-04-command-help-text-design.md` and `docs/superpowers/plans/2026-04-04-command-help-text.md` is also implemented and verified.

Completed tasks: 1 (schema types), 2 (synopsis parser), 3 (input analyser), 4 (value resolver), 5 (completion styles), 6 (completion dropdown widget), 7 (schema registry wiring), 8 (data source adapter), 9 (completion triggering/key routing), 10 (ghost hint extension), 11 (dropdown overlay rendering), 12 (harness tests), 13 (polish/edge cases), 14 (live integration test).
Recent follow-up fixes:
- `c0ddb85` — render the completion dropdown below the prompt when there is not enough room above it.
- `6877b8b` — preserve dropdown selection across backend refreshes and keep `Esc`-dismissed completion suppressed until the input text changes.
- `d8942e2` — keep command-menu filtering scoped to the command token and make `Tab` replace the current command token under the cursor.
- `c76301e` — add a repo-local generator plus checked-in native Go command help data from `~/git_tree/tmux/command-summary.md`.
- `9317f12` — show command summaries under the prompt and render aligned argument descriptions in the completion popup.
- Uncommitted follow-up: suppress exact-match value dropdowns and treat `move-window -r -t` as a session target so direct execution keeps working in the real tmux flow.
Full spec: `docs/superpowers/specs/2026-04-02-command-argument-completion-design.md`
