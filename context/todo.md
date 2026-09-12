## tmux next-3.9 follow-ups (from the 2026-09-12 upstream analysis)

Details and verification transcripts: `~/obsidian/agent-notes/tmux-popup-control/2026-09-12/tmux-next-39-impact-analysis.md`. The defects found there (and the five from 2026-08-17) were fixed on 2026-09-12; see `done.md`. What remains:

- gotmuxcc: `discoverAttachTarget` attaches the control client to the alphabetically-first session *by name*, which fails outright when that name contains `:` (noted in `internal/menu/ids_integration_test.go`, which keeps its weird session sorted last). Needs an upstream fix to attach by `$N`; do not work around it here.
- Resurrect: a v1 (pre-json) save of a window that had a floating pane still cannot apply its layout on restore; it is now a warning rather than an abort. Only re-saving on a tmux with json layouts fixes such a file.
- `new-pane -O -D -K -A` is now a credible `display-popup` replacement (tmux `d202aa7a` added `-D` and `remain-on-exit failed-key` "for better compatibility with popups"); revisit `main.sh` once modal panes settle.
- Opportunity: gate pane previews / the pane poller on `#{pane_output_generation}` and `#{history_generation}` to skip unchanged captures.

## Command argument tab completion (feature/cmd-completion branch)

Remaining tasks from `docs/superpowers/plans/2026-04-02-command-argument-completion.md`:

- None. Tasks 1 through 14 are complete on `feature/cmd-completion`, and the immediate post-implementation bugfixes from user testing are also merged.
- None. The follow-up help-text/spec work from `docs/superpowers/specs/2026-04-04-command-help-text-design.md` and `docs/superpowers/plans/2026-04-04-command-help-text.md` is also implemented and verified.

Completed tasks: 1 (schema types), 2 (synopsis parser), 3 (input analyser), 4 (value resolver), 5 (completion styles), 6 (completion dropdown widget), 7 (schema registry wiring), 8 (data source adapter), 9 (completion triggering/key routing), 10 (ghost hint extension), 11 (dropdown overlay rendering), 12 (harness tests), 13 (polish/edge cases), 14 (live integration test).
Recent follow-up fixes:
- `c0ddb85` — render the completion dropdown below the prompt when there is not enough room above it.
- `6877b8b` — preserve dropdown selection across backend refreshes and keep `Esc`-dismissed completion suppressed until the input text changes.
- `d8942e2` — keep command-menu filtering scoped to the command token and make `Tab` replace the current command token under the cursor.
- `c76301e` — add a repo-local generator plus checked-in native Go command help data from `~/git_tree/tmux/command-summary.md`. (Superseded by `f9cc2d2`: the generator and generated data are gone; command help now comes from the catalog embedded in `internal/tmuxopts`.)
- `9317f12` — show command summaries under the prompt and render aligned argument descriptions in the completion popup.
- Uncommitted follow-up: suppress exact-match value dropdowns and treat `move-window -r -t` as a session target so direct execution keeps working in the real tmux flow.
Full spec: `docs/superpowers/specs/2026-04-02-command-argument-completion-design.md`

## gotmuxcc upstream reports — RESOLVED in v0.2.0 (2026-08-23)

All three findings raised from `feat-watcher-fetch-context` are fixed and adopted on that branch:

- `Tmux.Close()` race — fixed more cleanly than proposed (`11bfb6d`): the fields are not cleared at all, since `router.close()`'s `failAll` already fails every pending and in-flight request. `Close` is idempotent via `closeOnce` and safe alongside a command in flight.
- Per-command context API (`cbf3afb`) — 8 additive `*Context` methods; existing signatures delegate through `context.Background()`.
- Handshake wait (`c00feb1`) — `DefaultHandshakeTimeout` (10s) + `WithHandshakeTimeout`.

## new gotmuxcc request (raised 2026-08-23, not yet filed)

- No `ListClientsContext` and no `DisplayMessageContext`. Both are on the hot fetch path — `ListClients` via `realAttachedClients` and `currentSessionName`, `DisplayMessage` via `popupSessionName` — and both run near the end of every poll cycle. They are the sole reason `internal/backend`'s `awaitFetch` abandon backstop still exists; with context variants for these two it could be deleted outright and cancellation would be pure propagation. Deliberately NOT worked around locally via `CommandContext`.

## adjacent, not fixed

- `newTmux` (`internal/tmux/types.go`) holds `clientMu` across the gotmuxcc dial + control-mode handshake. v0.2.0's `DefaultHandshakeTimeout` (10s) downgrades this from an infinite deadlock to a bounded 10s stall of `tmux.Shutdown()`, so it is no longer a hang — but a 10s freeze on popup exit is still user-visible. Fixing it properly means dialling outside the lock, or passing `WithHandshakeTimeout` a shorter bound suited to a popup. Not addressed on this branch.
