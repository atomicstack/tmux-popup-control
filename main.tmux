#!/usr/bin/env bash

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
LAUNCH_SCRIPT="$CURRENT_DIR/main.sh"

# if [ -x "$(command -v copyq)" ]; then
#   copyq &>/dev/null &
# fi

[[ -z "$TMUX_POPUP_CONTROL_LAUNCH_KEY" ]] && TMUX_POPUP_CONTROL_LAUNCH_KEY="F"
tmux bind-key -N "Launches $LAUNCH_SCRIPT" -T prefix "$TMUX_POPUP_CONTROL_LAUNCH_KEY" run-shell -b "$LAUNCH_SCRIPT"
