# tmux-popup-control

A terminal UI for managing tmux sessions, windows, panes, and plugins from
inside a `tmux display-popup`. Built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss) (v2), using a
persistent control-mode connection via
[gotmuxcc](https://github.com/atomicstack/gotmuxcc). Inspired by
[tmux-fzf](https://github.com/sainnhe/tmux-fzf).

## Features

### Session management
- **Save** sessions — auto-timestamped or named snapshots of all sessions,
  windows, panes, layouts, and optionally pane contents
- **Restore** sessions — from the most recent save or a picked snapshot,
  with progress UI and conflict handling (skips existing sessions)
- **Switch** between sessions with live pane-capture preview
- **New** session creation via inline form
- **Rename** sessions via inline form
- **Kill** sessions
- **Detach** clients from sessions
- **Tree view** — full session/window/pane hierarchy with expand/collapse,
  multi-word fuzzy filtering across the tree, and per-node live previews

### Window management
- **Switch** windows with live pane-capture preview
- **Rename** windows via inline form
- **Kill** windows (multi-select)
- **Swap** windows
- **Move** windows between sessions
- **Link** windows into other sessions
- **Layout** presets with live preview (even-horizontal, even-vertical,
  main-horizontal, main-vertical, tiled)

### Pane management
- **Switch** panes with live pane-capture preview
- **Rename** panes via inline form
- **Kill** panes (multi-select)
- **Swap** panes
- **Join** panes from other windows (multi-select)
- **Break** pane out to its own window
- **Resize** panes (left/right/up/down)

### Plugin management
- **Install** plugins declared via `@plugin` in tmux config (tpm-compatible)
- **Update** plugins via git pull (multi-select, with `[all]` option)
- **Uninstall** plugins with per-plugin confirmation (multi-select, with `[all]` option)
- **Status columns** in update/uninstall submenus and preview panel showing
  installed/undeclared state with color coding
- **`install-and-init-plugins`** CLI subcommand — drop-in replacement for tpm's
  `run '~/.tmux/plugins/tpm/tpm'` in tmux.conf; sources installed plugins at
  startup and opens a deferred popup for any that need installing

### Other menus
- **Keybinding** browser — lists all tmux key bindings, filterable
- **Command** browser — lists all tmux commands, filterable

### UI
- Fuzzy-search filtering on every menu level
- Breadcrumb navigation with push/pop menu stack
- Side-by-side preview panel with ANSI rendering and mouse-wheel scrolling
- Background polling keeps menu data in sync with tmux state
- Multi-select with Tab for bulk operations
- Alt-screen with mouse cell-motion support

## Not yet implemented

- **Process management** — menu entries exist but no action handlers are wired up
- **Clipboard** — menu entries exist but no action handlers are wired up

## Prerequisites

- Go 1.24+
- `tmux` available in `$PATH`

## Building and running

The repository keeps Go build artifacts inside the workspace (`.gocache/`,
`.gomodcache/`) so it works cleanly in sandboxed environments. Use the Makefile:

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

## Configuration

| Flag / env var | Purpose |
|---|---|
| `--socket` / `TMUX_POPUP_CONTROL_SOCKET` | tmux socket path (falls back to `TMUX_POPUP_SOCKET`, then `$TMUX`) |
| `--root-menu` / `TMUX_POPUP_CONTROL_ROOT_MENU` | open directly into a submenu (e.g. `window`, `pane:swap`, `session:tree`) |
| `--menu-args` / `TMUX_POPUP_CONTROL_MENU_ARGS` | arguments for the target menu (e.g. `expanded` for `session:tree`) |
| `--width` / `TMUX_POPUP_CONTROL_WIDTH` | viewport width in cells (0 = terminal width) |
| `--height` / `TMUX_POPUP_CONTROL_HEIGHT` | viewport height in rows (0 = terminal height) |
| `--footer` / `TMUX_POPUP_CONTROL_FOOTER` | show keybinding hint row |
| `--verbose` / `TMUX_POPUP_CONTROL_VERBOSE` | print success messages for actions |
| `--log-file` / `TMUX_POPUP_CONTROL_LOG_FILE` | log file path |
| `--trace` / `TMUX_POPUP_CONTROL_TRACE` | enable verbose JSON trace logging |
| `TMUX_POPUP_CONTROL_CLIENT` | explicit client ID override (env only) |
| `TMUX_POPUP_CONTROL_SESSION` | explicit session name override (env only) |
| `TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR` / `@tmux-popup-control-session-storage-dir` | override save/restore storage directory (env only / tmux option) |
| `TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS` / `@tmux-popup-control-restore-pane-contents` | enable pane content capture during save (env only / tmux option) |

### CLI subcommands

| Subcommand | Purpose |
|---|---|
| `save-sessions [--name NAME]` | save all sessions to a snapshot; opens a progress popup |
| `restore-sessions [--from NAME]` | restore sessions from a snapshot; opens a progress popup |
| `install-and-init-plugins` | sources installed plugins at tmux startup; opens a deferred install popup for any missing plugins |
| `deferred-install` | internal helper invoked via `run-shell -b`; waits for tmux startup, then opens the install UI in a `display-popup` |
| `--version` | prints the version string and exits |

### Migrating from tpm

Replace the tpm `run` line in `~/.tmux.conf`:

```tmux
# before
run '~/.tmux/plugins/tpm/tpm'

# after
run '/path/to/tmux-popup-control install-and-init-plugins'
```

Keep all `set -g @plugin '...'` declarations unchanged — tmux-popup-control
reads the same format.

## Architecture

See `CLAUDE.md` for detailed architecture notes. In brief:

```
main.go                   CLI entry point, config, signal handling, plugin subcommands
internal/config/          CLI flags + env vars → Config struct
internal/app/             wires backend + UI model, runs tea.Program
internal/backend/         polls tmux state via goroutines
internal/state/           thread-safe in-memory stores (sessions, windows, panes)
internal/tmux/            tmux operations via gotmuxcc control-mode + exec fallback
internal/menu/            menu tree definitions, loaders, action handlers
internal/resurrect/       save/restore orchestration, storage, pane archives
internal/ui/              Bubble Tea model, split across focused files
internal/ui/state/        per-level items, cursor, filter, selection, viewport
internal/format/table/    columnar table formatting with alignment
internal/plugin/          plugin management (discover, install, update, uninstall, source)
internal/theme/           lipgloss styles
internal/logging/         structured JSON log file + trace events
internal/testutil/        integration test helpers
```

## Dependencies

- [bubbletea v2](https://github.com/charmbracelet/bubbletea) — terminal UI framework
- [bubbles v2](https://github.com/charmbracelet/bubbles) — text input, cursor components
- [lipgloss v2](https://github.com/charmbracelet/lipgloss) — terminal styling and layout
- [charmbracelet/x/ansi](https://github.com/charmbracelet/x) — ANSI-aware string operations
- [gotmuxcc](https://github.com/atomicstack/gotmuxcc) — tmux control-mode client (vendored)
- [fuzzysearch](https://github.com/lithammer/fuzzysearch) — fuzzy string matching
