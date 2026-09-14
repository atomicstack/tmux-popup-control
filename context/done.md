## Current state

Here’s what’s happened so far:

- macos release binaries are signed and notarised (2026-09-14, `1f8466f`): `make release` gained opt-in `codesign-darwin` / `notarize-darwin` targets, gated on `SIGN_IDENTITY` and `NOTARY_PROFILE` so the unsigned path is unchanged when they are unset.
  - signing uses `--options runtime --timestamp`; the notary service refuses submissions without a hardened runtime or secure timestamp, and refuses an "Apple Development" certificate outright — it must be a "Developer ID Application" identity.
  - apple only staples tickets to `.app`/`.dmg`/`.pkg`, never to a bare executable, so the binaries ship unstapled and gatekeeper resolves the ticket online. `spctl -a -t exec` reports "rejected (the code is valid but does not seem to be an app)" for a bare CLI binary — that is the policy declining to assess a non-bundle, not a signing failure. the real checks are `codesign --verify --strict`, the notarytool log, and the cdhash comparison below.
  - v0.17.1 was re-cut with signed binaries (the github release was deleted and recreated; the tag was untouched). submission `c52dbf17` accepted, "ready for distribution", no issues, tickets for both arches. the cdhashes apple ticketed match the binaries in the published tarballs (arm64 `a96016bee67fe993`, x86_64 `393ec3b6f0cdcab2`), so the shipped bytes are the notarised bytes. darwin checksums changed as a result; `checksums.txt` on the release is authoritative.

- gotmuxcc updated to v0.4.0 (2026-09-13, `eb01ba9`): six upstream fixes, two of which change behaviour this app depends on.
  - `discoverAttachTarget` lists `#{session_id}` instead of `#{session_name}`, so the control connection no longer fails when the alphabetically-first session has a `:` or `.` in its name (tmux relaxed name validation in `166267c8`; `attach-session` reads those as the `session:window.pane` separators). The `internal/menu` weird-session fixture dropped its workaround — it was named `zz:weird` to sort *last* and dodge the bug, and is now `aa:weird` so it sorts first and exercises the fixed path.
  - `Options()` passes `-H` to `show-options`. Since tmux `7277712c`, `show-options` hides `@`-prefixed user options that are registered as hooks unless `-H` is given, so `tmux.UserOptions` silently dropped every `@`-option declared with `set-hook` and the command-prompt completion dropdown lost them too. New `TestUserOptionsIncludesHookRegisteredOptionsIntegration` covers it: 1 of 2 options against v0.3.0, both against v0.4.0.
  - The remaining four are internal and needed no changes here: positional `%layout-change` parsing (empty raw-flags and empty layout fields are no longer lost), hook guard blocks are never paired with a queued request, arguments tmux would lex as a conditional keyword (`%0:on`) are quoted, and flag docs corrected.
  - No API removals. The new `WindowLayout*` constants (`main-vertical` and the mirrored layouts) and the documented `new-layouts` control flag are additive; our `client.Options`/`SetControlFlags` call sites compile unchanged. `make test` and `make build` green.

- Released v0.17.0 (2026-09-12, tag on `5ba5196`): tmux next-3.9 support (json layouts, floating-pane resurrect, theme colour swatches, id-based targets), catalog-driven command completion, gotmuxcc v0.3.0, and the merged lifecycle/restore-safety/plugin-install work. `make release` now accepts `RELEASE_NOTES=<file>`; README bumped. Notes: https://github.com/atomicstack/tmux-popup-control/releases/tag/v0.17.0

