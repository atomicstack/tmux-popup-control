package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func NewSession(socketPath, name string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = client.NewSession(&gotmux.SessionOptions{Name: name})
	return err
}

func RenameSession(socketPath, target, newName string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	defer client.Close()
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
	defer client.Close()
	for _, target := range targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		hasClient, err := sessionHasClient(client, trimmed)
		if err != nil {
			return err
		}
		if !hasClient {
			continue
		}
		session, err := findSession(client, trimmed)
		if err != nil {
			return err
		}
		if session == nil {
			if err := detachSessionCLI(socketPath, trimmed); err != nil {
				return err
			}
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
	defer client.Close()
	for _, target := range targets {
		trimmed := strings.TrimSpace(target)
		if trimmed == "" {
			continue
		}
		session, err := findSession(client, trimmed)
		if err != nil {
			return err
		}
		if session != nil {
			if err := session.Kill(); err != nil {
				return err
			}
			if waitForSessionRemoval(socketPath, trimmed) {
				continue
			}
		}
		if err := killSessionCLI(socketPath, trimmed); err != nil {
			return err
		}
		if !waitForSessionRemoval(socketPath, trimmed) {
			return fmt.Errorf("session %s still exists after kill", trimmed)
		}
	}
	return nil
}

func waitForSessionRemoval(socketPath, name string) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !hasSessionCLI(socketPath, name) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return !hasSessionCLI(socketPath, name)
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
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		session, err := client.GetSessionByName(name)
		if err != nil {
			return nil, err
		}
		if handle := newSessionHandle(session); handle != nil {
			return handle, nil
		}
		if time.Now().After(deadline) {
			return nil, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
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

func killSessionCLI(socketPath, target string) error {
	args := append(baseArgs(socketPath), "kill-session", "-t", target)
	err := runExecCommand("tmux", args...).Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}
	return fmt.Errorf("failed to kill session %s: %w", target, err)
}

func detachSessionCLI(socketPath, target string) error {
	args := append(baseArgs(socketPath), "detach-client", "-s", target)
	err := runExecCommand("tmux", args...).Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}
	return fmt.Errorf("failed to detach session %s: %w", target, err)
}

func hasSessionCLI(socketPath, name string) bool {
	args := append(baseArgs(socketPath), "has-session", "-t", name)
	return runExecCommand("tmux", args...).Run() == nil
}
