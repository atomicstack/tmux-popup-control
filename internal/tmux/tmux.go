package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
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
	ID        string
	PaneID    string
	Session   string
	Window    string
	WindowIdx int
	Index     int
	Title     string
	Command   string
	Width     int
	Height    int
	Active    bool
	Label     string
	Current   bool
}

type PaneSnapshot struct {
	Panes          []Pane
	CurrentID      string
	CurrentLabel   string
	IncludeCurrent bool
	CurrentWindow  string
}

type Session struct {
	Name     string
	Label    string
	Attached bool
	Clients  []string
	Current  bool
	Windows  int
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
	labelMap := fetchSessionLabels(socketPath, os.Getenv("TMUX_POPUP_CONTROL_SESSION_FORMAT"))
	currentName := currentSessionName(client)
	includeCurrent := os.Getenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT") != ""
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
			Windows:  s.Windows,
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
		windowMap[w.Id] = w
	}
	currentSession := currentSessionName(client)
	includeCurrent := os.Getenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT") != ""
	var snapshot WindowSnapshot
	snapshot.IncludeCurrent = includeCurrent
	snapshot.CurrentSession = currentSession
	for _, line := range lines {
		w := windowMap[line.windowID]
		if w == nil {
			continue
		}
		session := firstSession(w)
		displayID := line.displayID
		if displayID == "" {
			displayID = fmt.Sprintf("%s:%d", session, w.Index)
		}
		entry := Window{
			ID:      displayID,
			Session: session,
			Index:   w.Index,
			Name:    w.Name,
			Active:  w.Active,
			Label:   line.label,
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

func FetchPanes(socketPath string) (PaneSnapshot, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return PaneSnapshot{}, err
	}
	allPanes, err := client.ListAllPanes()
	if err != nil {
		return PaneSnapshot{}, err
	}
	lines, err := fetchPaneLines(socketPath)
	if err != nil {
		lines = fallbackPaneLines(allPanes)
	}
	paneMap := make(map[string]*gotmux.Pane, len(allPanes))
	for _, p := range allPanes {
		paneMap[p.Id] = p
	}
	includeCurrent := os.Getenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT") != ""
	var snapshot PaneSnapshot
	snapshot.IncludeCurrent = includeCurrent
	for _, line := range lines {
		pane := paneMap[line.paneID]
		if pane == nil {
			continue
		}
		session := line.session
		entry := Pane{
			ID:        line.displayID,
			PaneID:    line.paneID,
			Session:   session,
			Window:    line.windowName,
			WindowIdx: line.windowIndex,
			Index:     line.paneIndex,
			Title:     pane.Title,
			Command:   pane.CurrentCommand,
			Width:     pane.Width,
			Height:    pane.Height,
			Active:    pane.Active,
			Label:     line.label,
			Current:   line.current,
		}
		if entry.Current {
			snapshot.CurrentID = entry.ID
			snapshot.CurrentLabel = entry.Label
			snapshot.CurrentWindow = fmt.Sprintf("%s:%d", entry.Session, entry.WindowIdx)
		}
		snapshot.Panes = append(snapshot.Panes, entry)
	}
	if snapshot.CurrentID == "" {
		for _, p := range snapshot.Panes {
			if p.Active {
				snapshot.CurrentID = p.ID
				snapshot.CurrentLabel = p.Label
				snapshot.CurrentWindow = fmt.Sprintf("%s:%d", p.Session, p.WindowIdx)
				break
			}
		}
	}
	return snapshot, nil
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

func UnlinkWindows(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	args := baseArgs(socketPath)
	for _, target := range targets {
		t := strings.TrimSpace(target)
		if t == "" {
			continue
		}
		cmd := exec.Command("tmux", append(args, "unlink-window", "-k", "-t", t)...) //nolint:gosec
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
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

func RenamePane(socketPath, target, newTitle string) error {
	trimmedTarget := strings.TrimSpace(target)
	if trimmedTarget == "" {
		return fmt.Errorf("pane target required")
	}
	trimmedTitle := strings.TrimSpace(newTitle)
	if trimmedTitle == "" {
		return fmt.Errorf("pane title required")
	}
	args := append(baseArgs(socketPath), "rename-pane", "-t", trimmedTarget, trimmedTitle)
	return exec.Command("tmux", args...).Run()
}

func KillPanes(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	args := baseArgs(socketPath)
	for _, target := range targets {
		t := strings.TrimSpace(target)
		if t == "" {
			continue
		}
		cmd := exec.Command("tmux", append(args, "kill-pane", "-t", t)...) //nolint:gosec
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func SwapPanes(socketPath, first, second string) error {
	if strings.TrimSpace(first) == "" || strings.TrimSpace(second) == "" {
		return fmt.Errorf("pane ids required")
	}
	args := append(baseArgs(socketPath), "swap-pane", "-s", first, "-t", second)
	return exec.Command("tmux", args...).Run()
}

func MovePane(socketPath, source, target string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("pane source required")
	}
	args := append(baseArgs(socketPath), "move-pane", "-s", source)
	if strings.TrimSpace(target) != "" {
		args = append(args, "-t", target)
	}
	return exec.Command("tmux", args...).Run()
}

func BreakPane(socketPath, source, destination string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("pane source required")
	}
	args := append(baseArgs(socketPath), "break-pane", "-s", source)
	if strings.TrimSpace(destination) != "" {
		args = append(args, "-t", destination)
	}
	return exec.Command("tmux", args...).Run()
}

func SelectLayout(socketPath, layout string) error {
	if strings.TrimSpace(layout) == "" {
		return fmt.Errorf("layout required")
	}
	args := append(baseArgs(socketPath), "select-layout", layout)
	return exec.Command("tmux", args...).Run()
}

func ResizePane(socketPath, direction string, amount int) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	flag := ""
	switch direction {
	case "left":
		flag = "-L"
	case "right":
		flag = "-R"
	case "up":
		flag = "-U"
	case "down":
		flag = "-D"
	default:
		return fmt.Errorf("unknown direction %q", direction)
	}
	args := append(baseArgs(socketPath), "resize-pane", flag, strconv.Itoa(amount))
	return exec.Command("tmux", args...).Run()
}

func SwitchPane(socketPath, target string) error {
	parts := strings.SplitN(target, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid pane target %q", target)
	}
	session := parts[0]
	windowPart := parts[1]
	windowParts := strings.SplitN(windowPart, ".", 2)
	if len(windowParts) != 2 {
		return fmt.Errorf("invalid pane target %q", target)
	}
	window := fmt.Sprintf("%s:%s", session, windowParts[0])
	if err := SwitchClient(socketPath, session); err != nil {
		return err
	}
	if err := SelectWindow(socketPath, window); err != nil {
		return err
	}
	args := append(baseArgs(socketPath), "select-pane", "-t", target)
	return exec.Command("tmux", args...).Run()
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
	windowID  string
	displayID string
	label     string
}

type paneLine struct {
	paneID      string
	displayID   string
	label       string
	session     string
	windowName  string
	windowIndex int
	paneIndex   int
	current     bool
}

func fetchWindowLines(socketPath string) ([]windowLine, error) {
	filter := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_WINDOW_FILTER"))
	formatExpr := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_WINDOW_FORMAT"))
	if formatExpr == "" {
		formatExpr = "#{window_name}"
	}
	labelFormat := fmt.Sprintf("#S:#{window_index}: %s", formatExpr)
	format := fmt.Sprintf("#{window_id}\t#{session_name}:#{window_index}\t%s", labelFormat)
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
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		wid := strings.TrimSpace(parts[0])
		display := strings.TrimSpace(parts[1])
		label := display
		if len(parts) > 2 {
			trimmed := strings.TrimSpace(parts[2])
			if trimmed != "" {
				label = trimmed
			}
		}
		result = append(result, windowLine{windowID: wid, displayID: display, label: label})
	}
	return result, nil
}

func fallbackWindowLines(windows []*gotmux.Window) []windowLine {
	lines := make([]windowLine, 0, len(windows))
	for _, w := range windows {
		session := firstSession(w)
		id := fmt.Sprintf("%s:%d", session, w.Index)
		label := fmt.Sprintf("%s:%d %s", session, w.Index, w.Name)
		lines = append(lines, windowLine{windowID: w.Id, displayID: id, label: label})
	}
	return lines
}

func fetchPaneLines(socketPath string) ([]paneLine, error) {
	filter := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_FILTER"))
	formatExpr := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_FORMAT"))
	if formatExpr == "" {
		formatExpr = "[#{window_name}:#{pane_title}] #{pane_current_command}  [#{pane_width}x#{pane_height}] [history #{history_size}/#{history_limit}, #{history_bytes} bytes] #{?pane_active,[active],[inactive]}"
	}
	labelFormat := fmt.Sprintf("#S:#{window_index}.#{pane_index}: %s", formatExpr)
	format := fmt.Sprintf("#{pane_id}\t#S:#{window_index}.#{pane_index}\t%s\t#{session_name}\t#{window_name}\t#{window_index}\t#{pane_index}\t#{?pane_active&&window_active&&session_attached,1,0}", labelFormat)
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "list-panes", "-a")
	if filter != "" {
		args = append(args, "-f", filter)
	}
	args = append(args, "-F", format)
	output, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return []paneLine{}, nil
	}
	lines := strings.Split(text, "\n")
	result := make([]paneLine, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 8)
		if len(parts) < 8 {
			continue
		}
		paneID := strings.TrimSpace(parts[0])
		displayID := strings.TrimSpace(parts[1])
		label := strings.TrimSpace(parts[2])
		session := strings.TrimSpace(parts[3])
		windowName := strings.TrimSpace(parts[4])
		windowIndex, _ := strconv.Atoi(strings.TrimSpace(parts[5]))
		paneIndex, _ := strconv.Atoi(strings.TrimSpace(parts[6]))
		current := strings.TrimSpace(parts[7]) == "1"
		if label == "" {
			label = displayID
		}
		result = append(result, paneLine{
			paneID:      paneID,
			displayID:   displayID,
			label:       label,
			session:     session,
			windowName:  windowName,
			windowIndex: windowIndex,
			paneIndex:   paneIndex,
			current:     current,
		})
	}
	return result, nil
}

func fallbackPaneLines(panes []*gotmux.Pane) []paneLine {
	lines := make([]paneLine, 0, len(panes))
	for _, p := range panes {
		id := p.Id
		display := p.Id
		label := display
		lines = append(lines, paneLine{
			paneID:    id,
			displayID: display,
			label:     label,
			paneIndex: p.Index,
		})
	}
	return lines
}

func baseArgs(socketPath string) []string {
	if strings.TrimSpace(socketPath) == "" {
		return []string{}
	}
	return []string{"-S", socketPath}
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