- tmux next-3.9 follow-ups implemented (2026-09-12, `2f27883`..`b80e126`):
  - merged `feat-watcher-fetch-context` (rebased, fast-forward): gotmuxcc v0.2.0 (fixes the P0 control-mode framing bug, exposes floating/modal pane formats, per-command contexts, bounded handshake), context-aware `Fetch*`/`ShowOption`, bounded watcher shutdown; `vendor/` re-vendored offline from the module cache
  - json layouts: the shared control client sets the `new-layouts` flag (separate `refresh-client` call, ignored by older tmux); `selectableLayout` handles the v2 `{"V":2,"L":{...}}` form by stripping the `"I"` pane-id keys; `tmux.Pane` carries `Floating`/`X`/`Y`/`Z` and `tmux.Window` carries `Zoomed`
  - resurrect: floating panes are marked in the save file (format version 3) and come back through the json layout (tmux converts a split pane into a floater when the cell says so); a layout tmux cannot apply is a warning, not an abort; `TestSaveRestoreFloatingPaneIntegration` verifies the round trip on tmux next-3.9 and skips on older builds
  - `window:layout`: escape re-zooms a window that was zoomed before the preview unzoomed it
  - `showPopup` refuses an empty client name (tmux ≥ `af3e4d2e` silently ignores popups aimed at a control client)
  - cmdparse builds schemas catalog-first from `argument_template`/`flags[]` (`BuildCatalogRegistry`), with the synopsis parser as fallback; digit bool clusters parse (`list-keys`, `command-prompt`, `send-prefix`), optional-value flags (`resize-pane -D`) no longer demand a value, `new-pane -e` and `refresh-client -B` are repeatable
  - theme colours: `tmux.ResolveThemeColour` resolves `themeblue`-style names through `#{c/f:…}` on the user's tty client (cached per socket/client/name); `#` values must be `#rrggbb`, `#{…}`/`#[…]` are never colours, quoted style values are unquoted, `#,` escapes and nested formats survive tokenisation, `name[N]` array keys are looked up; `TestShowOptionsThemeColourSwatchIntegration` drives the real binary
  - ids everywhere: `Session.ID`, `Window.SessionID`, `Pane.SessionID`/`WindowID` are fetched as dedicated format fields; every menu action resolves its display id to the entry and targets `$N`/`@N`/`%N` (`internal/menu/refs.go`); tree items are `tree:s:$N`/`tree:w:@N`/`tree:p:%N`; previews capture by `%N`; `SwitchPane` takes a `PaneRef`; `NewSession` returns the id; `TestActionsTargetSessionWithColonInNameIntegration` runs against a session named `zz:weird`
  - process: three subagents ran in parallel worktrees; two were cut off by a rate limit and their work (theme colours, ids) was finished by hand from the tests they had written

- Option catalog refreshed for tmux next-3.9 and upstream impact analysis (2026-09-12, `3cc4f9a`):
  - embedded catalog refreshed from option-catalog `647a917` + its uncommitted 2026-09-12 regeneration (tmux `e880cf63`): 272 options (+`clear-on-attach`), `remain-on-exit failed-key`, `pane-border-lines rounded`, `utf8` terminal feature, `capture-pane -I`, `display-message -j`, `new-pane -A/-D/-K`, and the `history_*`/`pane_output_generation` formats; command help diffed, no description regressions
  - analysed tmux `851c5a933d..e880cf63` (155 commits) with live verification on a scratch HEAD build; full `make test` passes against tmux next-3.9
  - findings: layout strings are JSON v2 but control clients keep v1 (tiled-only, now round-trippable, so the floating-pane `window:layout` defect is mitigated); `display-popup` targeted at a control client is a silent no-op; resurrect restore aborts for windows that had a floating pane ("have 3 panes but need 2"); gotmuxcc v0.2.0 fixes the P0 router framing bug and `main` still vendors v0.1.4
  - write-up: `~/obsidian/agent-notes/tmux-popup-control/2026-09-12/tmux-next-39-impact-analysis.md`; action items in `todo.md`

