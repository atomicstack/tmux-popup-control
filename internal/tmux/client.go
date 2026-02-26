package tmux

import (
	"os"
	"strings"
)

// CurrentClientID discovers the TTY client that launched the popup so
// SwitchClient commands can target the user's visible tmux client instead
// of the control-mode connection.
//
// Approach: determine the session the popup belongs to, then find a
// non-control-mode client attached to that session via ListClients.
//
// Inside display-popup, TMUX_PANE is empty but TMUX is set to
// "socket,pid,session_id". We extract the session ID and use it as the
// target for display-message to resolve the session name.
func CurrentClientID(socketPath string) string {
	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}

	sessionName := popupSessionName(client)
	if sessionName == "" {
		return ""
	}

	// Find a non-control-mode client attached to that session.
	clients, err := client.ListClients()
	if err != nil {
		return ""
	}
	for _, c := range clients {
		if c == nil || c.ControlMode {
			continue
		}
		if strings.TrimSpace(c.Session) == sessionName {
			name := strings.TrimSpace(c.Name)
			if isValidClientName(name) {
				return name
			}
		}
	}
	return ""
}

// popupSessionName determines which session the popup belongs to.
// It tries TMUX_PANE first (set in regular panes), then falls back to
// parsing the session ID from the TMUX env var (set in display-popup).
func popupSessionName(client tmuxClient) string {
	// Try TMUX_PANE first — set in regular pane contexts.
	if pane := strings.TrimSpace(os.Getenv("TMUX_PANE")); pane != "" {
		name, err := client.DisplayMessage(pane, "#{session_name}")
		if err == nil {
			if n := strings.TrimSpace(name); n != "" {
				return n
			}
		}
	}

	// Inside display-popup TMUX_PANE is empty, but TMUX is set to
	// "socket,pid,session_id". Use $<session_id> as the target.
	tmuxEnv := os.Getenv("TMUX")
	parts := strings.Split(tmuxEnv, ",")
	if len(parts) >= 3 {
		sessionID := "$" + strings.TrimSpace(parts[2])
		name, err := client.DisplayMessage(sessionID, "#{session_name}")
		if err == nil {
			if n := strings.TrimSpace(name); n != "" {
				return n
			}
		}
	}

	return ""
}

// isValidClientName checks that name looks like a real tmux client
// identifier. Client names are either device paths (/dev/ttys004) or
// internal names (client-67503). This guards against passing garbage
// (e.g. status-line text) as a -c argument to switch-client.
func isValidClientName(name string) bool {
	if name == "" {
		return false
	}
	// Client names never contain whitespace.
	if strings.ContainsAny(name, " \t\n") {
		return false
	}
	return true
}
