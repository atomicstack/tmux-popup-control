package gotmuxcc

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var tmuxListClients = func(path string) ([]byte, error) {
	cmd := exec.Command("tmux", "-S", path, "list-clients")
	cmd.Env = append(os.Environ(), "TMUX=")
	return cmd.CombinedOutput()
}

func newSocket(path string) (*Socket, error) {
	if path == "" {
		return nil, nil
	}
	if err := validateSocket(path); err != nil {
		return nil, err
	}
	return &Socket{Path: path}, nil
}

func validateSocket(path string) error {
	if out, err := tmuxListClients(path); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("tmux binary not found while validating socket %q", path)
		}
		msg := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(msg), "no such file or directory") {
			return fmt.Errorf("tmux socket %q not available: %s", path, msg)
		}
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("invalid tmux socket %q: %s", path, msg)
	}
	return nil
}
