#!/usr/bin/env bash

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CMD="$CURRENT_DIR/tmux-popup-control"

# One round trip for the launch context and the popup styling options. Fields
# are separated by \x1f because style values routinely contain commas.
IFS=$'\x1f' read -r POPUP_CLIENT POPUP_SESSION POPUP_SESSION_ID POPUP_PANE_ID \
  OPT_BORDER_LINES OPT_BORDER_STYLE OPT_STYLE OPT_TREE_STYLE < <(
  tmux display-message -p "#{client_tty}"$'\x1f'"#{session_name}"$'\x1f'"#{session_id}"$'\x1f'"#{pane_id}"$'\x1f'"#{@tmux-popup-control-popup-border-lines}"$'\x1f'"#{@tmux-popup-control-popup-border-style}"$'\x1f'"#{@tmux-popup-control-popup-style}"$'\x1f'"#{@tmux-popup-control-tree-style}"
)

# Options that the Go binary reads only from env vars (no ShowOption fallback)
# need to be propagated from tmux options into the popup environment.
# Most options (format, filter, switch-current, storage-dir, pane-contents)
# are handled in Go via envOrOption/tmuxOptionFn, so they don't need
# propagation here.
EXTRA_ENV=()

# Footer is read in config.go from env only, so propagate it.
if [[ -z "$TMUX_POPUP_CONTROL_FOOTER" ]]; then
  val="$(tmux show-option -gqv @tmux-popup-control-footer 2>/dev/null)"
  [[ -n "$val" ]] && EXTRA_ENV+=(-e "TMUX_POPUP_CONTROL_FOOTER=$val")
fi

# Tree style is read in config.go from env/flag only, so propagate it.
if [[ -z "$TMUX_POPUP_CONTROL_TREE_STYLE" && -n "$OPT_TREE_STYLE" ]]; then
  EXTRA_ENV+=(-e "TMUX_POPUP_CONTROL_TREE_STYLE=$OPT_TREE_STYLE")
fi

# Popup styling: env vars win over the tmux options; anything left unset is
# not passed, so tmux's own defaults apply.
STYLE_ARGS=()
border_lines="${TMUX_POPUP_CONTROL_POPUP_BORDER_LINES:-$OPT_BORDER_LINES}"
border_style="${TMUX_POPUP_CONTROL_POPUP_BORDER_STYLE:-$OPT_BORDER_STYLE}"
popup_style="${TMUX_POPUP_CONTROL_POPUP_STYLE:-$OPT_STYLE}"
[[ -n "$border_lines" ]] && STYLE_ARGS+=(-B "$border_lines")
[[ -n "$border_style" ]] && STYLE_ARGS+=(-S "$border_style" -R "$border_style")
[[ -n "$popup_style" ]] && STYLE_ARGS+=(-s "$popup_style")

# A centred, modal (-O) floating pane that captures every key (-K); -W blocks
# until the command exits and returns its exit status.
tmux new-pane -t "$POPUP_PANE_ID" -O -K -W \
  -x 90% -y 80% -X 5% -Y 10% \
  -T tmux-popup-control \
  "${STYLE_ARGS[@]}" \
  -e "TMUX_POPUP_CONTROL_CLIENT=$POPUP_CLIENT" \
  -e "TMUX_POPUP_CONTROL_SESSION=$POPUP_SESSION" \
  -e "TMUX_POPUP_CONTROL_SESSION_ID=$POPUP_SESSION_ID" \
  -e "TMUX_POPUP_CONTROL_PANE_ID=$POPUP_PANE_ID" \
  "${EXTRA_ENV[@]}" \
  `# -e GOTMUXCC_TRACE=1 -e GOTMUXCC_TRACE_FILE=$CURRENT_DIR/gotmuxcc_trace.log --trace` \
  $CMD "$@"
status=$?
if [ "$status" -eq 129 ]; then
  exit 0
fi
exit "$status"
