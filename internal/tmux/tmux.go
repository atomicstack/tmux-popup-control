package tmux

import (
	"fmt"
	"os"
	"os/exec"
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
	Label   string
	Current bool
}

type Pane struct {
	ID      string
	Session string
	Window  string
	Index   int
	Title   string
	Active  bool
}

type Session struct {
	Name     string
	Label    string
	Attached bool
	Clients  []string
	Current  bool
}

type SessionSnapshot struct {
	Sessions       []Session
	Current        string
	IncludeCurrent bool
}

type WindowSnapshot struct {
	Windows        []Window
	CurrentID      string
	CurrentLabel   string
	CurrentSession string
	IncludeCurrent bool
}

const defaultSessionFormat = "#S: #{session_windows} windows#{?session_attached, (attached),}"

func FetchSessions(socketPath string) (SessionSnapshot, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return SessionSnapshot{}, err
	}
	sessions, err := client.ListSessions()
	if err != nil {
		return SessionSnapshot{}, err
	}
	labelMap := fetchSessionLabels(socketPath, os.Getenv("TMUX_FZF_SESSION_FORMAT"))
	currentName := currentSessionName(client)
	includeCurrent := os.Getenv("TMUX_FZF_SWITCH_CURRENT") != ""
	out := make([]Session, 0, len(sessions))
	for _, s := range sessions {
		label := labelMap[s.Name]
		if label == "" {
			label = defaultLabelForSession(s)
		}
		entry := Session{
			Name:     s.Name,
			Label:    label,
			Attached: s.Attached > 0,
			Clients:  append([]string(nil), s.AttachedList...),
			Current:  s.Name == currentName,
		}
		out = append(out, entry)
	}
	return SessionSnapshot{Sessions: out, Current: currentName, IncludeCurrent: includeCurrent}, nil
}

func FetchWindows(socketPath string) (WindowSnapshot, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return WindowSnapshot{}, err
	}
	allWindows, err := client.ListAllWindows()
	if err != nil {
		return WindowSnapshot{}, err
	}
	lines, err := fetchWindowLines(socketPath)
	if err != nil {
		lines = fallbackWindowLines(allWindows)
	}
	windowMap := make(map[string]*gotmux.Window, len(allWindows))
	for _, w := range allWindows {
		session := firstSession(w)
		key := fmt.Sprintf("%s:%d", session, w.Index)
		windowMap[key] = w
	}
	currentSession := currentSessionName(client)
	includeCurrent := os.Getenv("TMUX_FZF_SWITCH_CURRENT") != ""
	var snapshot WindowSnapshot
	snapshot.IncludeCurrent = includeCurrent
	snapshot.CurrentSession = currentSession
	for _, line := range lines {
		w := windowMap[line.ID]
		if w == nil {
			continue
		}
		session := firstSession(w)
		entry := Window{
			ID:      line.ID,
			Session: session,
			Index:   w.Index,
			Name:    w.Name,
			Active:  w.Active,
			Label:   line.Label,
			Current: session == currentSession && w.Active,
		}
		if entry.Current {
			snapshot.CurrentID = entry.ID
			snapshot.CurrentLabel = entry.Label
		}
		snapshot.Windows = append(snapshot.Windows, entry)
	}
	if snapshot.CurrentID == "" {
		for _, w := range snapshot.Windows {
			if w.Session == currentSession && w.Active {
				snapshot.CurrentID = w.ID
				snapshot.CurrentLabel = w.Label
				break
			}
		}
	}
	return snapshot, nil
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

func RenameWindow(socketPath, target, newName string) error {
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
	return window.Rename(newName)
}

func LinkWindow(socketPath, source, targetSession string) error {
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "link-window", "-a", "-s", source, "-t", targetSession)
	if err := exec.Command("tmux", args...).Run(); err != nil {
		return fmt.Errorf("failed to link window %s to %s: %w", source, targetSession, err)
	}
	return nil
}

func MoveWindow(socketPath, source, targetSession string) error {
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "move-window", "-a", "-s", source, "-t", targetSession)
	if err := exec.Command("tmux", args...).Run(); err != nil {
		return fmt.Errorf("failed to move window %s to %s: %w", source, targetSession, err)
	}
	return nil
}

