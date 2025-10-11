# tmux-popup-control [BETA]

This project is a a vibe-coded re-implementation of [tmux-fzf by @sainnhe](https://github.com/sainnhe/tmux-fzf), using [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Prerequisites

- Go 1.21+
- `tmux` available in `$PATH`

## Building and running

The repository is configured to keep Go build artifacts inside the workspace so
it works cleanly in sandboxed environments. Use the supplied Makefile targets:

```sh
make build    # builds the binary ./tmux-popup-control
make run      # runs the application
make tidy     # refreshes go.mod/go.sum
make fmt      # gofmt on the repository
```

Each target ensures local caches exist at `.gocache` and `.gomodcache` and sets
`GOFLAGS=-modcacherw` so Go can write to them.

To clear the local caches, run:

```sh
make clean-cache
```

## Configuration

- Specify a tmux socket explicitly with `--socket /path/to/socket`.
- Alternatively set `TMUX_POPUP_SOCKET` or rely on the active `$TMUX` value.
- Launch directly into a submenu with `--root-menu window` (or any other menu
  identifier such as `pane:swap`). Set `TMUX_POPUP_CONTROL_ROOT_MENU` to apply
  the same override via the environment.

## Logging

Errors and trace output are written to a log file. By default the file is
`tmux-popup-control.log` in the working directory, but you can change this via
the `--log-file` command-line option or by setting the
`TMUX_POPUP_CONTROL_LOG` environment variable.

## Changelog

- Refactored the UI layer: `internal/ui/model.go` is now decomposed into
  focused files (`commands.go`, `navigation.go`, `input.go`, `view.go`,
  `prompt.go`, `forms.go`, `backend.go`) with a supporting `doc.go`, making
  message handling, prompts, and rendering easier to reason about and test.
- Moved menu state management into the `internal/ui/state` package and split
  responsibilities across `level.go`, `selection.go`, `cursor.go`, `filter.go`,
  and `items.go`, alongside new cursor/filter unit tests to raise coverage.
- Reworked the tmux client package into modular files (`types.go`,
  `snapshots.go`, `windows.go`, `panes.go`, `sessions.go`, `command.go`),
  expanded the fake client, and added extensive unit/integration coverage;
  session detaches now skip when no clients are attached.
