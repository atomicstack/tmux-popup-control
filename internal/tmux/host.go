package tmux

import (
	"os"
	"strings"
)

// hostPaneID returns the pane ID (e.g. "%4") of the pane that was active
// when the popup was opened. main.sh captures #{pane_id} before opening
// display-popup and passes it as TMUX_POPUP_CONTROL_PANE_ID.
func hostPaneID() string {
	return strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_ID"))
}

// hostSessionID returns the session ID of the popup's host session in
// tmux's "$N" format (e.g. "$1"). It checks TMUX_POPUP_CONTROL_SESSION_ID
// (set by main.sh) first, then falls back to parsing the TMUX env var.
func hostSessionID() string {
	if id := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_SESSION_ID")); id != "" {
		if !strings.HasPrefix(id, "$") {
			id = "$" + id
		}
		return id
	}
	parts := strings.Split(os.Getenv("TMUX"), ",")
	if len(parts) >= 3 {
		if id := strings.TrimSpace(parts[2]); id != "" {
			return "$" + id
		}
	}
	return ""
}

func currentSessionName(client tmuxClient) string {
	// Prefer the session ID — stable across renames.
	if id := hostSessionID(); id != "" {
		if sessions, err := client.ListSessions(); err == nil {
			for _, s := range sessions {
				if s.Id == id {
					return s.Name
				}
			}
		}
	}
	if name := popupSessionName(client); name != "" {
		return name
	}
	if clients, err := client.ListClients(); err == nil {
		for _, c := range clients {
			if c != nil && c.Session != "" {
				return c.Session
			}
		}
	}
	return ""
}
