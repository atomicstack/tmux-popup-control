package tmux

import (
	"os"
	"strings"
)

// CurrentClientID attempts to detect the client that launched the popup so
// SwitchClient commands can target the visible tmux client instead of the
// control-mode connection.
func CurrentClientID(socketPath string) string {
	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}
	target := strings.TrimSpace(os.Getenv("TMUX_PANE"))
	name, err := client.DisplayMessage(target, "#{client_name}")
	if err != nil {
		return ""
	}
	name = strings.TrimSpace(name)
	if !isValidClientName(name) {
		return ""
	}
	return name
}

// isValidClientName checks that name looks like a real tmux client path
// (e.g. /dev/ttys004). Display-message can occasionally return stale
// status-line text instead of the format result; this guards against
// passing garbage as a -c argument to switch-client.
func isValidClientName(name string) bool {
	if name == "" {
		return false
	}
	// Client names are device paths and never contain spaces.
	if strings.ContainsAny(name, " \t\n") {
		return false
	}
	// On all supported platforms client names start with '/'.
	if !strings.HasPrefix(name, "/") {
		return false
	}
	return true
}
