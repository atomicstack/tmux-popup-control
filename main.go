package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const logFile = "tmux-popup-control.log"

var socketFlag = flag.String("socket", "", "path to the tmux socket (overrides environment detection)")

type model struct {
	sessions   []string
	selected   int
	loading    bool
	socketPath string
	errMsg     string
}

type sessionsLoadedMsg struct {
	sessions []string
	err      error
}

// Log errors to file
func logError(err error) {
	f, ferr := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "logging failed: %v\n", ferr)
		return
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println(err)
}

func fetchSessions(socketPath string) ([]string, error) {
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
		if strings.HasPrefix(line, "session-added") || strings.HasPrefix(line, "session-renamed") ||
			strings.HasPrefix(line, "session-changed") || strings.HasPrefix(line, "session-closed") {
			continue // skip event lines
		}
		if strings.HasPrefix(line, "%begin") || strings.HasPrefix(line, "%end") {
			continue // control markers
		}
		if strings.HasPrefix(line, "list-sessions") {
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

func resolveSocketPath(flagValue string) (string, error) {
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

func fetchSessionsCmd(socketPath string) tea.Cmd {
	return func() tea.Msg {
		sessions, err := fetchSessions(socketPath)
		if err != nil {
			logError(err)
		}
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m *model) Init() tea.Cmd {
	return fetchSessionsCmd(m.socketPath)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.sessions = nil
			m.selected = 0
			return m, nil
		}
		m.errMsg = ""
		m.sessions = msg.sessions
		if len(m.sessions) == 0 {
			m.selected = 0
		} else if m.selected >= len(m.sessions) {
			m.selected = len(m.sessions) - 1
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.errMsg = ""
			return m, fetchSessionsCmd(m.socketPath)
		case "up":
			if m.selected > 0 {
				m.selected--
			}
		case "down":
			if m.selected < len(m.sessions)-1 {
				m.selected++
			}
		}
	}
	return m, nil
}

func (m *model) View() string {
	if m.loading {
		return "Loading tmux sessions..."
	}
	if m.errMsg != "" {
		return fmt.Sprintf("Error loading tmux sessions:\n%s\n\nPress r to retry, q to quit.\n", m.errMsg)
	}
	if len(m.sessions) == 0 {
		return "No tmux sessions found. Press r to refresh, q to quit."
	}
	s := "Tmux Sessions:\n\n"
	for i, sess := range m.sessions {
		cursor := " "
		if i == m.selected {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, sess)
	}
	s += "\nUse ↑/↓ to navigate, press r to refresh, q or ctrl+c to quit.\n"
	return s
}

func main() {
	flag.Parse()
	socketPath, err := resolveSocketPath(*socketFlag)
	if err != nil {
		logError(err)
		fmt.Fprintf(os.Stderr, "Error resolving tmux socket: %v\n", err)
		os.Exit(1)
	}
	m := &model{loading: true, socketPath: socketPath}
	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		logError(err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
