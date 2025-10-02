#!/usr/bin/env bash
set -euo pipefail
SOCKET=$1
shift
SESSION_NAME=${SESSION_NAME:-tpctest}
TMUX_TMPDIR=${TMUX_TMPDIR:-$(dirname "$SOCKET")}
TMUX=/usr/bin/env tmux
$TMUX -S "$SOCKET" -f /dev/null new-session -d -s "$SESSION_NAME" "$*"
