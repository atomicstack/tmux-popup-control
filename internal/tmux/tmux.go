package tmux

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	gotmux "github.com/GianlucaP106/gotmux/gotmux"
)

type Window struct {
	ID      string
	Session string
	Index   int
	Name    string
	Active  bool
}

type Pane struct {
	ID      string
	Session string
	Window  string
	Index   int
	Title   string
	Active  bool
}

func FetchSessions(socketPath string) ([]string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	sessions, err := client.ListSessions()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, s.Name)
	}
	return out, nil
}

func FetchWindows(socketPath string) ([]Window, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	windows, err := client.ListAllWindows()
	if err != nil {
		return nil, err
	}
	out := make([]Window, 0, len(windows))
	for _, w := range windows {
		session := firstSession(w)
		id := w.Id
		if session != "" {
			id = fmt.Sprintf("%s:%d", session, w.Index)
		}
		out = append(out, Window{
			ID:      id,
			Session: session,
			Index:   w.Index,
			Name:    w.Name,
			Active:  w.Active,
		})
	}
	return out, nil
}

func FetchPanes(socketPath string) ([]Pane, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	panes, err := client.ListAllPanes()
	if err != nil {
		return nil, err
	}
	out := make([]Pane, 0, len(panes))
	for _, p := range panes {
		out = append(out, Pane{
			ID:     p.Id,
			Index:  p.Index,
			Title:  p.Title,
			Active: p.Active,
		})
	}
	return out, nil
}

func SwitchClient(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	return client.SwitchClient(&gotmux.SwitchClientOptions{TargetSession: target})
}

func SelectWindow(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	window, err := findWindow(client, target)
	if err != nil {
		return err
	}
	if window == nil {
		return fmt.Errorf("window %s not found", target)
	}
	return window.Select()
}

func KillWindow(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	window, err := findWindow(client, target)
	if err != nil {
		return err
	}
	if window == nil {
		return fmt.Errorf("window %s not found", target)
	}
	return window.Kill()
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

func newTmux(socketPath string) (*gotmux.Tmux, error) {
	if socketPath != "" {
		return gotmux.NewTmux(socketPath)
	}
	return gotmux.DefaultTmux()
}

func findWindow(client *gotmux.Tmux, target string) (*gotmux.Window, error) {
	windows, err := client.ListAllWindows()
	if err != nil {
		return nil, err
	}
	for _, w := range windows {
		session := firstSession(w)
		candidates := []string{w.Id}
		if session != "" {
			candidates = append(candidates, fmt.Sprintf("%s:%d", session, w.Index))
		}
		for _, c := range candidates {
			if c == target {
				return w, nil
			}
		}
	}
	return nil, nil
}

func firstSession(w *gotmux.Window) string {
	if len(w.ActiveSessionsList) > 0 {
		return w.ActiveSessionsList[0]
	}
	if len(w.LinkedSessionsList) > 0 {
		return w.LinkedSessionsList[0]
	}
	return ""
}
