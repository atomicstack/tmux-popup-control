#!/usr/bin/env bash

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BINARY_PATH="$CURRENT_DIR/tmux-popup-control"
BINARY_NAME=$(basename "$BINARY_PATH")
LAUNCH_SCRIPT="$CURRENT_DIR/main.sh"

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

# Load all @tmux-popup-control-* options in one shot and populate an
# associative array so we never call tmux show-option more than once.
declare -A TMUX_OPTS
while IFS=$'\t' read -r key val; do
  # tmux show-options -g outputs "option value" — strip the leading @.
  key="${key#@}"
  # Values may be wrapped in double quotes with \$ escapes; clean them.
  val="${val#\"}"
  val="${val%\"}"
  val="${val//\\$/\$}"
  TMUX_OPTS["$key"]="$val"
done < <(tmux show-options -g 2>/dev/null | grep '^@tmux-popup-control-' | sed 's/ /\t/')

# opt returns the value for a @tmux-popup-control-* option, or empty string.
opt() { echo "${TMUX_OPTS["tmux-popup-control-$1"]}"; }

[[ -z "$TMUX_POPUP_CONTROL_LAUNCH_KEY" ]] && TMUX_POPUP_CONTROL_LAUNCH_KEY="$(opt launch-key)"
[[ -z "$TMUX_POPUP_CONTROL_LAUNCH_KEY" ]] && TMUX_POPUP_CONTROL_LAUNCH_KEY="F"
tmux bind-key -T prefix -N "Launches $BINARY_NAME via $LAUNCH_SCRIPT" "$TMUX_POPUP_CONTROL_LAUNCH_KEY" run-shell -b "$LAUNCH_SCRIPT"

[[ -z "$TMUX_POPUP_CONTROL_KEY_COMMAND_MENU" ]] && TMUX_POPUP_CONTROL_KEY_COMMAND_MENU="$(opt key-command-menu)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_COMMAND_MENU" ]] && TMUX_POPUP_CONTROL_KEY_COMMAND_MENU=":"
tmux bind-key -T prefix -N "Launches $BINARY_NAME's command menu" "$TMUX_POPUP_CONTROL_KEY_COMMAND_MENU" run-shell -b "$LAUNCH_SCRIPT --root-menu=command"

[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_TREE" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_TREE="$(opt key-session-tree)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_TREE" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_TREE='s'
tmux bind-key -T prefix -N "Launches $BINARY_NAME's session tree" "$TMUX_POPUP_CONTROL_KEY_SESSION_TREE" run-shell -b "$LAUNCH_SCRIPT --root-menu session:tree"

[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER" ]] && TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER="$(opt key-pane-switcher)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER" ]] && TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER='f'
tmux bind-key -T prefix -N "Launches $BINARY_NAME's pane switcher" "$TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER" run-shell -b "$LAUNCH_SCRIPT --root-menu pane:switch"

[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE" ]] && TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE="$(opt key-pane-capture)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE" ]] && TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE='H'
tmux bind-key -T prefix -N "Captures pane to file via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE" run-shell -b "$LAUNCH_SCRIPT --root-menu pane:capture"

[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_SAVE" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_SAVE="$(opt key-session-save)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_SAVE" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_SAVE='C-s'
tmux bind-key -T prefix -N "Saves sessions via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_SESSION_SAVE" run-shell -b "$LAUNCH_SCRIPT --root-menu session:save"

[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RESTORE_FROM" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RESTORE_FROM="$(opt key-session-restore-from)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RESTORE_FROM" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RESTORE_FROM='C-r'
tmux bind-key -T prefix -N "Restores sessions from a snapshot via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_SESSION_RESTORE_FROM" run-shell -b "$LAUNCH_SCRIPT --root-menu session:restore-from"
