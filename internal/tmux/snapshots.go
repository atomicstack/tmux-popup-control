package tmux

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	gotmux "github.com/GianlucaP106/gotmux/gotmux"
)

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
			ID:         displayID,
			Session:    session,
			Index:      w.Index,
			Name:       w.Name,
			Active:     w.Active,
			Label:      line.label,
			Current:    session == currentSession && w.Active,
			InternalID: line.windowID,
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
	cmd := runExecCommand("tmux", args...)
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

func currentSessionName(client tmuxClient) string {
	if clients, err := client.ListClients(); err == nil {
		for _, c := range clients {
			if c != nil && c.Session != "" {
				return c.Session
			}
		}
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
	output, err := runExecCommand("tmux", args...).Output()
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
	output, err := runExecCommand("tmux", args...).Output()
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
