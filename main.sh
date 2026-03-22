#!/usr/bin/env bash

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CMD="$CURRENT_DIR/tmux-popup-control"

POPUP_HINTS="$(tmux display-message -p '#{client_tty},#{session_name}')"
POPUP_CLIENT="${POPUP_HINTS%%,*}"
POPUP_SESSION="${POPUP_HINTS#*,}"

# Options that the Go binary reads only from env vars (no ShowOption fallback)
# need to be propagated from tmux options into the display-popup environment.
# Most options (format, filter, switch-current, storage-dir, pane-contents)
# are handled in Go via envOrOption/tmuxOptionFn, so they don't need
# propagation here.
EXTRA_ENV=()

# Footer is read in config.go from env only, so propagate it.
if [[ -z "$TMUX_POPUP_CONTROL_FOOTER" ]]; then
  val="$(tmux show-option -gqv @tmux-popup-control-footer 2>/dev/null)"
  [[ -n "$val" ]] && EXTRA_ENV+=(-e "TMUX_POPUP_CONTROL_FOOTER=$val")
fi

tmux display-popup -w 90% -h 80% \
  -e "TMUX_POPUP_CONTROL_CLIENT=$POPUP_CLIENT" \
  -e "TMUX_POPUP_CONTROL_SESSION=$POPUP_SESSION" \
  "${EXTRA_ENV[@]}" \
  `# -e GOTMUXCC_TRACE=1 -e GOTMUXCC_TRACE_FILE=$CURRENT_DIR/gotmuxcc_trace.log --trace` \
  -E $CMD "$@"
status=$?
if [ "$status" -eq 129 ]; then
  exit 0
fi
exit "$status"