- Option catalog refreshed to schema v2 and command help re-sourced from it (2026-08-17, `f9cc2d2`):
  - embedded catalog refreshed from option-catalog `647a917` (tmux `851c5a933d`): 271 options — 21 new pane/window/client lifecycle hooks, `after-queue` removed, `display-panes-border-style` and `copy-mode-current-line-style` added
  - schema v2 adds structured command data (`usage`, `argument_template`, `description`, `after_hook_name`, `positional_arguments`, per-flag `value_mode`/`value_name`/`description`) — modelled on `TmuxCommandEntry`/`TmuxCommandFlag`; the new top-level `events` block (91 events with payload/`hook_*` format metadata) is deliberately left unmodelled until a consumer needs it
  - `cmd/gen_command_help` and the generated `internal/cmdhelp/data.go` deleted; `cmdhelp.Commands()` now builds from `tmuxopts.Default()` on first use, so a catalog refresh is the whole update
  - command help gained `new-pane` and `switch-mode`, and the corrected `display-panes` flags (`-b` is gone in tmux next-3.8; `-k`/`-s`/`-Z` were missing from the old snapshot)
  - eight description regressions found by diffing the old generated data against the catalog were fixed upstream in `tmux-command-metadata.json` rather than patched locally: scope words restored to the `set-option`/`show-options` summaries, and ACL/SIGHUP/LF/UTF-8/Enter un-lowercased

- Flaky completion Enter harness test fixed (2026-07-11):
  - added `Harness.Update` for tests that need to inspect synchronous model state without executing the returned `tea.Cmd`
  - changed `TestCompletionEnterExecutesInsteadOfAccepting` to verify Enter returns a command while asserting the synchronously-set loading and pending state before the command result is drained
  - added direct coverage that `Harness.Update` returns commands without executing them, removing the test's dependency on the harness's 10 ms timer-command timeout

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

## extract feature — extrakto mvp (feature/extract-extrakto-mvp branch, 2026-07-08)

Extrakto-style token extractor. Spec/plan in obsidian vault (tmux-popup-control/2026-07-07/extract-extrakto-mvp{,-plan}.md). Subagent-driven-development; every task spec+quality reviewed, final whole-branch review clean (ready to merge). Full `make test` green (22 pkgs), binary builds.

- `internal/extract/` — pure engine (no bubbletea/tmux/menu imports): `Category` enum (word/path/url/quote/s-quote/line/all) + cycle order via `Categories()`; `Extract(text, cat) []Token`. RE2 patterns ported verbatim from `~/.tmux/plugins/extrakto/extrakto.conf`; `"\n"+text` prefix, minLen 5, lstrip/rstrip/exclude, dedup then reverse; `line` fast-path; `All`=union(path,url,quote,s-quote) excluding word+line.
- `internal/tmux/extract.go` + `host.go` — `OriginPaneID()`, `CaptureVisible()` (capture-pane -pJ visible screen), `InsertText()` (set-buffer + paste-buffer -p), `CopyText()` (set-buffer only; buffer-only, OSC-52 deferred).
- `internal/menu/` — `Context.ExtractCategory`; `loadExtractMenu` (item ID==Label==token text); `extract` registered as root category (first item) + in CategoryLoaders + markMultiSelect; `SetExtractCaptureForTest` seam.
- `internal/ui/extract.go` (+ model/commands/navigation/view) — token list is a normal Level (reuses fuzzy filter/multiselect/render); ctrl-f cycles category in place (async, seq-guarded, filter preserved, category reset to word on entry); header via Level.Subtitle (theme styles, raw:true); enter=insert into origin pane, ctrl-y=copy to buffer; multi-select join newline for all/line else space; quit-on-escape for direct invocation.
- Tests: engine unit (ported extrakto corpus), UI harness (cycle/insert/copy/escape/multiselect/seq-guard), live-tmux integration (`internal/tmux`: capture→extract→insert paste-landed). Regenerated `testdata/capture/root_menu.txt` golden for the new extract root item.
- Deferred (inventory in spec §9): grab-area cycle, edit/open actions, clip-mode/OSC-52 system clipboard, alt-variants, prefix-name, @extrakto-* config compat.

## bounded watcher shutdown — context through the fetch path (feat-watcher-fetch-context, 2026-08-23)

Fixed a shutdown hang: `internal/backend/watcher.go` threaded its context into the throttle but dropped it before the tmux call, so a wedged control-mode connection pinned a poller forever, `Stop()` did nothing, and `app.Run`'s ordered teardown blocked on `Wait()` — the popup never exited. Design note in the obsidian vault (`tmux-popup-control/2026-08-23/feat-watcher-fetch-context.md`).

