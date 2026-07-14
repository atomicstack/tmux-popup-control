# tmux-popup-control

Current version: **v0.14.2** — see the [release
notes](https://github.com/atomicstack/tmux-popup-control/releases/tag/v0.14.2)
for the latest changes.

A terminal UI for managing tmux sessions, windows, panes, and plugins from
inside a `tmux display-popup`. Built with [Bubble
Tea](https://github.com/charmbracelet/bubbletea) and [Lip
Gloss](https://github.com/charmbracelet/lipgloss) (v2), using a persistent
control-mode connection via
[gotmuxcc](https://github.com/atomicstack/gotmuxcc). Inspired by
[tmux-fzf](https://github.com/sainnhe/tmux-fzf),
[tpm](https://github.com/tmux-plugins/tpm), and
[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect).

## Features

### Session management
- **Switch** between sessions with live pane-capture preview, with the
  session tree initially focused on the currently-attached entry
- **New** session creation via inline form
- **Rename** sessions via inline form
- **Kill** sessions
- **Detach** clients from sessions
- **Tree view** — full session/window/pane hierarchy with expand/collapse,
  multi-word fuzzy filtering across the tree, and per-node live previews;
  marks the currently-attached window with a `(current)` suffix and
  singularises the pane count for single-pane windows

### Resurrect (save / restore)
- **Save** sessions — auto-timestamped or named snapshots of all sessions,
  windows, panes, layouts, and optionally pane contents; supports
  interval-based autosaves with bounded retention
- **Save as…** — inline form to name a snapshot
- **Restore** sessions — from the most recent save, with progress UI;
  merges windows into existing sessions idempotently
- **Restore from…** — pick any snapshot from the picker, with manual vs
  autosaved snapshots colour-coded; restore timestamps include seconds
- **Delete saved** snapshots — multi-select picker for pruning unwanted
  snapshots, gated behind a y/n confirmation before anything is removed

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
- **Capture** pane scrollback to file with configurable template path
  (supports tmux format variables and strftime tokens)

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
- **Customize-mode** — opens tmux's built-in `customize-mode` directly
- **Keybinding** browser — lists all tmux key bindings, filterable
- **Command** browser — lists all tmux commands, filterable, with contextual
  argument completion for flags, flag values, and positional parameters;
  preserves `tmux lscm` flag order, keeps repeatable flags available, uses
  Tab as the only completion accept key, and offers completion for tmux
  option names, hook names, and enumerated option values via a built-in
  catalog covering every documented option, hook, and value with
  descriptions (`set-option`, `set-window-option`, `set-hook`,
  `show-options`, etc.); option and hook names, completion candidates,
  and contextual help text are scope-coloured (server / session / window /
  pane / user), colour values render inline rather than as swatch blocks,
  and a live user-option loader exposes user-defined `@…` options

### Extract (extrakto-style)
- Captures the originating pane's visible screen and extracts tokens to
  fuzzy-find, then insert or copy — retype paths, URLs, git hashes, and
  command output without reaching for the mouse (works over SSH, since it
  operates on captured text)
- Token categories (in cycle order): **word**, **path**, **line**, **quote**,
  **s-quote**, **quoted** (inner text of `"…"`/`'…'`), **url**, **host**
  (hostname from urls — `scheme://`, `user@host:` scp, etc.), and **all**
  (path ∪ url ∪ quote ∪ s-quote). Patterns ported from extrakto's filter
  definitions
- Grab areas (capture scope): **viewport** (current pane, visible screen —
  default), **pane-history** (current pane, full scrollback), **window** (every
  pane in the current window, viewport), and **window-history** (every pane,
  full scrollback)
- `Ctrl-F` opens the token-mode selector popup and `Ctrl-G` opens the grab-area
  selector popup; the bottom bar shows both as `mode: <current> <^f>   area:
  <current> <^g>`. Each hotkey or the arrow keys cycle its selector, re-extracting
  in place while preserving your filter query. The popup auto-dismisses after 1s
  or on `Enter`; `Esc` reverts to the previous value (only one popup opens at a time)
- `Enter` inserts the selection into the originating pane; `Tab` / `Ctrl-Y`
  copies it to a tmux buffer **and the system clipboard**; `Shift-Tab` marks
  multiple tokens (joined with spaces, or newlines for the line/all categories)
- System-clipboard copy detects the host OS and shells out to the native tool
  (`pbcopy` on macOS; `wl-copy`/`xclip`/`xsel` on Linux; `clip` on Windows). The
  tmux buffer stays the source of truth — a clipboard failure never blocks the copy
- Reachable from the root menu or directly via `--root-menu extract` (see the
  keybinding below); quits on `Esc` when invoked directly
- OSC-52 (for remote copy), edit/open actions, and `@extrakto-*` config
  compatibility are planned follow-ups

### UI
- Fuzzy-search filtering on every menu level
- Breadcrumb navigation with push/pop menu stack
- Side-by-side preview panel with ANSI rendering and mouse-wheel scrolling
- Background polling keeps menu data in sync with tmux state
- Multi-select with Tab for bulk operations
- Command prompt help line with tmux command summaries under the input field
- Command completion popup with ghost hints, aligned flag/parameter
  descriptions, wraparound navigation, viewport-sized page up/down scoped
  to the popup, stable width across scrolling (hard-capped at 50 visible
  columns so a single long candidate cannot blow the popup across the
  screen), and live tmux-backed value candidates
- Thin vertical-line scrollbar column on every long list (main menu,
  completion popup, session tree), rendered outside row highlighting so
  selection styling never bleeds into the scrollbar glyph
- Alt-screen with mouse cell-motion support
- Hybrid progress bar with gradient blocks and background fill for
  save/restore operations, smoothed at ~60 fps via a lerped display value
  so bursts of restore events do not look jerky
- Non-selectable header items for menu section grouping

## Not yet implemented

- **Process management** — menu entries exist but no action handlers are wired up
- **Clipboard** — menu entries exist but no action handlers are wired up

## Prerequisites

- Go 1.24+
- `tmux` 3.2+ available in `$PATH`

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
make release VERSION=0.7.0 # release a specific version tag
```

`make release` requires the GitHub CLI (`gh`) to be installed and authenticated.

## Configuration

| Flag | Env var | Tmux option | Purpose |
|---|---|---|---|
| `--socket` | `TMUX_POPUP_CONTROL_SOCKET` | | tmux socket path (falls back to `TMUX_POPUP_SOCKET`, then `$TMUX`) |
| `--root-menu` | `TMUX_POPUP_CONTROL_ROOT_MENU` | | open directly into a submenu (e.g. `window`, `pane:swap`, `session:tree`) |
| `--menu-args` | `TMUX_POPUP_CONTROL_MENU_ARGS` | | arguments for the target menu (e.g. `expanded` for `session:tree`) |
| `--width` | `TMUX_POPUP_CONTROL_WIDTH` | | viewport width in cells (0 = terminal width) |
| `--height` | `TMUX_POPUP_CONTROL_HEIGHT` | | viewport height in rows (0 = terminal height) |
| `--footer` | `TMUX_POPUP_CONTROL_FOOTER` | `@tmux-popup-control-footer` | show keybinding hint row |
| `--verbose` | `TMUX_POPUP_CONTROL_VERBOSE` | | print success messages for actions |
| `--no-preview` | `TMUX_POPUP_CONTROL_NO_PREVIEW` | | disable the side-by-side preview panel |
| `--log-file` | `TMUX_POPUP_CONTROL_LOG_FILE` | | log file path |
| `--trace` | `TMUX_POPUP_CONTROL_TRACE` | | enable verbose JSON trace logging |
| `--debug-to-sqlite` | | | write structured debug runs, events, and spans to `<binary>.debug.sqlite3` next to the executable |
| | `TMUX_POPUP_CONTROL_CLIENT` | | explicit client ID override |
| | `TMUX_POPUP_CONTROL_SESSION` | | explicit session name override |
| | `TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR` | `@tmux-popup-control-session-storage-dir` | override save/restore storage directory; supports `$HOME` and other env vars |
| | `TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS` | `@tmux-popup-control-restore-pane-contents` | enable pane content capture during save |
| | `TMUX_POPUP_CONTROL_SESSION_FORMAT` | `@tmux-popup-control-session-format` | custom tmux format string for session labels |
| | `TMUX_POPUP_CONTROL_WINDOW_FORMAT` | `@tmux-popup-control-window-format` | custom tmux format string for window labels |
| | `TMUX_POPUP_CONTROL_WINDOW_FILTER` | `@tmux-popup-control-window-filter` | tmux filter expression for window list |
| | `TMUX_POPUP_CONTROL_PANE_FORMAT` | `@tmux-popup-control-pane-format` | custom tmux format string for pane labels |
| | `TMUX_POPUP_CONTROL_PANE_FILTER` | `@tmux-popup-control-pane-filter` | tmux filter expression for pane list |
| | `TMUX_POPUP_CONTROL_SWITCH_CURRENT` | `@tmux-popup-control-switch-current` | include current session/window/pane in switch menus |
| | `TMUX_POPUP_CONTROL_COLOR_PROFILE` | | force colour profile (`ansi256`, etc.) |
| | `TMUX_POPUP_CONTROL_RESURRECT_NAME` | | snapshot name for save/restore CLI subcommands |
| | `TMUX_POPUP_CONTROL_RESURRECT_FROM` | | snapshot name to restore from in CLI subcommand |
| | `TMUX_POPUP_CONTROL_AUTOSAVE_INTERVAL_MINUTES` | `@tmux-popup-control-autosave-interval-minutes` | automatic save interval in minutes; `0` or unset disables autosave |
| | `TMUX_POPUP_CONTROL_AUTOSAVE_MAX` | `@tmux-popup-control-autosave-max` | maximum number of retained autosaves; manual saves are never pruned |
| | `TMUX_POPUP_CONTROL_AUTOSAVE_ICON` | `@tmux-popup-control-autosave-icon` | status-right icon shown while a save is in progress |
| | `TMUX_POPUP_CONTROL_AUTOSAVE_ICON_SECONDS` | `@tmux-popup-control-autosave-icon-seconds` | any value `> 0` enables the autosave icon; `0` or unset hides it. the icon appears when the save starts and clears one second after it finishes |

### Keybindings

`main.tmux` binds prefix keys for common actions. Each is configurable via an
env var or a tmux option in `tmux.conf` (env var takes precedence).

| Env var | Tmux option | Default | Action |
|---|---|---|---|
| `TMUX_POPUP_CONTROL_LAUNCH_KEY` | `@tmux-popup-control-launch-key` | `F` | open the main popup menu |
| `TMUX_POPUP_CONTROL_KEY_COMMAND_MENU` | `@tmux-popup-control-key-command-menu` | `:` | open the command browser |
| `TMUX_POPUP_CONTROL_KEY_SESSION_TREE` | `@tmux-popup-control-key-session-tree` | `s` | open the session tree |
| `TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER` | `@tmux-popup-control-key-pane-switcher` | `f` | open the pane switcher |
| `TMUX_POPUP_CONTROL_KEY_WINDOW_SWITCHER` | `@tmux-popup-control-key-window-switcher` | `w` | open the window switcher |
| `TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE` | `@tmux-popup-control-key-pane-capture` | `H` | capture pane to file |
| `TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE` | `@tmux-popup-control-key-resurrect-save` | `C-s` | save sessions (legacy `key-session-save` honoured as fallback) |
| `TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM` | `@tmux-popup-control-key-resurrect-restore-from` | `C-r` | restore sessions from a snapshot (legacy `key-session-restore-from` honoured as fallback) |
| `TMUX_POPUP_CONTROL_KEY_SESSION_RENAME` | `@tmux-popup-control-key-session-rename` | `$` | rename the current session via inline form |
| `TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME` | `@tmux-popup-control-key-window-rename` | `,` | rename the current window via inline form |
| `TMUX_POPUP_CONTROL_KEY_EXTRACT` | `@tmux-popup-control-key-extract` | `Tab` | extract tokens from the current pane (extrakto-style) |

### CLI subcommands

| Subcommand | Purpose |
|---|---|
| `save-sessions [--name NAME]` | save all sessions to a snapshot; opens a progress popup |
| `restore-sessions [--from NAME]` | restore sessions from a snapshot; opens a progress popup |
| `autosave [--socket PATH]` | internal helper for tmux `#()` status snippets; runs the autosave cadence and optional status icon |
| `install-and-init-plugins` | sources installed plugins at tmux startup; opens a deferred install popup for any missing plugins |
| `deferred-install` | internal helper invoked via `run-shell -b`; waits for tmux startup, then opens the install UI in a `display-popup` |
| `--version` | prints the version string and exits |

### Automatic session saves

Autosaves are disabled by default. Enable them by setting an interval and
adding the helper command to a tmux format that is evaluated regularly,
typically `status-right`. The plugin publishes its resolved binary path in
`@tmux-popup-control-binary-path` so the snippet does not need to guess where
the binary lives.

The important part to add somewhere in your status line is:

```tmux
#(#{@tmux-popup-control-binary-path} autosave -socket '#{socket_path}')
```

A reusable `.tmux.conf` setup looks like this:

```tmux
set -g @tmux-popup-control-autosave-interval-minutes 5
set -g @tmux-popup-control-autosave-max 5
set -g @tmux-popup-control-autosave-icon "💾"
set -g @tmux-popup-control-autosave-icon-seconds 1  # any value > 0 enables the icon
set -g @status-right-autosave "#(#{@tmux-popup-control-binary-path} autosave -socket '#{socket_path}')"
set -ag status-right "#{E:@status-right-autosave}"
```

If you do not want the extra `@status-right-autosave` helper option, you can
append it directly instead:

```tmux
set -ag status-right "#(#{@tmux-popup-control-binary-path} autosave -socket '#{socket_path}')"
```

Autosaves are stored as `auto-YYYY-MM-DDTHH-MM-SS` snapshots, update the
default restore target, and are pruned independently from manual saves. The
restore-from picker shows both snapshot types and colors them differently so
they are easy to distinguish at a glance.

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
internal/cmdparse/        tmux command synopsis parsing, completion analysis, and value resolution
internal/cmdhelp/         checked-in tmux command summaries and flag/parameter help data
internal/resurrect/       save/restore orchestration, storage, pane archives
internal/ui/              Bubble Tea model, split across focused files
internal/ui/state/        per-level items, cursor, filter, selection, viewport
internal/format/table/    columnar table formatting with alignment
internal/plugin/          plugin management (discover, install, update, uninstall, source)
internal/theme/           lipgloss styles
internal/logging/         structured JSON log file, trace events, and SQLite debug sink
internal/testutil/        integration test helpers
```

## Dependencies

- [bubbletea v2](https://github.com/charmbracelet/bubbletea) — terminal UI framework
- [bubbles v2](https://github.com/charmbracelet/bubbles) — text input, cursor components
- [lipgloss v2](https://github.com/charmbracelet/lipgloss) — terminal styling and layout
- [charmbracelet/x/ansi](https://github.com/charmbracelet/x) — ANSI-aware string operations
- [gotmuxcc](https://github.com/atomicstack/gotmuxcc) — tmux control-mode client (vendored)
- [fuzzysearch](https://github.com/lithammer/fuzzysearch) — fuzzy string matching
