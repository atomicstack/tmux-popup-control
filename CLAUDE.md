# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project context

The `context/` directory tracks ongoing work:
- `context/context.md` — project overview and design goals
- `context/todo.md` — pending work items
- `context/done.md` — completed work log
- `context/scratchpad.md` — cross-session environment notes

Check `todo.md` and `done.md` at the start of sessions to understand current state. Update them after completing work.

**Design goals:** colourful, dynamic, modern feel; fast and async (goroutines for background work); code that is clear, concise, and modular (logic in sub-packages, not dumped into the model); comprehensive tests kept in sync with the code.

**Reference implementation:** a local checkout of tmux-fzf lives at `/tmp/tmux-fzf`.

## Commands

The repository uses a Makefile that keeps Go build artifacts inside the workspace (`.gocache/`, `.gomodcache/`) and sets `GOPROXY=off` for offline builds. Always use `make` targets rather than raw `go` commands:

```sh
make build      # builds ./tmux-popup-control
make run        # runs the application
make test       # runs all tests
make cover      # runs tests with coverage report
make fmt        # gofmt -w .
make tidy       # go mod tidy
make clean-cache
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

## Architecture

This is a [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI that wraps tmux as a popup control menu. The app runs inside a `tmux display-popup` and communicates with tmux via the `gotmuxcc` library (vendored, replaced with `../gotmuxcc`).

### Layer overview

```
main.go                → config loading, signal handling, app.Run()
internal/config/       → CLI flags + env vars → Config struct
internal/app/          → wires backend.Watcher + ui.Model, runs tea.Program
internal/backend/      → Watcher polls tmux every 1.5s (sessions/windows/panes) via goroutines
internal/data/dispatcher/ → converts backend.Events into state store updates
internal/state/        → SessionStore, WindowStore, PaneStore (in-memory, thread-safe)
internal/tmux/         → wraps tmux CLI + gotmuxcc for all tmux operations
internal/menu/         → menu tree definitions, loaders, action handlers
internal/ui/           → Bubble Tea Model; split across focused files
internal/theme/        → lipgloss styles (theme.Default())
internal/logging/      → structured JSON log file; events/ sub-package for trace points
```

### Menu system

Menu items are identified by colon-separated IDs (e.g. `session:switch`, `pane:kill`). `internal/menu/registry.go` builds a `Registry` tree from three maps defined in `menu.go`:

- `CategoryLoaders()` — top-level submenu loaders (session, window, pane, etc.)
- `ActionLoaders()` — loaders for nested items within actions
- `ActionHandlers()` — leaf actions that execute tmux operations

The UI maintains a `stack []*level` where each level holds the current items, cursor, filter state, and viewport offset. Navigation pushes/pops levels. Multi-select (tab to mark) is enabled per node in the registry (`window:kill`, `pane:join`, `pane:kill`).

### UI decomposition (`internal/ui/`)

| File | Responsibility |
|---|---|
| `model.go` | `Model` struct, `Init`/`Update`, handler dispatch via `reflect.Type` map |
| `navigation.go` | cursor movement, enter/escape handling, level stack management |
| `input.go` | text filter input, backspace, filter cursor |
| `view.go` | `View()` rendering, `limitHeight`, `applyWidth`, viewport logic |
| `commands.go` | backend event handling, menu loading commands |
| `prompt.go` | filter prompt rendering |
| `forms.go` | rename/new-session forms (ModePaneForm, ModeWindowForm, ModeSessionForm) |
| `backend.go` | `waitForBackendEvent`, backend update dispatch |
| `preview.go` | inline preview for session/window/pane switch menus |

### State sub-package (`internal/ui/state/`)

`level.go` / `cursor.go` / `filter.go` / `selection.go` / `items.go` — the `Level` type owns items, cursor, filter, selection, and viewport for one menu depth.

### tmux layer (`internal/tmux/`)

Two communication paths coexist:
1. **gotmuxcc control-mode** (`client.go`, `sessions.go`, `windows.go`) — used for structured queries (list sessions, create/rename/kill, switch client).
2. **exec `tmux` subprocess** (`command.go`, `panes.go`) — used for operations not exposed cleanly via control mode (rename-pane, resize-pane, capture-pane, etc.).

`runExecCommand` is a package-level var (`func(name string, args ...string) commander`) swapped in tests with `withStubCommander`. `newTmux` is similarly swappable for `fakeClient` in tests.

**Important:** every gotmuxcc client must be `Close()`d after use. Failing to do so leaks background `tmux -C` processes.

**Handle interfaces:** `windowHandle` and `sessionHandle` interfaces in `internal/tmux` abstract gotmuxcc types so lifecycle operations (Select, Kill, Rename, Detach) can be tested with stub handles without a live server. `newWindowHandle` and `newSessionHandle` are injectable package-level vars.

**Resilience:** fetchers fall back to direct `tmux` CLI calls when control-mode races or returns stale data. Vanished resources are silently ignored rather than failing whole queries.

**Backend polling** (`internal/backend/`) is rate-limited to ~4Hz via a 250ms throttle per poller goroutine.

### Configuration

CLI flags and env vars are both supported. Key env vars:

| Env var | Purpose |
|---|---|
| `TMUX_POPUP_CONTROL_SOCKET` / `TMUX_POPUP_SOCKET` / `$TMUX` | Socket path resolution (flag wins) |
| `TMUX_POPUP_CONTROL_ROOT_MENU` | Launch directly into a submenu (e.g. `window`, `pane:swap`) |
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

Previews render below the menu list for `session:switch`, `window:switch`, and `pane:switch` levels. Session and window previews are built synchronously from cached snapshot data in the `Model`. Pane previews fire an async `tmux capture-pane` command (`panePreviewFn`, injectable for tests). Per-level sequence numbers prevent stale responses from overwriting newer data.

### gotmuxcc dependency

`go.mod` uses `replace github.com/atomicstack/gotmuxcc => ../gotmuxcc` pointing at a sibling checkout. The library is also vendored under `vendor/`. Changes to gotmuxcc require updating the sibling repo, not the vendor copy.

## Pending work

From `context/todo.md`:
- Review whether `LinkWindow`/`MoveWindow`/`SwapWindows` and pane move/break flows need `windowHandle`-style abstractions or expanded fake scenarios.
- Consider extending integration coverage to pane moves/swaps and multi-session client interactions.
- Evaluate whether additional message handlers in `model.go` (handler registry management) should be decomposed further or covered with focused tests.
- Identify further UI cleanups or feature work once the refactor settles.

## Testing patterns

- Unit tests use `withStubCommander` / `withStubTmux` to inject fakes at the package-var level.
- `internal/ui/harness.go` provides `Harness` for driving the Bubble Tea model in tests without running a program loop.
- Golden files live in `testdata/capture/`; regenerate with `UPDATE_GOLDEN=1`.
- Integration tests (`internal/testutil/`, `internal/tmux/integration_test.go`, `internal/ui/integration_test.go`) build the binary and spin up a temporary tmux server via `StartTmuxServer`. They clear `$TMUX` so the user's own session is never touched, and tear down via `KillServer` to avoid orphaned processes.
- Each integration test uses a fresh socket path to avoid cross-test contamination.
