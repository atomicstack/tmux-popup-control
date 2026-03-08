# tmux-popup-control [BETA]

A terminal UI for managing tmux sessions, windows, and panes from inside a
`tmux display-popup`. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
and [Lip Gloss](https://github.com/charmbracelet/lipgloss) (v2), inspired by
[tmux-fzf](https://github.com/sainnhe/tmux-fzf).

## Features

### Session management
- **Switch** between sessions with live pane-capture preview
- **New** session creation via inline form
- **Rename** sessions via inline form
- **Kill** sessions
- **Detach** clients from sessions
- **Tree view** showing the full session/window/pane hierarchy with
  expand/collapse, filtering, and per-node previews

### Window management
- **Switch** windows with live pane-capture preview
- **Rename** windows via inline form
- **Kill** windows (multi-select supported)
- **Swap** windows
- **Move** windows between sessions
- **Link** windows into other sessions

### Pane management
- **Switch** panes with live pane-capture preview
- **Rename** panes via inline form
- **Kill** panes (multi-select supported)
- **Swap** panes
- **Join** panes from other windows (multi-select supported)
- **Break** pane out to its own window
- **Resize** panes (left/right/up/down)
- **Layout** presets (even-horizontal, even-vertical, main-horizontal, etc.)

### Other menus
- **Keybinding** browser -- lists all tmux key bindings, filterable
- **Command** browser -- lists all tmux commands, filterable

### UI
- Fuzzy-search filtering on every menu level
- Breadcrumb navigation with push/pop menu stack
- Side-by-side preview panel (pane capture with ANSI rendering, mouse-wheel
  scrollable) for session/window/pane switch menus
- Background polling (~4 Hz) keeps menu data in sync with tmux state
- Multi-select with Tab for bulk operations (kill, join)
- Alt-screen with mouse cell-motion support

## Not yet implemented

- **Process management** -- menu entries exist (display, tree, terminate, kill,
  interrupt, continue, stop, quit, hangup) but no action handlers are wired up
- **Clipboard** -- menu entries exist (tmux buffers, system clipboard via copyq)
  but no action handlers are wired up
- Custom tmux format strings for session/window labels
  (`TMUX_POPUP_CONTROL_SESSION_FORMAT`, `TMUX_POPUP_CONTROL_WINDOW_FORMAT`)
- Window/pane filter expressions (`TMUX_POPUP_CONTROL_WINDOW_FILTER`)
- Toggling current session/window/pane visibility in switch menus
  (`TMUX_POPUP_CONTROL_SWITCH_CURRENT`)

## Prerequisites

- Go 1.24+
- `tmux` available in `$PATH`

## Building and running

The repository keeps Go build artifacts inside the workspace (`.gocache/`,
`.gomodcache/`) so it works cleanly in sandboxed environments. Use the Makefile:

```sh
make build    # builds the binary ./tmux-popup-control
make run      # runs the application
make test     # runs all tests
make cover    # runs tests with coverage report
make tidy     # refreshes go.mod/go.sum
make fmt      # gofmt on the repository
```

To clear the local caches:

```sh
make clean-cache
```

## Configuration

| Flag / env var | Purpose |
|---|---|
| `--socket` / `TMUX_POPUP_CONTROL_SOCKET` | tmux socket path (falls back to `$TMUX`) |
| `--root-menu` / `TMUX_POPUP_CONTROL_ROOT_MENU` | open directly into a submenu (e.g. `window`, `pane:swap`, `session:tree`) |
| `--menu-args` / `TMUX_POPUP_CONTROL_MENU_ARGS` | arguments for the target menu (e.g. `expanded` for `session:tree`) |
| `--width` / `TMUX_POPUP_CONTROL_WIDTH` | viewport width in cells (0 = terminal width) |
| `--height` / `TMUX_POPUP_CONTROL_HEIGHT` | viewport height in rows (0 = terminal height) |
| `--footer` / `TMUX_POPUP_CONTROL_FOOTER` | show keybinding hint row |
| `--verbose` / `TMUX_POPUP_CONTROL_VERBOSE` | print success messages for actions |
| `--log-file` / `TMUX_POPUP_CONTROL_LOG_FILE` | log file path |
| `--trace` / `TMUX_POPUP_CONTROL_TRACE` | enable verbose JSON trace logging |

## Architecture

See `CLAUDE.md` for detailed architecture notes. In brief:

```
main.go              CLI entry point, config loading, signal handling
internal/config/     CLI flags + env vars
internal/app/        wires backend + UI model, runs tea.Program
internal/backend/    polls tmux state (~4 Hz) via goroutines
internal/state/      thread-safe in-memory stores (sessions, windows, panes)
internal/tmux/       tmux operations via gotmuxcc control-mode + exec fallback
internal/menu/       menu tree definitions, loaders, action handlers
internal/ui/         Bubble Tea model, split across focused files
internal/theme/      lipgloss styles
internal/logging/    structured JSON log file
```

## Dependencies

- [bubbletea v2](https://charm.land/bubbletea/v2) -- terminal UI framework
- [bubbles v2](https://charm.land/bubbles/v2) -- text input, cursor components
- [lipgloss v2](https://charm.land/lipgloss/v2) -- terminal styling and layout
- [gotmuxcc](https://github.com/atomicstack/gotmuxcc) -- tmux control-mode client (vendored)
- [fuzzysearch](https://github.com/lithammer/fuzzysearch) -- fuzzy string matching
