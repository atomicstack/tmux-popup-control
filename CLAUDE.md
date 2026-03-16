# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview:
This tool is called tmux-popup-control. It is a Golang port and improvement of
the tmux-fzf (https://github.com/sainnhe/tmux-fzf) plugin for tmux. This tool
uses Bubble Tea and Lip Gloss from the Charm project for its TUI and styling,
gotmuxcc by @atomicstack as a tmux control-mode client library, and Fuzzy
Search by @lithammer for dealing with user input.

## Project context

The `context/` directory tracks ongoing work:
- `context/context.md` — project overview and design goals
- `context/todo.md` — pending work items
- `context/done.md` — completed work log

Check `todo.md` and `done.md` at the start of sessions to understand current state. Update them after completing work.

**Design goals:** colourful, dynamic, modern feel; fast and async (goroutines for background work); code that is clear, concise, and modular (logic in sub-packages, not dumped into the model); comprehensive tests kept in sync with the code.

**Key principle:** use the gotmuxcc persistent control-mode connection for all tmux operations. Direct `tmux` exec (`runExecCommand`) is a last resort for operations not yet exposed by gotmuxcc. Never work around gotmuxcc bugs — report them so they can be fixed upstream.

**Do not hand-roll code when a vendored dependency already provides the functionality.** Before writing low-level helpers (ANSI escape codes, string manipulation, formatting), check what `lipgloss`, `charmbracelet/x/ansi`, `bubbletea`, and other vendored libraries expose. For example, use `ansi.Style.ForegroundColor()` instead of emitting raw `\x1b[38;5;…m` sequences.

## Commands

The repository uses a Makefile that keeps Go build artifacts inside the workspace (`.gocache/`, `.gomodcache/`) and sets `GOPROXY=off` for offline builds. Always use `make` targets rather than raw `go` commands:

```sh
make build           # builds ./tmux-popup-control
make run             # runs the application
make test            # runs all tests
make cover           # runs tests with coverage report
make fmt             # gofmt -w .
make tidy            # go mod tidy
make clean-cache     # removes .gocache/ and .gomodcache/
make update-gotmuxcc # fetches latest gotmuxcc + re-vendors (online)
make release         # cross-compiles + creates GitHub release via gh
```

To run a single test or package:
```sh
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/tmux/... -run TestFetchSessions
```

To update golden files in tests:
```sh
UPDATE_GOLDEN=1 make test
```

Integration tests (in `internal/testutil/`, `internal/tmux/integration_test.go`, `internal/ui/integration_test.go`) require a live tmux socket and are skipped automatically when tmux is unavailable.

### CLI subcommands

Besides the default TUI mode, `main.go` supports:
- `install-and-init-plugins` — called during tmux startup; sources installed plugins, schedules a deferred popup for any uninstalled ones.
- `deferred-install` — background helper that waits for tmux startup, then opens the plugin install UI in a `display-popup`.

## Architecture

This is a [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI that wraps tmux as a popup control menu. The app runs inside a `tmux display-popup` and communicates with tmux via the `gotmuxcc` library (vendored).

### Layer overview

```
main.go                   → config loading, signal handling, app.Run(), plugin subcommands
internal/config/          → CLI flags + env vars → Config struct
internal/app/             → wires backend.Watcher + ui.Model, runs tea.Program
internal/backend/         → Watcher polls tmux every 1.5s (sessions/windows/panes) via goroutines
internal/data/dispatcher/ → converts backend.Events into state store updates
internal/state/           → SessionStore, WindowStore, PaneStore (in-memory, thread-safe)
internal/tmux/            → gotmuxcc control-mode client (primary) + exec fallback for tmux operations
internal/menu/            → menu tree definitions, loaders, action handlers
internal/ui/              → Bubble Tea Model; split across focused files
internal/ui/state/        → Level type: items, cursor, filter, selection, viewport for one menu depth
internal/ui/command/      → command bus for menu action dispatch with trace logging
internal/format/table/    → columnar table formatting with alignment
internal/plugin/          → tmux plugin management (discover, install, update, uninstall, source)
internal/theme/           → lipgloss styles (theme.Default())
internal/logging/         → structured JSON log file; events/ sub-package for trace points
internal/testutil/        → integration test helpers (StartTmuxServer, CapturePane, SendKeys, etc.)
```

### Menu system

Menu items are identified by colon-separated IDs (e.g. `session:switch`, `pane:kill`). `internal/menu/registry.go` builds a `Registry` tree from three maps defined in `menu.go`:

- `CategoryLoaders()` — top-level submenu loaders (session, window, pane, process, clipboard, keybinding, command, plugins)
- `ActionLoaders()` — loaders for nested items within actions
- `ActionHandlers()` — leaf actions that execute tmux operations

Root menu categories (in display order): process, clipboard, keybinding, command, pane, window, plugins, session.

The UI maintains a `stack []*level` where each level holds the current items, cursor, filter state, and viewport offset. Navigation pushes/pops levels. Multi-select (tab to mark) is enabled per node in the registry (`window:kill`, `pane:join`, `pane:kill`, `plugins:update`, `plugins:uninstall`).

**Menu item styling:** all menu levels must use `buildItemLine` for consistent rendering. The selected item's highlight (background colour) must fill the full container width. Multi-select lines are rendered as `raw: true` via `buildMultiSelectLine`, which renders each segment (indicator, checkbox, body) independently with composite styles to avoid ANSI nesting issues. Checkbox characters (■/□) use `styles.CheckboxChecked`/`styles.Checkbox` foreground composited with the line's background via `Inherit`.

### UI decomposition (`internal/ui/`)

| File | Responsibility |
|---|---|
| `model.go` | `Model` struct, `Init`/`Update`, handler dispatch via `reflect.Type` map |
| `navigation.go` | cursor movement, enter/escape handling, level stack management |
| `input.go` | text filter input, backspace, filter cursor |
| `view.go` | `View()` rendering, side-by-side preview layout, viewport logic, mouse handling |
| `commands.go` | backend event handling, menu loading commands |
| `prompt.go` | filter prompt rendering |
| `forms.go` | rename/new-session forms (ModePaneForm, ModeWindowForm, ModeSessionForm) |
| `backend.go` | `waitForBackendEvent`, backend update dispatch |
| `preview.go` | async preview system (pane capture, tree, layout, plugin overview) |
| `tree.go` | hierarchical session/window/pane tree view with expand/collapse |
| `plugin_confirm.go` | plugin uninstall/tidy confirmation UI (cycles through plugins with y/n) |
| `plugin_install.go` | per-plugin progress display for install/update operations |

UI modes: `ModeMenu`, `ModePaneForm`, `ModeWindowForm`, `ModeSessionForm`, `ModePluginConfirm`, `ModePluginInstall`.

### State sub-package (`internal/ui/state/`)

`level.go` / `cursor.go` / `filter.go` / `selection.go` / `items.go` — the `Level` type owns items, cursor, filter, selection, and viewport for one menu depth.

### Session tree (`internal/menu/tree_state.go`)

`TreeState` tracks expand/collapse state for hierarchical session/window/pane browsing. `BuildTreeItems` produces a flat item list respecting expand state; `FilterTreeItems` supports fuzzy filtering with ancestor visibility. Tree item IDs use prefixes: `"tree:s:"`, `"tree:w:"`, `"tree:p:"`.

### tmux layer (`internal/tmux/`)

**Shared control-mode connection:** `newTmux` caches a single long-lived gotmuxcc client behind a `sync.Mutex`, returning the cached instance when the socket path matches and reconnecting when it changes. `tmux.Shutdown()` closes the cached connection at app exit (called via `defer` in `app.Run`).

Two communication paths coexist:
1. **gotmuxcc control-mode** (`client.go`, `sessions.go`, `windows.go`, `panes.go`, `preview.go`) — the primary path for nearly all tmux operations (list/create/rename/kill sessions and windows, switch client, display-message, capture-pane, rename/resize pane, etc.).
2. **exec `tmux` subprocess** (`command.go`, `snapshots.go`, `sessions.go`) — resilience fallbacks only (`killSessionCLI`, `detachSessionCLI`, `hasSessionCLI`, `fetchSessionsFallback`), used when control-mode races or is unavailable.

`runExecCommand` is a package-level var (`func(name string, args ...string) commander`) swapped in tests with `withStubCommander`. `newTmux` is similarly swappable for `fakeClient` in tests.

**Handle interfaces:** `windowHandle` and `sessionHandle` interfaces abstract gotmuxcc types so lifecycle operations (Select, Kill, Rename, Detach) can be tested with stub handles without a live server. `newWindowHandle` and `newSessionHandle` are injectable package-level vars.

**Client identification:** `CurrentClientID` resolves the user's TTY client (not the control-mode client) by finding non-control-mode clients attached to the popup's session. `FindTerminalClient` provides a similar lookup for plugin subcommands.

**Resilience:** fetchers fall back to direct `tmux` CLI calls when control-mode races or returns stale data. Vanished resources are silently ignored rather than failing whole queries.

### Backend polling (`internal/backend/`)

The `Watcher` runs three goroutines (sessions, windows, panes), each polling on a 1.5s ticker interval. Each poller additionally has a 250ms throttle to prevent bursts if fetches complete faster than expected.

### Plugin system (`internal/plugin/`)

Discovers plugins from `@plugin` declarations in tmux config files (matching tpm conventions). Supports install (git clone with GitHub URL fallback), update (git pull + submodule update), uninstall, tidy (find undeclared plugins), and source (execute `*.tmux` files). `runGitCommand` is a package-level var injectable for tests.

### Configuration

CLI flags and env vars are both supported. Key env vars:

| Env var | Purpose |
|---|---|
| `TMUX_POPUP_CONTROL_SOCKET` / `TMUX_POPUP_SOCKET` / `$TMUX` | Socket path resolution (flag wins) |
| `TMUX_POPUP_CONTROL_ROOT_MENU` | Launch directly into a submenu (e.g. `window`, `pane:swap`) |
| `TMUX_POPUP_CONTROL_MENU_ARGS` | Arguments for target menu (e.g. `"expanded"` for `session:tree`) |
| `TMUX_POPUP_CONTROL_CLIENT` | Explicit client ID override |
| `TMUX_POPUP_CONTROL_SESSION` | Explicit session name override |
| `TMUX_POPUP_CONTROL_WIDTH/HEIGHT` | Override terminal dimensions |
| `TMUX_POPUP_CONTROL_FOOTER` | Show keybinding hint row |
| `TMUX_POPUP_CONTROL_LOG_FILE` | Log file path |
| `TMUX_POPUP_CONTROL_TRACE` | Enable verbose JSON trace logging |
| `TMUX_POPUP_CONTROL_SESSION_FORMAT` | Custom tmux format for session labels |
| `TMUX_POPUP_CONTROL_WINDOW_FORMAT/FILTER` | Custom format/filter for window list |
| `TMUX_POPUP_CONTROL_SWITCH_CURRENT` | Include current session/window/pane in switch menus |

### Filter / fuzzy search

`github.com/lithammer/fuzzysearch` is used for live filter input on menu levels. Filter state (text, cursor position) lives in `internal/ui/state/filter.go` on the `Level` type.

### Preview system

Previews render for switch menus (`session:switch`, `window:switch`, `pane:switch`, `pane:join`), the session tree (`session:tree`), layout selection (`window:layout`), and the plugins menu. The default layout is **side-by-side**: a rounded-border preview panel on the right (60% width) when the terminal is wide enough, falling back to vertical (inline below the list) when too narrow. Mouse wheel scrolling is supported in the preview panel.

Preview content varies by kind: pane/session/window previews use async `client.CapturePane` via control-mode (`panePreviewFn`, injectable for tests), with a fallback to static window/pane lists when no active pane is found. Layout previews apply the selected layout live. Plugin previews show a static status table. Per-level sequence numbers prevent stale responses from overwriting newer data. Pane previews default-scroll to the bottom so recent output is visible.

### gotmuxcc dependency

The library is vendored under `vendor/`. Changes to gotmuxcc require updating the sibling repo in `~/git_tree/gotmuxcc`, not the vendor copy. Use `make update-gotmuxcc` to pull the latest version.

## Pending work

From `context/todo.md`:
- Review whether remaining tmux helpers (LinkWindow/MoveWindow/SwapWindows, pane move/break flows) need handle abstractions or expanded fake scenarios.
- Consider extending integration coverage to pane moves/swaps and multi-session client interactions.
- Evaluate whether additional message handlers in `model.go` should be decomposed further or covered with focused tests.
- Consider adding reconnection logic to the shared control-mode connection if the cached client's transport dies mid-session.
- Investigate pre-existing `TestRootMenuRendering` integration test failure.
- Review whether the vertical (inline) preview fallback path is still needed or can be removed.
- Verify popup width in `main.sh` is set wide enough for comfortable side-by-side preview.

## Testing patterns

- Unit tests use `withStubCommander` / `withStubTmux` to inject fakes at the package-var level.
- `internal/ui/harness.go` provides `Harness` for driving the Bubble Tea model in tests without running a program loop.
- Golden files live in `testdata/capture/`; regenerate with `UPDATE_GOLDEN=1`.
- Integration tests (`internal/testutil/`, `internal/tmux/integration_test.go`, `internal/ui/integration_test.go`) build the binary and spin up a temporary tmux server via `StartTmuxServer`. They clear `$TMUX` so the user's own session is never touched, and tear down via `KillServer` to avoid orphaned processes.
- Each integration test uses a fresh socket path to avoid cross-test contamination.
- Plugin tests use injectable `runGitCommand` for stubbing git operations.
