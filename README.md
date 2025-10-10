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
