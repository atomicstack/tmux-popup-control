package tmux

import (
	"os"
	"strings"
)

// CurrentClientID attempts to detect the client that launched the popup so
// SwitchClient commands can target the visible tmux client instead of the
// control-mode connection.
func CurrentClientID(socketPath string) string {
	args := append(baseArgs(socketPath), "display-message", "-p", "#{client_name}")
	if pane := strings.TrimSpace(os.Getenv("TMUX_PANE")); pane != "" {
		args = append(args, "-t", pane)
	}
	cmd := runExecCommand("tmux", args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
