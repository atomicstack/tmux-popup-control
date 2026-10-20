#!/usr/bin/env bash

CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BINARY_PATH="$CURRENT_DIR/tmux-popup-control"
BINARY_NAME=$(basename "$BINARY_PATH")

if [[ ! -x "$BINARY_PATH" ]]; then
  echo "can't find binary at $BINARY_PATH, calling make build..." 1>&2
  ( cd "$CURRENT_DIR"; go mod vendor; make build )
  if [[ ! -x "$BINARY_PATH" ]]; then
    echo "make build doesn't seem to have done anything, bailing early!" 1>&2
    exit 1
  fi
fi

# Load all @tmux-popup-control-* options in one round trip. -F prints values
# raw, where plain show-options would quote and escape them.
declare -A TMUX_OPTS
while IFS='=' read -r key val; do
  TMUX_OPTS["$key"]="$val"
done < <(tmux show-options -g -F '#{option_name}=#{option_value}' 2>/dev/null | grep '^@tmux-popup-control-')

# key resolves a key binding: env var, then tmux option, then the default.
key() {
  local env_name="$1" option="$2" default="$3"
  local value="${!env_name}"
  [[ -z "$value" ]] && value="${TMUX_OPTS["@tmux-popup-control-$option"]}"
  echo "${value:-$default}"
}

LAUNCH_KEY="$(key TMUX_POPUP_CONTROL_LAUNCH_KEY launch-key Space)"
KEY_COMMAND_MENU="$(key TMUX_POPUP_CONTROL_KEY_COMMAND_MENU key-command-menu :)"
KEY_SESSION_TREE="$(key TMUX_POPUP_CONTROL_KEY_SESSION_TREE key-session-tree s)"
KEY_PANE_SWITCHER="$(key TMUX_POPUP_CONTROL_KEY_PANE_SWITCHER key-pane-switcher f)"
KEY_WINDOW_SWITCHER="$(key TMUX_POPUP_CONTROL_KEY_WINDOW_SWITCHER key-window-switcher w)"
KEY_PANE_CAPTURE="$(key TMUX_POPUP_CONTROL_KEY_PANE_CAPTURE key-pane-capture H)"
KEY_RESURRECT_SAVE="$(key TMUX_POPUP_CONTROL_KEY_RESURRECT_SAVE key-resurrect-save C-s)"
KEY_RESURRECT_RESTORE_FROM="$(key TMUX_POPUP_CONTROL_KEY_RESURRECT_RESTORE_FROM key-resurrect-restore-from C-r)"
KEY_SESSION_RENAME="$(key TMUX_POPUP_CONTROL_KEY_SESSION_RENAME key-session-rename '$')"
KEY_WINDOW_RENAME="$(key TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME key-window-rename ,)"
KEY_EXTRACT="$(key TMUX_POPUP_CONTROL_KEY_EXTRACT key-extract Tab)"

# The popup launcher, stored as an option and expanded by each binding with
# run-shell -C '#{E:@tmux-popup-control-launch} ...'. run-shell -C expands the
# formats and runs the result as a tmux command inside the server, so a key
# press costs no shell and no tmux client process: just the binary in a
# centred, modal (-O) floating pane that captures every key (-K). q/a escapes
# names for tmux's command parser, and the style flags are only passed when
# their option is set, leaving tmux's defaults otherwise. With more than one
# argument new-pane execs the binary directly, without sh -c.
LAUNCH='new-pane -t #{pane_id} -O -K -x 90% -y 80% -X 5% -Y 10% -T tmux-popup-control'
LAUNCH+=' #{?@tmux-popup-control-popup-border-lines,-B #{q/a:@tmux-popup-control-popup-border-lines},}'
LAUNCH+=' #{?@tmux-popup-control-popup-border-style,-S #{q/a:@tmux-popup-control-popup-border-style} -R #{q/a:@tmux-popup-control-popup-border-style},}'
LAUNCH+=' #{?@tmux-popup-control-popup-style,-s #{q/a:@tmux-popup-control-popup-style},}'
LAUNCH+=' -e TMUX_POPUP_CONTROL_CLIENT=#{q/a:client_tty}'
LAUNCH+=' -e TMUX_POPUP_CONTROL_SESSION=#{q/a:session_name}'
LAUNCH+=' -e TMUX_POPUP_CONTROL_SESSION_ID=#{session_id}'
LAUNCH+=' -e TMUX_POPUP_CONTROL_PANE_ID=#{pane_id}'
LAUNCH+=' #{q/a:@tmux-popup-control-binary-path} -socket #{q/a:socket_path}'