- `internal/tmux/snapshots.go` — added `FetchSessionsContext` / `FetchWindowsContext` / `FetchPanesContext`; the old names are now `context.Background()` wrappers so `internal/resurrect` (which holds them as `func(string) (T, error)` values) is untouched. ctx reaches an entry check, one interstitial check after the primary list call, and the exec-based helpers (`fetchSessionsFallback`, `envOrOption` → `ShowOptionContext`, `fetchWindowLines`, `fetchPaneLines`).
- `internal/tmux/restore.go` — `ShowOptionContext` on `runExecCommandContext`; cache behaviour unchanged.
- `internal/backend/watcher.go` — `fetchFunc` gains a ctx; `poll` now goes through `awaitFetch`, which runs the fetch on its own goroutine and, once ctx is cancelled, waits only `fetchAbandonGrace` (250ms) before abandoning the result.
- `internal/app/app.go` — teardown comment restated: `Wait()` is bounded now, and the `tmux.Shutdown()` that follows is what reclaims an abandoned fetch (gotmuxcc's router fails every pending/in-flight request on close).

**Honest scope:** this does not cancel the underlying gotmuxcc call — that API takes no context — so it converts an unbounded hang into a ~250ms drain plus a transiently-detached goroutine. Deliberately not worked around with exec hacks; reported upstream instead.

Tests: `TestWatcherStopDrainsPromptlyThroughWedgedFetch` (fails at its 2s deadline before the fix, passes in ~250ms after), `TestPollAwaitsInFlightFetchWithinGrace`, `TestAwaitFetchAbandonsWedgedFetchWithCancelledContext`, `TestStartForwardsWatcherContextToFetch`, plus `internal/tmux/snapshots_context_test.go`. `internal/backend` green under `-race`. Full `make test` green except the pre-existing `TestTreeFilterShowsOnlyMatchingItems` failure in `internal/testutil`, verified failing identically on unmodified `main`.

### follow-up: adopted gotmuxcc v0.2.0 and rethreaded onto the real context api (2026-08-23)

`chore(deps)` bumped gotmuxcc to v0.2.0, which fixed all three findings this branch reported upstream (unsynchronised `Tmux.Close()`, no per-command context api, unbounded constructor handshake) plus a router framing bug affecting our `CapturePane` preview path and a `send on closed channel` panic on a library-owned goroutine.

- `internal/tmux/types.go` / `tracing.go` — `tmuxClient` and `tracedTmuxClient` gained the six context-aware list operations; traced wrappers reuse the existing span names so traces stay comparable.
- `internal/tmux/{snapshots,labels,host}.go` — the fetchers now call `ListSessionsContext` / `ListAllWindowsContext` / `ListAllPanesContext` / `List*FormatContext`, with ctx carried through `fetchSessionLabels`, `currentSessionName`, `fetchWindowLines`, `fetchPaneLines`.
- `fakeClient`'s context variants delegate to the existing stubs, so every existing fixture kept working.

**The `awaitFetch` backstop survived, deliberately.** gotmuxcc has no `ListClientsContext` and no `DisplayMessageContext`, and both run near the end of every fetch (`realAttachedClients`, `currentSessionName` → `popupSessionName`). A wedge in either would pin a poller exactly as the uncancellable list calls once did, so removing `awaitFetch` reopens the hang through a narrower door — verified: `TestWatcherStopDrainsPromptlyThroughWedgedFetch` still fails at its 2s deadline without it. The **grace window's** original justification (closing the client under an in-flight fetch was unsafe) is obsolete now that `Close` is idempotent; it is kept on the narrower ground that the two remaining uncancellable calls are fast, so letting them land beats detaching a goroutine whose result is discarded.

Cancellation is caller-side by design in v0.2.0: an already-written command stays in the router's pending queue and its reply is discarded on arrival. That is what makes abandoning safe rather than lossy.

Verified: `internal/{backend,tmux,resurrect,ui,menu}` green under `-race`; full `make test` green except the pre-existing `TestTreeFilterShowsOnlyMatchingItems`; `make build` produced the binary.
