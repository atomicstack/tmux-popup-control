package tmux

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

// FetchSessions runs `tmux -C list-sessions` and returns the session names.
func FetchSessions(socketPath string) ([]string, error) {
	args := []string{"-C", "list-sessions"}
	if socketPath != "" {
		args = append([]string{"-S", socketPath}, args...)
	}
	cmd := exec.Command("tmux", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux command failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	var sessions []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "session-added"),
			strings.HasPrefix(line, "session-renamed"),
			strings.HasPrefix(line, "session-changed"),
			strings.HasPrefix(line, "session-closed"),
			strings.HasPrefix(line, "%begin"),
			strings.HasPrefix(line, "%end"),
			strings.HasPrefix(line, "list-sessions"):
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], "%") {
			sessions = append(sessions, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

// ResolveSocketPath determines the tmux socket to talk to based on CLI flag,
// env vars, or a reasonable default.
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
