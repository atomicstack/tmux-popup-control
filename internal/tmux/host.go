package tmux

import (
	"context"
	"os"
	"strings"
)

// hostPaneID returns the pane ID (e.g. "%4") of the pane that was active
// when the popup was opened. the main.tmux launcher captures #{pane_id} before opening
// display-popup and passes it as TMUX_POPUP_CONTROL_PANE_ID.
func hostPaneID() string {
	return strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_ID"))
}

// OriginPaneID returns the pane ID that launched the popup. Exported wrapper
// around hostPaneID for consumers outside this package (the extract feature).
func OriginPaneID() string {
	return hostPaneID()
}

// hostSessionID returns the session ID of the popup's host session in
// tmux's "$N" format (e.g. "$1"). It checks TMUX_POPUP_CONTROL_SESSION_ID
// (set by the main.tmux launcher) first, then falls back to parsing the TMUX env var.
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

// hostSessionName resolves hostSessionID to the session's current name.
func hostSessionName(ctx context.Context, client tmuxClient) string {
	id := hostSessionID()
	if id == "" {
		return ""
	}
	sessions, err := client.ListSessionsContext(ctx)
	if err != nil {
		return ""
	}
	for _, s := range sessions {
		if s.Id == id {
			return s.Name
		}
	}
	return ""
}

// currentSessionName is the popup's host session, or failing that the
// session of the first attached client.
func currentSessionName(ctx context.Context, client tmuxClient) string {
	if name := hostSessionName(ctx, client); name != "" {
		return name
	}
	if clients, err := client.ListClients(); err == nil {
		for _, c := range clients {
			if c != nil && !c.ControlMode && c.Session != "" {
				return c.Session
			}
		}
	}
	return ""
}
