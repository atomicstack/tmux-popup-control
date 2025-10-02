package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// Window describes a tmux window identified by session:index.
type Window struct {
	ID      string
	Session string
	Index   int
	Name    string
	Active  bool
}

// Pane describes a tmux pane identified by session:index.pane.
type Pane struct {
	ID      string
	Session string
	Window  string
	Index   int
	Title   string
	Active  bool
}

func buildArgs(socketPath string, args ...string) []string {
	if socketPath == "" {
		return args
	}
	return append([]string{"-S", socketPath}, args...)
}

// FetchSessions returns the list of session names.
func FetchSessions(socketPath string) ([]string, error) {
	args := buildArgs(socketPath, "list-sessions", "-F", "#{session_name}")
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux command failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	sessions := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

// FetchWindows returns metadata about all tmux windows.
func FetchWindows(socketPath string) ([]Window, error) {
	format := "#{session_name}:#{window_index} #{window_active} #{window_name}"
	args := buildArgs(socketPath, "list-windows", "-a", "-F", format)
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux list-windows failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	windows := make([]Window, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		idPart := parts[0]
		active := parts[1] == "1"
		name := strings.TrimSpace(parts[2])
		session, indexStr, found := strings.Cut(idPart, ":")
		if !found {
			continue
		}
		idx, _ := strconv.Atoi(indexStr)
		windows = append(windows, Window{
			ID:      idPart,
			Session: session,
			Index:   idx,
			Name:    name,
			Active:  active,
		})
	}
	return windows, nil
}

// FetchPanes returns metadata about all tmux panes.
func FetchPanes(socketPath string) ([]Pane, error) {
	format := "#{session_name}:#{window_index}.#{pane_index} #{pane_active} #{pane_title}"
	args := buildArgs(socketPath, "list-panes", "-a", "-F", format)
	out, err := exec.Command("tmux", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	panes := make([]Pane, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			continue
		}
		idPart := parts[0]
		active := parts[1] == "1"
		title := strings.TrimSpace(parts[2])
		sessWin, paneIdxStr, found := strings.Cut(idPart, ".")
		if !found {
			continue
		}
		session, winIdxStr, found := strings.Cut(sessWin, ":")
		if !found {
			continue
		}
		paneIdx, _ := strconv.Atoi(paneIdxStr)
		windowIdx, _ := strconv.Atoi(winIdxStr)
		panes = append(panes, Pane{
			ID:      idPart,
			Session: session,
			Window:  fmt.Sprintf("%s:%d", session, windowIdx),
			Index:   paneIdx,
			Title:   title,
			Active:  active,
		})
	}
	return panes, nil
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
