#!/usr/bin/env bash

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
LAUNCH_SCRIPT="$CURRENT_DIR/main.sh"

BINARY_PATH="$CURRENT_DIR/tmux-popup-control"

if [[ ! -x "$BINARY_PATH" ]]; then
  echo "can't find binary at $BINARY_PATH, calling make build..." 1>&2
  ( cd "$CURRENT_DIR"; go mod vendor; make build )
  if [[ ! -x "$BINARY_PATH" ]]; then
    echo "make build doesn't seem to have done anything, bailing early!" 1>&2
    exit 1
  fi
fi

# if [ -x "$(command -v copyq)" ]; then
#   copyq &>/dev/null &
# fi

[[ -z "$TMUX_POPUP_CONTROL_LAUNCH_KEY" ]] && TMUX_POPUP_CONTROL_LAUNCH_KEY="F"
tmux bind-key -N "Launches $LAUNCH_SCRIPT" -T prefix "$TMUX_POPUP_CONTROL_LAUNCH_KEY" run-shell -b "$LAUNCH_SCRIPT"
