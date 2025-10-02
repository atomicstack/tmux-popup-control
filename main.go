package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbletea"
)

const logFile = "tmux-popup-control.log"

type model struct {
	sessions []string
	selected int
	loading  bool
	mu       sync.Mutex
}

type sessionListMsg []string

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

func fetchSessions() ([]string, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}
	socketPath := filepath.Join("/tmp", fmt.Sprintf("tmux-%s", u.Uid), "default")
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Enter control mode
	_, err = fmt.Fprintf(conn, "attach-client -t=default -C\n")
	if err != nil {
		return nil, err
	}

	_, err = fmt.Fprintf(conn, "list-sessions\n")
	if err != nil {
		return nil, err
	}

	var sessions []string
	scanner := bufio.NewScanner(conn)
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

func (m model) Init() bubbletea.Cmd {
	return func() bubbletea.Msg {
		sessions, err := fetchSessions()
		if err != nil {
			logError(err)
			return sessionListMsg([]string{"<error: see log>"})
		}
		return sessionListMsg(sessions)
	}
}

func (m model) Update(msg bubbletea.Msg) (bubbletea.Model, bubbletea.Cmd) {
	switch msg := msg.(type) {
	case sessionListMsg:
		m.mu.Lock()
		m.sessions = msg
		m.loading = false
		m.mu.Unlock()
	case bubbletea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, bubbletea.Quit
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

func (m model) View() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loading {
		return "Loading tmux sessions..."
	}
	if len(m.sessions) == 0 {
		return "No tmux sessions found."
	}
	s := "Tmux Sessions:\n\n"
	for i, sess := range m.sessions {
		cursor := " "
		if i == m.selected {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, sess)
	}
	s += "\nPress q or ctrl+c to quit.\n"
	return s
}

func main() {
	m := model{loading: true}
	p := bubbletea.NewProgram(m)
	if err := p.Start(); err != nil {
		logError(err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
