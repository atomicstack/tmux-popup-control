package tmux

import (
	"fmt"
	"strconv"
	"time"
)

// ServerStartTime returns the tmux server's start time, read from the
// #{start_time} format variable. Returns a zero time and an error if the
// socket is unreachable or the value cannot be parsed.
func ServerStartTime(socketPath string) (time.Time, error) {
	raw, err := ExpandFormat(socketPath, "", "#{start_time}")
	if err != nil {
		return time.Time{}, err
	}
	if raw == "" {
		return time.Time{}, fmt.Errorf("empty start_time")
	}
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse start_time %q: %w", raw, err)
	}
	return time.Unix(secs, 0), nil
}
