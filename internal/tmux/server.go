package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ServerStartTime returns the tmux server's start time, read from the
// #{start_time} format variable. Returns a zero time and an error if the
// socket is unreachable or the value cannot be parsed.
//
// This reads via a one-shot `tmux display-message`, not the control-mode
// connection. The only caller (autosave-status) runs roughly once per second,
// and a control-mode attach there forces a full server state-sync and, before
// the subcommand Shutdown fix, leaked the tmux -C subprocess. A plain exec is
// the appropriate last-resort path here (see CLAUDE.md control-mode principle).
func ServerStartTime(socketPath string) (time.Time, error) {
	args := append(baseArgs(socketPath), "display-message", "-p", "#{start_time}")
	output, err := runExecCommand("tmux", args...).Output()
	if err != nil {
		return time.Time{}, err
	}
	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty start_time")
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse start_time %q: %w", raw, err)
	}
	return time.Unix(secs, 0), nil
}
