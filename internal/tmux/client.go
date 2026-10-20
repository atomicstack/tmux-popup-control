package tmux

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// CurrentClientID discovers the TTY client that launched the popup so
// SwitchClient commands can target the user's visible tmux client instead
// of the control-mode connection. The launcher normally passes the client in
// TMUX_POPUP_CONTROL_CLIENT; this is the fallback, which picks a terminal
// client attached to the popup's host session.
func CurrentClientID(socketPath string) string {
	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}
	sessionName := hostSessionName(context.Background(), client)
	if sessionName == "" {
		return ""
	}
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

// FindTerminalClient returns the name of the first non-control-mode client
// attached to the tmux server, or an error if none is found.
func FindTerminalClient(socketPath string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	clients, err := client.ListClients()
	if err != nil {
		return "", err
	}
	if preferred := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_CLIENT")); isValidClientName(preferred) {
		for _, c := range clients {
			if c == nil || c.ControlMode {
				continue
			}
			if strings.TrimSpace(c.Name) == preferred {
				return preferred, nil
			}
		}
	}
	for _, c := range clients {
		if c == nil || c.ControlMode {
			continue
		}
		name := strings.TrimSpace(c.Name)
		if isValidClientName(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("no terminal client found")
}

// ClientSessionInfo looks up a specific client by name and returns its current
// session and last-session. Returns empty strings if the client is not found.
func ClientSessionInfo(socketPath, clientID string) (session, lastSession string) {
	if clientID == "" {
		return "", ""
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return "", ""
	}
	clients, err := client.ListClients()
	if err != nil {
		return "", ""
	}
	for _, c := range clients {
		if c == nil {
			continue
		}
		if strings.TrimSpace(c.Name) == clientID {
			return strings.TrimSpace(c.Session), strings.TrimSpace(c.LastSession)
		}
	}
	return "", ""
}

// UserOptions returns the sorted set of user-defined option names (those
// starting with `@`) visible at the server-global scope. It is the live
// counterpart to the static tmuxopts catalog: the catalog knows every
// documented option but has no way of listing @-options because they are
// whatever the user's tmux.conf declares. Duplicates are removed and the
// result is returned in lexical order so the completion dropdown is stable.
func UserOptions(socketPath string) ([]string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	opts, err := client.Options("", "-g")
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(opts))
	names := make([]string, 0, len(opts))
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		key := strings.TrimSpace(opt.Key)
		if !strings.HasPrefix(key, "@") {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, key)
	}
	slices.Sort(names)
	return names, nil
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
