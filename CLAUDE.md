# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview:
This tool is called tmux-popup-control. It is a Golang port and improvement of
the tmux-fzf (https://github.com/sainnhe/tmux-fzf) plugin for tmux. This tool
uses Bubble Tea and Lip Gloss from the Charm project for its TUI and styling,
gotmuxcc by @atomicstack as a tmux control-mode client library, and Fuzzy
Search by @lithammer for dealing with user input. It also absorbs the roles of
tpm (plugin management), tmux-resurrect (session save/restore) and extrakto
(token extraction from pane text).

## Project context

The `context/` directory tracks ongoing work:
- `context/context.md` — project overview and design goals
- `context/todo.md` — pending work items
- `context/done.md` — completed work log

Check `todo.md` and `done.md` at the start of sessions to understand current state. Update them after completing work (edit in the working tree; they are not committed as part of feature work).

**Design goals:** colourful, dynamic, modern feel; fast and async (goroutines for background work); code that is clear, concise, and modular (logic in sub-packages, not dumped into the model); comprehensive tests kept in sync with the code.

**Key principle:** use the gotmuxcc persistent control-mode connection for all tmux operations. Direct `tmux` exec (`runExecCommand`) is a last resort for operations not yet exposed by gotmuxcc. Never work around gotmuxcc bugs — report them so they can be fixed upstream.

**Target tmux HEAD only.** The project tracks tmux `next-3.9` (the revision is named in README.md). Do not add compatibility guards for older tmux; design for the latest semantics (json v2 layouts, floating panes, `:`/`.` allowed in session and window names, popups implemented as modal floating panes).

**Do not hand-roll code when a vendored dependency already provides the functionality.** Before writing low-level helpers (ANSI escape codes, string manipulation, formatting), check what `lipgloss`, `charmbracelet/x/ansi`, `bubbletea`, and other vendored libraries expose. For example, use `ansi.Style.ForegroundColor()` instead of emitting raw `\x1b[38;5;…m` sequences.

## Commands

The repository uses a Makefile that keeps Go build artifacts inside the workspace (`.gocache/`, `.gomodcache/`) and sets `GOPROXY=off` for offline builds. Always use `make` targets rather than raw `go` commands:

```sh
make build           # builds ./tmux-popup-control
make build TAGS=sqlitedebug  # TAGS threads go build tags into build/run/test/cover; sqlitedebug compiles in --debug-to-sqlite (modernc.org/sqlite), never in release builds
make run             # runs the application
make test            # runs all tests (isolated from the user's tmux server, -p 4)
make test-trace      # same, with gotmuxcc command tracing → ./gotmuxcc_trace.log (~4x slower)
make cover           # runs tests with coverage report
make fmt             # gofmt -w .
make tidy            # go mod tidy
make clean-cache     # removes .gocache/ and .gomodcache/
make update-gotmuxcc # fetches latest gotmuxcc + re-vendors (online)
make update-deps     # updates all deps to latest (go get -u) + re-vendors (online)
make release         # cross-compiles, packages tarballs + checksums, creates GitHub release via gh
```

**Test isolation:** `make test` wraps `go test` in `ISOLATED_TEST`, which unsets `TMUX`/`TMUX_PANE` and points `TMUX_TMPDIR` at an empty temp dir, so any code path that reaches a real `tmux` without an explicit socket fails with "no server running" instead of acting on the user's live sessions. `-p 4` caps package parallelism because four packages (`resurrect`, `testutil`, `tmux`, `ui`) spin up real tmux servers. Every package whose tests could reach tmux has a `TestMain` (`isolation_main_test.go` / `main_test.go`) calling `testutil.IsolateFromUserServer()` for the same reason — add one to any new package that imports `internal/tmux`.

To run a single test or package, reproduce the same isolation:
```sh
env -u TMUX -u TMUX_PANE TMUX_TMPDIR=$(mktemp -d) \
  GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off \
  go test ./internal/tmux/... -run TestFetchSessions
```

To update golden files in tests:
```sh
UPDATE_GOLDEN=1 make test
```

Integration tests (`internal/testutil/`, `internal/tmux/integration_test.go`, `internal/ui/*integration_test.go`, `internal/resurrect/integration_test.go`) require a live tmux binary and are skipped automatically when tmux is unavailable.

### Releasing

`make release` cross-compiles, packages tarballs (binary + `README.md`, `main.tmux`; macOS xattrs suppressed), generates checksums, and calls `gh release create`. The version comes from `git describe --tags`, so **tag first** (`git tag vX.Y.Z`), **push the tag** (`git push origin vX.Y.Z`), then run `make release VERSION=X.Y.Z`. The `VERSION` override ensures the ldflags match the tag even if `git describe` adds a suffix.

Every release needs a changelog: write a notes file and pass `RELEASE_NOTES=path/to/notes.md`; without it the target falls back to `--generate-notes`. Name the tmux commit the release was tested against (see the README header). Optional `SIGN_IDENTITY=…` / `NOTARY_PROFILE=…` trigger the `codesign-darwin` / `notarize-darwin` steps for the darwin binaries.

**Do not build binaries separately before running `make release`** — the target already handles cross-compilation. After tagging, bump the version header and tested-tmux revision in README.md.

### CLI subcommands

Besides the default TUI mode, `main.go` dispatches these via `commandHandlers()`:
- `save-sessions [--name NAME]` / `restore-sessions [--from NAME]` — resurrect save/restore with a progress popup.
- `autosave-now [--socket PATH]` — run by the `set-hook -B` monitor main.tmux installs (tmux schedules autosaves itself: it fires when `#{t/d:start_time}` passes `@tmux-popup-control-autosave-last` + the interval). Advances `-autosave-last` first, saves, prunes, and toggles `@tmux-popup-control-autosave-saving` (the status-line indicator).
- `install-and-init-plugins` — called from `main.tmux` at tmux startup; sources installed plugins, schedules a deferred popup for any uninstalled ones.
- `deferred-install` — background helper (`run-shell -b`) that waits for tmux startup, then opens the plugin install UI via `showPopup`.
- `--version`.

## Architecture

This is a [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI that wraps tmux as a popup control menu. Key bindings launch it with `run-shell -C '#{E:@tmux-popup-control-launch} …'`: main.tmux stores the whole `new-pane -O -K …` command in that option, tmux expands it (names escaped with `#{q/a:…}`) and runs it in-process, so the binary starts in a modal floating pane with no shell or tmux client in between. The launcher passes the host pane/session/client as `TMUX_POPUP_CONTROL_PANE_ID` / `_SESSION_ID` / `_CLIENT`. The app and communicates with tmux via the `gotmuxcc` library (vendored).

### Layer overview

```
main.go                   → config loading, signal handling, app.Run(), subcommands
internal/config/          → CLI flags + env vars → Config struct
internal/app/             → wires backend.Watcher + ui.Model, runs tea.Program
internal/backend/         → Watcher polls tmux every 1.5s (sessions/windows/panes) via goroutines
internal/data/dispatcher/ → converts backend.Events into state store updates
internal/state/           → SessionStore, WindowStore, PaneStore (in-memory, thread-safe)
internal/tmux/            → gotmuxcc control-mode client (primary) + exec fallback
internal/menu/            → menu tree definitions, loaders, action handlers, refs.go (display id → tmux id)
internal/ui/              → Bubble Tea Model; split across focused files
internal/ui/state/        → Level type: items, cursor, filter, selection, viewport for one menu depth
internal/ui/command/      → command bus for menu action dispatch with trace logging
internal/format/table/    → columnar table formatting with alignment
internal/plugin/          → tmux plugin management (discover, install, update, uninstall, source)
internal/resurrect/       → session save/restore/autosave: snapshot format, storage, layout replay
internal/extract/         → extrakto-style token extraction from captured pane text (consumer-agnostic)
internal/clipboard/       → native system clipboard copy, dispatching on OS (consumer-agnostic)
internal/tmuxopts/        → embedded tmux option/command catalog (consumer-agnostic)
internal/cmdparse/        → tmux command synopsis → schema; input analysis + value resolution for completion
internal/cmdhelp/         → per-command summaries and flag descriptions derived from tmuxopts
internal/shquote/         → sh-safe quoting / field splitting for run-shell style command strings
internal/theme/           → lipgloss styles (theme.Default())
internal/logging/         → structured JSON log file; events/ sub-package for trace points
internal/testutil/        → integration test helpers (StartTmuxServer, IsolateFromUserServer, CapturePane, SendKeys, …)
```

Packages marked consumer-agnostic must not import bubbletea, lipgloss, tmux or menu — they are pure logic with their own unit tests.

### Menu system

Menu items are identified by colon-separated IDs (e.g. `session:switch`, `pane:kill`). `internal/menu/registry.go` builds a `Registry` tree from three maps defined in `menu.go`:

- `CategoryLoaders()` — top-level submenu loaders (extract, process, clipboard, keybinding, command, pane, window, session, plugins, resurrect; `customize-mode` is a leaf `ActionHandler` that targets the host pane)
- `ActionLoaders()` — loaders for nested items within actions
- `ActionHandlers()` — leaf actions that execute tmux operations

Root menu (`RootItems()`, in display order): extract, customize-mode, keybinding, command, pane, window, plugins, resurrect, session. `process` and `clipboard` loaders stay wired but are hidden from the root until their submenus are implemented.

The UI maintains a `stack []*level` where each level holds the current items, cursor, filter state, and viewport offset. Navigation pushes/pops levels. Multi-select (tab to mark) is enabled per node via `markMultiSelect` in `registry.go` (`window:kill`, `pane:join`, `pane:kill`, `plugins:update`, `plugins:uninstall`, `extract`).

**Display ids vs tmux targets:** menu items carry human-readable ids (`session:index`, `session:index.pane`) because that is what users filter on, but since tmux allows `:` and `.` in names those can't be parsed back into targets. Every tmux command must target the tmux id (`$N`, `@N`, `%N`) resolved through the helpers in `internal/menu/refs.go` (`SessionEntryFor` etc.), never a name.

**Untargeted commands resolve to the popup.** Because the popup is a modal floating pane, a tmux command without `-t` acts on the popup itself, not the pane the user launched from — even when `$TMUX_PANE` names another pane. Always target the host pane explicitly (see how `customize-mode` does it). Never silently inject `-t` into a command the user typed in the command prompt; ask first.

**Menu item styling:** all menu levels must use `buildItemLine` for consistent rendering. The selected item's highlight (background colour) must fill the full container width. Multi-select lines are rendered as `raw: true` via `buildMultiSelectLine`, which renders each segment (indicator, checkbox, body) independently with composite styles to avoid ANSI nesting issues. Checkbox characters (■/□) use `styles.CheckboxChecked`/`styles.Checkbox` foreground composited with the line's background via `Inherit`. Every y/n confirmation goes through `renderYNPrompt` in `yn_prompt.go`.

**Live data:** any level that shows tmux state must rebuild from backend updates rather than freezing a snapshot when opened; re-anchor the cursor by tmux id (see `window_move.go`, `restore_refresh.go`). Forms and submenus launched directly via `--root-menu` must quit on `Esc`, not pop back to the root.

### UI decomposition (`internal/ui/`)

| File | Responsibility |
|---|---|
| `model.go` | `Model` struct, `Init`/`Update`, handler dispatch via `reflect.Type` map, `Mode` enum |
| `navigation.go` | cursor movement, enter/escape handling, level stack management |
| `input.go` | text filter input, backspace, filter cursor |
| `view.go` | `View()` rendering, side-by-side preview layout, viewport logic, mouse handling |
| `commands.go` | backend event handling, menu loading commands |
| `backend.go` | `waitForBackendEvent`, backend update dispatch; simple live levels rebuild via the `menu.LiveLevels` table (`internal/menu/live.go`) |
| `prompt.go` | filter / command prompt rendering |
| `completion.go`, `completion_datasource.go`, `completion_options.go` | command-prompt tab completion: dropdown widget, live tmux data source, option/hook/value completion with colour swatches |
| `forms.go` | rename/new-session/save-as/pane-capture forms |
| `preview.go` | async preview system (pane capture, tree, layout, plugin overview) |
| `tree.go`, `treestyle.go` | hierarchical session/window/pane tree view; `TMUX_POPUP_CONTROL_TREE_STYLE` variants |
| `window_move.go` | window move tree, kept live with backend updates |
| `plugin_install.go` | per-plugin progress display for install/update operations |
| `resurrect.go`, `resurrect_view.go`, `restore_refresh.go` | `ModeResurrect` save/restore progress UI and snapshot picker refresh |
| `extract.go`, `extract_mode.go`, `extract_selector.go` | extract level: token categories, grab areas, Ctrl-F/Ctrl-G selector popups with auto-dismiss |
| `delete_confirm.go`, `yn_prompt.go` | multi-select delete confirmation; the single shared y/n prompt renderer |
| `pane_title.go`, `scrollbar.go`, `gradient.go` | small rendering helpers |
| `harness.go` | `Harness` for driving the model in tests without a program loop |

UI modes: `ModeMenu`, `ModePaneForm`, `ModeWindowForm`, `ModeSessionForm`, `ModePluginInstall`, `ModeResurrect`, `ModeSessionSaveForm`, `ModePaneCaptureForm`, `ModeCommandOutput`.

### State sub-package (`internal/ui/state/`)

`level.go` / `cursor.go` / `filter.go` / `selection.go` / `items.go` — the `Level` type owns items, cursor, filter, selection, and viewport for one menu depth.

### Session tree (`internal/menu/tree_state.go`)

`TreeState` tracks expand/collapse state for hierarchical session/window/pane browsing. `BuildTreeItems` produces a flat item list respecting expand state; `FilterTreeItems` supports fuzzy filtering with ancestor visibility. Tree item IDs use prefixes: `"tree:s:"`, `"tree:w:"`, `"tree:p:"`.

### tmux layer (`internal/tmux/`)

**Shared control-mode connection:** `newTmux` caches a single long-lived gotmuxcc client behind a `sync.Mutex`, returning the cached instance when the socket path matches and reconnecting when it changes. `tmux.Shutdown()` closes the cached connection at app exit (called via `defer` in `app.Run` and in every subcommand). `ForbidDialForTests` (`client_cache.go`) makes every dial fail with `ErrDialForbidden`; `internal/ui` unit tests enable it from `TestMain` so no UI unit test can reach a real server.

Two communication paths coexist:
1. **gotmuxcc control-mode** (`client.go`, `sessions.go`, `windows.go`, `panes.go`, `preview.go`) — the primary path for nearly all tmux operations (list/create/rename/kill sessions and windows, switch client, display-message, capture-pane, rename/resize pane, etc.). The control client sets `new-layouts` so json v2 layouts (floating panes) round-trip.
2. **exec `tmux` subprocess** (`command.go`, `snapshots.go`, `sessions.go`) — resilience fallbacks (`killSessionCLI`, `detachSessionCLI`, `hasSessionCLI`, `fetchSessionsFallback`) used when control-mode races or is unavailable. Options are control-mode too: `PluginOptions` reads every `@tmux-popup-control-*` option with one `show-options -g -F` (cached until a user command invalidates it), and `ShowOption`/`Option` (env var first) serve from it — use those, never a per-option `show-options`.

`runExecCommand` is a package-level var (`func(name string, args ...string) commander`) swapped in tests with `withStubCommander`. `newTmux` is similarly swappable for `fakeClient` in tests.

**Handle interfaces:** `windowHandle` and `sessionHandle` interfaces abstract gotmuxcc types so lifecycle operations (Select, Kill, Rename, Detach) can be tested with stub handles without a live server. `newWindowHandle` and `newSessionHandle` are injectable package-level vars.

**Client identification:** `CurrentClientID` resolves the user's TTY client (not the control-mode client) (normally passed by the launcher in `TMUX_POPUP_CONTROL_CLIENT`; the fallback lists clients and picks a non-`ControlMode` one attached to the host session from `TMUX_POPUP_CONTROL_SESSION_ID`) — `display-message #{client_name}` over control mode returns the control client's own name. `FindTerminalClient` provides a similar lookup for subcommands. `display-popup` is silently refused for control clients, so `showPopup` always targets the terminal client.

**Resilience:** fetchers fall back to direct `tmux` CLI calls when control-mode races or returns stale data. Vanished resources are silently ignored rather than failing whole queries. `KillWindows`/`KillWindow` issue `kill-window -t <@id>` directly rather than going through a looked-up handle, because newly created windows can be missing from the cached session list.

### Backend polling (`internal/backend/`)

The `Watcher` runs three goroutines (sessions, windows, panes), each polling on a 1.5s ticker interval. Each poller additionally has a 250ms throttle to prevent bursts if fetches complete faster than expected. `watcher.Stop()` is non-blocking; `watcher.Wait()` drains for tests.

### Plugin system (`internal/plugin/`)

Discovers plugins from `@plugin` declarations in tmux config files (matching tpm conventions). Supports install (git clone with GitHub URL fallback), update (git pull + submodule update), uninstall, tidy (find undeclared plugins), and source (execute `*.tmux` files). `runGitCommand` is a package-level var injectable for tests. `main.tmux` publishes the resolved binary path as `@tmux-popup-control-binary-path` so status-line snippets can find it.

### Resurrect (`internal/resurrect/`)

`save.go` / `restore.go` orchestrate full-server snapshots (sessions, windows, panes, json layouts including floating panes, optionally pane contents) and emit `ProgressEvent`s consumed by `ModeResurrect`; callers go through `resurrect.BackendFor(socket)` (`backend.go`), the seam for a future native tmux save-state backend. `storage.go` writes `<name>_<uuid>_<timestamp>.json` (manual) or `<uuid>_<timestamp>.json` snapshots under the storage dir and maintains a "last" pointer; autosaves are named `auto-YYYY-MM-DDTHH-MM-SS` and pruned independently (`autosave.go`, bounded by `@tmux-popup-control-autosave-max`). Restore merges windows into existing sessions idempotently. Pane-content replay waits are bounded (`replayWaitTimeout`); a pane whose replay outlives the bound is reported as an error-kind progress line and skipped, never fatal, and still-pending replays get one more bounded wait before the client switch and content removal. The progress UI renders the final failure (`resurrectFailureLines`) — a failed save/restore that showed nothing used to look exactly like a hang — so if a restore seems stuck, read the error on screen first, then the livelock postmortem referenced in memory before touching that code. Pre-json (v1) saves of windows with floating panes restore with a warning, not an abort.

### Extract (`internal/extract/`, `internal/ui/extract*.go`)

Captures the originating pane (viewport / pane-history / window / window-history grab areas) and extracts tokens by category (word, path, line, quote, s-quote, quoted, url, host, all) using patterns ported from extrakto. `Enter` sends the selection to the originating pane; `Tab`/`Ctrl-Y` copies to a tmux buffer and, best-effort, the system clipboard via `internal/clipboard`. The tmux buffer is the source of truth; clipboard failure never blocks the copy.

### Command prompt and completion (`internal/cmdparse/`, `internal/cmdhelp/`, `internal/ui/completion*.go`)

The command menu lets users type raw tmux commands. `cmdparse` turns catalog synopses into `CommandSchema` (`schema.go`, `catalog.go`), analyses the partial input under the cursor (`analyse.go`) and resolves candidate values from live tmux data (`resolve.go`). `cmdhelp` supplies the one-line summary under the prompt and the per-flag descriptions in the dropdown. Flags, option names/values, hooks and targets all complete from the embedded catalog plus backend state; the dropdown renders above the prompt and falls back to below when there is no room. Pasted text (bracketed paste) is accepted into both the filter and the prompt.

### Configuration

CLI flags and env vars are both supported (flag wins). Key env vars:

| Env var | Purpose |
|---|---|
| `TMUX_POPUP_CONTROL_SOCKET` / `TMUX_POPUP_SOCKET` / `$TMUX` | Socket path resolution |
| `TMUX_POPUP_CONTROL_ROOT_MENU` | Launch directly into a submenu (e.g. `window`, `pane:swap`, `extract`) |
| `TMUX_POPUP_CONTROL_MENU_ARGS` | Arguments for target menu (e.g. `"expanded"` for `session:tree`) |
| `TMUX_POPUP_CONTROL_CLIENT` / `TMUX_POPUP_CONTROL_SESSION` | Explicit client ID / session name override |
| `TMUX_POPUP_CONTROL_WIDTH` / `HEIGHT` | Override terminal dimensions |
| `TMUX_POPUP_CONTROL_FOOTER` | Show keybinding hint row |
| `TMUX_POPUP_CONTROL_NO_PREVIEW` | Disable the preview panel |
| `TMUX_POPUP_CONTROL_TREE_STYLE` | Session tree connector style |
| `TMUX_POPUP_CONTROL_LOG_FILE` / `TRACE` / `VERBOSE` | Log file path; verbose JSON trace logging |
| `TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR` | Resurrect snapshot directory |
| `TMUX_POPUP_CONTROL_RESURRECT_NAME` / `RESURRECT_FROM` | Snapshot name for save / restore subcommands |
| `TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS` | Save and replay pane contents |
| `TMUX_POPUP_CONTROL_AUTOSAVE_MAX` / `AUTOSAVE_ICON` | Autosave retention and indicator icon (also `@tmux-popup-control-autosave-*`); the interval is the `@tmux-popup-control-autosave-interval-minutes` option only, since tmux's monitor reads it |
| `TMUX_POPUP_CONTROL_SESSION_FORMAT` / `WINDOW_FORMAT` / `WINDOW_FILTER` | Custom tmux formats for session/window labels |
| `TMUX_POPUP_CONTROL_SWITCH_CURRENT` | Include current session/window/pane in switch menus |

### tmux option catalog (`internal/tmuxopts/`)

`internal/tmuxopts/` embeds a JSON catalog of every tmux option/hook (names, scopes, value types, choices, shared value domains) so the command menu can offer name/value completion, validation, and inline help **without a tmux runtime dependency**. The package is consumer-agnostic (no bubbletea/lipgloss).

- **Source of truth is upstream**, not this repo: the catalog is generated at `~/git_tree/tmux/option-catalog/` (note the `option-catalog/` subdir; an older path was `~/git_tree/tmux/`). To refresh, `cp ~/git_tree/tmux/option-catalog/tmux-option-catalog.json internal/tmuxopts/tmux-option-catalog.json` and **include that repo's HEAD commit hash in the commit message** so the embedded snapshot is traceable. The `.md`/`.py` files there are the schema doc and generator — don't hand-edit the JSON. Diff the command help output after every refresh; descriptions are hand-maintained upstream and silently regress.
- **JSON shape**: top-level `schema_version` (currently 2), `options`, `pseudo_options` (just `@<name>` user options), `name_aliases` (alias↔canonical, e.g. `display-panes-color`→`display-panes-colour`), `shared_domains` (`colour`, `flag`, `key`, `style`, `tmux_command`, `format_string`, `terminal_features`, `terminal_override_capabilities`), and `events`. The Go structs in `catalog.go` only model the fields the UI consumes; extra JSON fields (`documented_scope`, `default_source`, `scope_mismatch_with_manpage`, the whole `events` block, etc.) are ignored by `json.Unmarshal` — add a struct field only when a consumer needs it.
- **Schema v2 (2026-08-16 refresh)** added structured command data and events. `shared_domains.tmux_command.commands[]` now carries `usage`, `argument_template`, `description`, `after_hook_name`, `positional_arguments`, and `flags[]` (each with `value_mode`/`value_name`/`description`) — modelled by `TmuxCommandEntry`/`TmuxCommandFlag`. The top-level `events` block (91 events with payload fields and their `hook_*` format names) is **not** modelled yet; it would be the source for completing hook payload formats inside `set-hook` commands.
- **Command help comes from the catalog**, not a generated file: `internal/cmdhelp` builds `Commands()` from `tmuxopts.Default()` on first use. There is no generator to re-run — refreshing the JSON is the whole update. (`cmd/gen_command_help`, which parsed the upstream `command-summary.md`, was removed on 2026-08-16.) Upstream's `command-summary.json` is a standalone duplicate of the same `commands[]` array; the repo deliberately does not embed it twice.
- **Colour options are NOT identified by `type=="colour"` alone.** As of the 2026-06-27 refresh the catalog decoupled colour-ness from `type`: most colour options (cursor-colour, clock-mode-colour, display-panes[-active]-colour, the dark-/light-theme-* palette, prompt[-command]-cursor-colour — 26 in total) are now `type=="string"` with a new `colour_option: true` boolean, while only status-bg, status-fg, and pane-colours retain `type=="colour"`. Always use `Option.IsColour()` (`type==colour || colour_option`; equivalent to `value.domain=="colour"`) for colour completion/decoration, never a bare `Type==TypeColour` check.
- **Consumers**: `values.go` (`ValueCandidates`/`ValueHint`/`OptionSummary`), `internal/ui/completion_options.go` (name/hook/value completion + `decorateShowOptionsLine` colour swatches), `internal/cmdhelp` (command summaries + flag descriptions for the command prompt and flag dropdown), `internal/cmdparse` (synopsis schemas), and `internal/tmux/client.go`. Load via `tmuxopts.Default()` (parses the embedded JSON once via `sync.OnceValues`).
- **Descriptions are hand-maintained upstream** in `tmux-command-metadata.json` (`.commands[].description` / `.flags`), not derived from the manpage. If a command summary or flag description reads wrong in the UI, fix it there and refresh — don't patch it locally. The UI colours scope words (server/session/window/pane/user/hook) in command summaries via `decorateScopeSummary`, so those summaries need to name the scopes they apply to.

### Filter / fuzzy search

`github.com/lithammer/fuzzysearch` is used for live filter input on menu levels. Filter state (text, cursor position) lives in `internal/ui/state/filter.go` on the `Level` type. Switch menus skip `UpdateItems` while they are the active level, to keep window formatting stable under the cursor.

### Preview system

Previews render for switch menus (`session:switch`, `window:switch`, `pane:switch`, `pane:join`), the session tree (`session:tree`), layout selection (`window:layout`), and the plugins menu. The default layout is **side-by-side**: a rounded-border preview panel on the right (60% width) when the terminal is wide enough, falling back to vertical (inline below the list) when too narrow. Mouse wheel scrolling is supported in the preview panel.

Preview content varies by kind: pane/session/window previews use async `client.CapturePane` via control-mode (`panePreviewFn`, injectable for tests), with a fallback to static window/pane lists when no active pane is found. Layout previews apply the selected layout live. Plugin previews show a static status table. Per-level sequence numbers prevent stale responses from overwriting newer data. Pane previews default-scroll to the bottom so recent output is visible.

### gotmuxcc dependency

The library is vendored under `vendor/` (git-ignored — never stage it). Changes to gotmuxcc require updating the sibling repo in `~/git_tree/gotmuxcc`, not the vendor copy. Use `make update-gotmuxcc` to pull the latest version. `go mod vendor` only vendors packages that are actually imported.

## Pending work

`context/todo.md` is the live list; don't duplicate it here. Current headline themes: tmux popups-as-floating-panes follow-ups (untargeted command audit, popup pane appearing in pane lists, `switch-client` from inside the popup), one open gotmuxcc attach-by-name bug, and missing `ListClientsContext`/`DisplayMessageContext` in gotmuxcc.

## Testing patterns

- Unit tests use `withStubCommander` / `withStubTmux` to inject fakes at the package-var level; resurrect uses the same `with*` pattern for its package vars.
- `internal/ui/harness.go` provides `Harness` for driving the Bubble Tea model in tests without running a program loop. `Harness.Update` is non-draining on purpose (blocking blink commands would deadlock otherwise).
- Golden files live in `testdata/capture/`; regenerate with `UPDATE_GOLDEN=1`.
- Integration tests build the binary (`buildBinary`) and spin up a temporary tmux server via `StartTmuxServer` / `StartIsolatedTmuxServer` with a fresh socket per test; `launchBinary(t, bin, socket, session, rootMenu)` starts it in a pane; assert with `WaitForContent` / `WaitForAbsent`, drive with `SendKeys` / `SendText`; tear down via `KillServer`. Every tmux invocation in tests and hook scripts must pass `-S <socket>`.
- User-reported bugs get an integration test that reproduces them against real tmux before any fix.
- Plugin tests use injectable `runGitCommand` for stubbing git operations.
- Watch test failure messages for leaked environment (never dump `os.Environ()`), and when a tmux upgrade makes everything slow, check `otool -L $(which tmux) | grep asan` before debugging the suite.