func SwapWindows(socketPath, first, second string) error {
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "swap-window", "-s", first, "-t", second)
	if err := exec.Command("tmux", args...).Run(); err != nil {
		return fmt.Errorf("failed to swap windows %s and %s: %w", first, second, err)
	}
	return nil
}

func KillWindows(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		window, err := findWindow(client, target)
		if err != nil {
			return err
		}
		if window == nil {
			return fmt.Errorf("window %s not found", target)
		}
		if err := window.Kill(); err != nil {
			return err
		}
	}
	return nil
}

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
	session, err := findSession(client, target)
	if err != nil {
		return err
	}
	if session == nil {
		return fmt.Errorf("session %s not found", target)
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
		session, err := findSession(client, target)
		if err != nil {
			return err
		}
		if session == nil {
			return fmt.Errorf("session %s not found", target)
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
		session, err := findSession(client, target)
		if err != nil {
			return err
		}
		if session == nil {
			return fmt.Errorf("session %s not found", target)
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

func findSession(client *gotmux.Tmux, target string) (*gotmux.Session, error) {
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
	return session, nil
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

type windowLine struct {
	ID    string
	Label string
}

func fetchWindowLines(socketPath string) ([]windowLine, error) {
	filter := strings.TrimSpace(os.Getenv("TMUX_FZF_WINDOW_FILTER"))
	formatExpr := strings.TrimSpace(os.Getenv("TMUX_FZF_WINDOW_FORMAT"))
	if formatExpr == "" {
		formatExpr = "#{window_name}"
	}
	labelFormat := fmt.Sprintf("#S:#{window_index}: %s", formatExpr)
	format := fmt.Sprintf("#{session_name}:#{window_index}\t%s", labelFormat)
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "list-windows")
	if filter == "" {
		args = append(args, "-a")
	} else {
		args = append(args, "-a", "-f", filter)
	}
	args = append(args, "-F", format)
	output, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return []windowLine{}, nil
	}
	lines := strings.Split(text, "\n")
	result := make([]windowLine, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		id := parts[0]
		label := id
		if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
			label = strings.TrimSpace(parts[1])
		}
		result = append(result, windowLine{ID: id, Label: label})
	}
	return result, nil
}

func fallbackWindowLines(windows []*gotmux.Window) []windowLine {
	lines := make([]windowLine, 0, len(windows))
	for _, w := range windows {
		session := firstSession(w)
		id := fmt.Sprintf("%s:%d", session, w.Index)
		label := fmt.Sprintf("%s:%d %s", session, w.Index, w.Name)
		lines = append(lines, windowLine{ID: id, Label: label})
	}
	return lines
}

func fetchSessionLabels(socketPath, envFormat string) map[string]string {
	labelExpr := strings.TrimSpace(envFormat)
	if labelExpr != "" {
		labelExpr = fmt.Sprintf("#S: %s", labelExpr)
	} else {
		labelExpr = defaultSessionFormat
	}
	format := fmt.Sprintf("#{session_name}\t%s", labelExpr)
	args := make([]string, 0, 4)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "list-sessions", "-F", format)
	cmd := exec.Command("tmux", args...)
	output, err := cmd.Output()
	if err != nil {
		return map[string]string{}
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	labels := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}
		label := name
		if len(parts) > 1 {
			trimmed := strings.TrimSpace(parts[1])
			if trimmed != "" {
				label = trimmed
			}
		}
		labels[name] = label
	}
	return labels
}

func defaultLabelForSession(s *gotmux.Session) string {
	label := fmt.Sprintf("%s: %d window", s.Name, s.Windows)
	if s.Windows != 1 {
		label += "s"
	}
	if s.Attached > 0 {
		label += " (attached)"
	}
	return label
}

func currentSessionName(client *gotmux.Tmux) string {
	if clients, err := client.ListClients(); err == nil {
		for _, c := range clients {
			if c != nil && c.Session != "" {
				return c.Session
			}
		}
	}
	return ""
}
