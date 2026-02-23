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
	defer client.Close()
	target := strings.TrimSpace(os.Getenv("TMUX_PANE"))
	name, err := client.DisplayMessage(target, "#{client_name}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}