# Autosave is scheduled by tmux itself: a monitor hook evaluates this format
# every second and fires when it turns true, i.e. once the server has been up
# for an interval past the last save (#{t/d:start_time} is the uptime in
# seconds). autosave-now records the uptime in -last before saving, which
# makes the format false again. The interval option is read live, and 0 or
# unset never fires.
AUTOSAVE_DUE='#{&&:#{e|>:#{@tmux-popup-control-autosave-interval-minutes},0},#{e|>=:#{t/d:start_time},#{e|+:#{@tmux-popup-control-autosave-last},#{e|*:60,#{@tmux-popup-control-autosave-interval-minutes}}}}}'
AUTOSAVE_NOW='run-shell -b "#{q:@tmux-popup-control-binary-path} autosave-now -socket #{q:socket_path}"'

BINDINGS_FILE="$(mktemp "${TMPDIR:-/tmp}/tmux-popup-control-bindings.XXXXXX")"
cleanup() {
  rm -f "$BINDINGS_FILE"
}
trap cleanup EXIT

bind() {
  local key="$1" note="$2" args="$3"
  printf 'bind-key -T prefix -N "%s" "%s" run-shell -C %s\n' "$note" "$key" "'#{E:@tmux-popup-control-launch}${args:+ $args}'"
}

{
  printf 'set-option -gq @tmux-popup-control-binary-path "%s"\n' "$BINARY_PATH"
  printf 'set-option -gq @tmux-popup-control-launch %s\n' "'$LAUNCH'"
  bind "$LAUNCH_KEY" "Launches $BINARY_NAME" ""
  bind "$KEY_COMMAND_MENU" "Launches $BINARY_NAME's command menu" "--root-menu command"
  bind "$KEY_SESSION_TREE" "Launches $BINARY_NAME's session tree" "--root-menu session:tree"
  bind "$KEY_PANE_SWITCHER" "Launches $BINARY_NAME's pane switcher" "--root-menu pane:switch"
  bind "$KEY_WINDOW_SWITCHER" "Launches $BINARY_NAME's window switcher" "--root-menu window:switch"
  bind "$KEY_PANE_CAPTURE" "Captures pane to file via $BINARY_NAME" "--root-menu pane:capture"
  bind "$KEY_RESURRECT_SAVE" "Saves sessions via $BINARY_NAME" "--root-menu resurrect:save"
  bind "$KEY_RESURRECT_RESTORE_FROM" "Restores sessions from a snapshot via $BINARY_NAME" "--root-menu resurrect:restore-from"
  bind "$KEY_SESSION_RENAME" "Renames session via $BINARY_NAME" "--root-menu session:rename --menu-args #{q/a:session_name}"
  bind "$KEY_WINDOW_RENAME" "Renames window via $BINARY_NAME" "--root-menu window:rename --menu-args #{q/a:session_name}:#{window_index}"
  bind "$KEY_EXTRACT" "Extracts tokens from the current pane via $BINARY_NAME" "--root-menu extract"
  # Re-sourcing replaces the monitor, and a monitor never fires for the
  # value it first sees, so a save that is already due runs straight away.
  printf 'set-hook -g -T -B %s %s\n' "'@tmux-popup-control-autosave::$AUTOSAVE_DUE'" "'$AUTOSAVE_NOW'"
  printf 'if-shell -F %s %s\n' "'$AUTOSAVE_DUE'" "'$AUTOSAVE_NOW'"
} >"$BINDINGS_FILE"

tmux source-file "$BINDINGS_FILE"
