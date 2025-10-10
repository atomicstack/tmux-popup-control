package tmux

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	gotmux "github.com/GianlucaP106/gotmux/gotmux"
)

func NewSession(socketPath, name string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.NewSession(&gotmux.SessionOptions{Name: name})
	return err
}

func RenameSession(socketPath, target, newName string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	trimmedTarget := strings.TrimSpace(target)
	if trimmedTarget == "" {
		return fmt.Errorf("session target required")
	}
	session, err := findSession(client, trimmedTarget)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session %s not found", trimmedTarget)
	}
	return session.Rename(newName)
}

func DetachSessions(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	for _, target := range targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		session, err := findSession(client, trimmed)
		if err != nil {
			return err
		}
		if session == nil {
			return fmt.Errorf("session %s not found", trimmed)
		}
		hasClient, err := sessionHasClient(client, trimmed)
		if err != nil {
			return err
		}
		if !hasClient {
			continue
		}
		if err := session.Detach(); err != nil {
			return err
		}
	}
	return nil
}

func KillSessions(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	for _, target := range targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		session, err := findSession(client, trimmed)
		if err != nil {
			return err
		}
		if session == nil {
			return fmt.Errorf("session %s not found", trimmed)
		}
		if err := session.Kill(); err != nil {
			return err
		}
	}
	return nil
}

func ResolveSocketPath(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if envSocket := os.Getenv("TMUX_POPUP_SOCKET"); envSocket != "" {
		return envSocket, nil
	}
	if tmuxEnv := os.Getenv("TMUX"); tmuxEnv != "" {
		parts := strings.Split(tmuxEnv, ",")
		if len(parts) > 0 && parts[0] != "" {
			return parts[0], nil
		}
	}
	baseDir := os.Getenv("TMUX_TMPDIR")
	if baseDir == "" {
		baseDir = "/tmp"
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	return filepath.Join(baseDir, fmt.Sprintf("tmux-%s", u.Uid), "default"), nil
}

func findSession(client tmuxClient, target string) (sessionHandle, error) {
	name := target
	if idx := strings.IndexRune(target, ':'); idx >= 0 {
		name = target[:idx]
	}
	if name == "" {
		name = target
	}
	session, err := client.GetSessionByName(name)
	if err != nil {
		return nil, err
	}
	return newSessionHandle(session), nil
}

func sessionHasClient(client tmuxClient, session string) (bool, error) {
	clients, err := client.ListClients()
	if err != nil {
		return false, err
	}
	for _, c := range clients {
		if c == nil {
			continue
		}
		if strings.TrimSpace(c.Session) == session {
			return true, nil
		}
	}
	return false, nil
}
