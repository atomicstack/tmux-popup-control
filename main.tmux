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

[[ -z "$TMUX_POPUP_CONTROL_KEY_COMMAND_MENU" ]] && TMUX_POPUP_CONTROL_KEY_COMMAND_MENU="$(opt key-command-menu)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_COMMAND_MENU" ]] && TMUX_POPUP_CONTROL_KEY_COMMAND_MENU=":"

[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_TREE" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_TREE="$(opt key-session-tree)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_TREE" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_TREE='s'

[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER" ]] && TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER="$(opt key-pane-switcher)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER" ]] && TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER='f'

[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_SWITCHER" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_SWITCHER="$(opt key-window-switcher)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_SWITCHER" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_SWITCHER='w'

[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE" ]] && TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE="$(opt key-pane-capture)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE" ]] && TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE='H'

# resurrect (save/restore/delete) hotkeys. The legacy
# @tmux-popup-control-key-session-save / -session-restore-from option
# names are still honoured as fallbacks so existing tmux.conf entries
# keep working after the menu reorg.
[[ -z "$TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE" ]] && TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE="$(opt key-resurrect-save)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE" ]] && TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE="$(opt key-session-save)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE" ]] && TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE='C-s'

[[ -z "$TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM" ]] && TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM="$(opt key-resurrect-restore-from)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM" ]] && TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM="$(opt key-session-restore-from)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM" ]] && TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM='C-r'

[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RENAME="$(opt key-session-rename)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RENAME='$'

[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME="$(opt key-window-rename)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME=','

BINDINGS_FILE="$(mktemp "${TMPDIR:-/tmp}/tmux-popup-control-bindings.XXXXXX")"
cleanup() {
  rm -f "$BINDINGS_FILE"
}
trap cleanup EXIT

cat >"$BINDINGS_FILE" <<EOF
set-option -gq @tmux-popup-control-binary-path "$BINARY_PATH"
bind-key -T prefix -N "Launches $BINARY_NAME via $LAUNCH_SCRIPT" "$TMUX_POPUP_CONTROL_LAUNCH_KEY" run-shell -b "$LAUNCH_SCRIPT"
bind-key -T prefix -N "Launches $BINARY_NAME's command menu" "$TMUX_POPUP_CONTROL_KEY_COMMAND_MENU" run-shell -b "$LAUNCH_SCRIPT --root-menu=command"
bind-key -T prefix -N "Launches $BINARY_NAME's session tree" "$TMUX_POPUP_CONTROL_KEY_SESSION_TREE" run-shell -b "$LAUNCH_SCRIPT --root-menu session:tree"
bind-key -T prefix -N "Launches $BINARY_NAME's pane switcher" "$TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER" run-shell -b "$LAUNCH_SCRIPT --root-menu pane:switch"
bind-key -T prefix -N "Launches $BINARY_NAME's window switcher" "$TMUX_POPUP_CONTROL_KEY_WINDOW_SWITCHER" run-shell -b "$LAUNCH_SCRIPT --root-menu window:switch"
bind-key -T prefix -N "Captures pane to file via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE" run-shell -b "$LAUNCH_SCRIPT --root-menu pane:capture"
bind-key -T prefix -N "Saves sessions via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE" run-shell -b "$LAUNCH_SCRIPT --root-menu resurrect:save"
bind-key -T prefix -N "Restores sessions from a snapshot via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM" run-shell -b "$LAUNCH_SCRIPT --root-menu resurrect:restore-from"
bind-key -T prefix -N "Renames session via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" run-shell -b "$LAUNCH_SCRIPT --root-menu session:rename --menu-args '#{q:session_name}'"
bind-key -T prefix -N "Renames window via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" run-shell -b "$LAUNCH_SCRIPT --root-menu window:rename --menu-args '#{q:session_name}:#{window_index}'"
EOF

tmux source-file "$BINDINGS_FILE"
