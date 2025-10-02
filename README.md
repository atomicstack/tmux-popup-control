# tmux-popup-control

This project is a Bubble Tea (Charm) TUI for interacting with tmux via control mode.

## Prerequisites

- Go 1.21+
- `tmux` available on `PATH`

## Building and running

The repository is configured to keep Go build artifacts inside the workspace so it works cleanly in sandboxed environments. Use the supplied Makefile targets:

```sh
make build    # builds ./...
make run      # runs the application
make tidy     # refreshes go.mod/go.sum
make fmt      # gofmt on the repository
```

Each target ensures local caches exist at `.gocache` and `.gomodcache` and sets `GOFLAGS=-modcacherw` so Go can write to them.

To clear the local caches, run:

```sh
make clean-cache
```

## Configuration

- Specify a tmux socket explicitly with `--socket /path/to/socket`.
- Alternatively set `TMUX_POPUP_SOCKET` or rely on the active `$TMUX` value.

## Logging

Errors are written to `tmux-popup-control.log` in the working directory.

