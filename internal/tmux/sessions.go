package tmux

import (
	"cmp"
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
			continue
		}
		id := session.ID()
		if err := session.Kill(); err != nil {
			return err
		}
		if waitForSessionRemoval(socketPath, id) {
			continue
		}
		if err := killSessionCLI(socketPath, trimmed, id); err != nil {
			return err
		}
		if !waitForSessionRemoval(socketPath, id) {
			return fmt.Errorf("session %s [%s] still exists after kill", trimmed, id)
		}
	}
	return nil
}

func waitForSessionRemoval(socketPath, id string) bool {
	removed := pollUntil(2*time.Second, 20*time.Millisecond, func() bool {
		return !hasSessionCLI(socketPath, id)
	})
	if removed {
		return true
	}
	return !hasSessionCLI(socketPath, id)
}

// pollUntil repeatedly invokes done until it returns true or the timeout
// elapses, sleeping interval between attempts. It returns true if done
// succeeded within the timeout. The condition is always checked at least once.
func pollUntil(timeout, interval time.Duration, done func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if done() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(interval)
	}
}

func ResolveSocketPath(flagValue string) (string, error) {
	if socket := cmp.Or(flagValue, os.Getenv("TMUX_POPUP_CONTROL_SOCKET"), os.Getenv("TMUX_POPUP_SOCKET")); socket != "" {
		return socket, nil
	}
	if tmuxEnv := os.Getenv("TMUX"); tmuxEnv != "" {
		if first, _, _ := strings.Cut(tmuxEnv, ","); first != "" {
			return first, nil
		}
	}
	baseDir := cmp.Or(os.Getenv("TMUX_TMPDIR"), "/tmp")
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

func killSessionCLI(socketPath, name, id string) error {
	args := append(baseArgs(socketPath), "kill-session", "-t", id)
	err := runExecCommand("tmux", args...).Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}
	return fmt.Errorf("failed to kill session %s [%s]: %w", name, id, err)
}

func hasSessionCLI(socketPath, id string) bool {
	args := append(baseArgs(socketPath), "has-session", "-t", id)
	return runExecCommand("tmux", args...).Run() == nil
}
