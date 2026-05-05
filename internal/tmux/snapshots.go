package tmux

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// envOrOption returns the value of an env var, falling back to a tmux server
// option if the env var is empty. This lets users configure settings in
// tmux.conf via `set -g @option-name "value"` as an alternative to env vars.
func envOrOption(socketPath, envKey, optionName string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return ShowOption(socketPath, optionName)
}

func FetchSessions(socketPath string) (SessionSnapshot, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return SessionSnapshot{}, err
	}

	sessions, err := client.ListSessions()
	if err != nil {
		return SessionSnapshot{}, err
	}
	if len(sessions) == 0 {
		fallback, err := fetchSessionsFallback(socketPath)
		if err == nil {
			sessions = fallback
		}
	}
	// Only run the per-session label format command when the user has
	// configured a custom format — otherwise defaultLabelForSession produces
	// the same output as the built-in defaultSessionFormat without paying
	// for an extra control-mode round-trip on every poll cycle.
	customFormat := envOrOption(socketPath, "TMUX_POPUP_CONTROL_SESSION_FORMAT", "@tmux-popup-control-session-format")
	var labelMap map[string]string
	if strings.TrimSpace(customFormat) != "" {
		labelMap = fetchSessionLabels(client, customFormat)
	}
	currentName := currentSessionName(client)
	realClients := realAttachedClients(client)
	includeCurrent := envOrOption(socketPath, "TMUX_POPUP_CONTROL_SWITCH_CURRENT", "@tmux-popup-control-switch-current") != ""
	out := make([]Session, 0, len(sessions))
	for _, s := range sessions {
		label := labelMap[s.Name]
		if label == "" {
			label = defaultLabelForSession(s)
		}
		clients := realClients[s.Name]
		entry := Session{
			Name:     s.Name,
			Label:    label,
			Path:     s.Path,
			Attached: len(clients) > 0,
			Clients:  clients,
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
	lines, err := fetchWindowLines(socketPath, client)
	if err != nil {
		lines = fallbackWindowLines(allWindows)
	}
	windowMap := make(map[string]*gotmux.Window, len(allWindows))
	for _, w := range allWindows {
		windowMap[w.Id] = w
	}
	currentSession := currentSessionName(client)
	includeCurrent := envOrOption(socketPath, "TMUX_POPUP_CONTROL_SWITCH_CURRENT", "@tmux-popup-control-switch-current") != ""
	var snapshot WindowSnapshot
	snapshot.IncludeCurrent = includeCurrent
	snapshot.CurrentSession = currentSession
	for _, line := range lines {
		w := windowMap[line.windowID]
		if w == nil {
			session := ""
			var idx int
			if parts := strings.SplitN(line.displayID, ":", 2); len(parts) > 0 {
				session = strings.TrimSpace(parts[0])
				if len(parts) > 1 {
					if parsed, err := strconv.Atoi(parts[1]); err == nil {
						idx = parsed
					}
				}
			}
			entry := Window{
				ID:         line.displayID,
				Session:    session,
				Index:      idx,
				Label:      line.label,
				InternalID: line.windowID,
			}
			if session == currentSession {
				entry.Current = true
			}
			snapshot.Windows = append(snapshot.Windows, entry)
			continue
		}
		session := firstSession(w)
		if session == "" {
			session = strings.TrimSpace(w.Session)
		}
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
			Layout:     w.Layout,
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
	lines, err := fetchPaneLines(socketPath, client)
	if err != nil {
		lines = fallbackPaneLines(allPanes)
	}
	paneMap := make(map[string]*gotmux.Pane, len(allPanes))
	for _, p := range allPanes {
		paneMap[p.Id] = p
	}
	includeCurrent := envOrOption(socketPath, "TMUX_POPUP_CONTROL_SWITCH_CURRENT", "@tmux-popup-control-switch-current") != ""
	hostSession := currentSessionName(client)
	var snapshot PaneSnapshot
	snapshot.IncludeCurrent = includeCurrent
	for _, line := range lines {
		pane := paneMap[line.paneID]
		if pane == nil {
			continue
		}
		session := line.session
		// The tmux format "session_attached" is true for all attached
		// sessions, so line.current may be set for panes outside the
		// popup's host session. Narrow it to the host session only.
		current := line.current && (hostSession == "" || session == hostSession)
		entry := Pane{
			ID:        line.displayID,
			PaneID:    line.paneID,
			Session:   session,
			Window:    line.windowName,
			WindowIdx: line.windowIndex,
			Index:     line.paneIndex,
			Title:     pane.Title,
			Command:   pane.CurrentCommand,
			Path:      pane.CurrentPath,
			Width:     pane.Width,
			Height:    pane.Height,
			Active:    pane.Active,
			Label:     line.label,
			Current:   current,
		}
		if entry.Current {
			snapshot.CurrentID = entry.ID
			snapshot.CurrentLabel = entry.Label
			snapshot.CurrentWindow = fmt.Sprintf("%s:%d", entry.Session, entry.WindowIdx)
		}
		snapshot.Panes = append(snapshot.Panes, entry)
	}
	// Prefer the pane ID captured by main.sh before the popup opened.
	// This is the most reliable source because it was resolved in the
	// user's actual terminal context, not the control-mode client's.
	if hpid := hostPaneID(); hpid != "" {
		for i, p := range snapshot.Panes {
			if p.PaneID == hpid {
				snapshot.Panes[i].Current = true
				snapshot.CurrentID = p.ID
				snapshot.CurrentLabel = p.Label
				snapshot.CurrentWindow = fmt.Sprintf("%s:%d", p.Session, p.WindowIdx)
				break
			}
		}
	}
	if snapshot.CurrentID == "" {
		// Fallback: find the active pane in the host session.
		for _, p := range snapshot.Panes {
			if p.Active && (hostSession == "" || p.Session == hostSession) {
				snapshot.CurrentID = p.ID
				snapshot.CurrentLabel = p.Label
				snapshot.CurrentWindow = fmt.Sprintf("%s:%d", p.Session, p.WindowIdx)
				break
			}
		}
	}
	return snapshot, nil
}

// fetchSessionsFallback is an exec-based fallback used only when the
// control-mode ListSessions call returns no sessions (e.g. during a
// race at startup). It is intentionally kept as a direct tmux invocation
// so it still works if the control-mode transport is misbehaving.
func fetchSessionsFallback(socketPath string) ([]*gotmux.Session, error) {
	format := "#{session_name}\t#{session_windows}\t#{session_attached}"
	args := make([]string, 0, 6)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "list-sessions", "-F", format)
	output, err := runExecCommand("tmux", args...).Output()
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return []*gotmux.Session{}, nil
	}
	lines := strings.Split(text, "\n")
	sessions := make([]*gotmux.Session, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		windows, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
		attached, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
		sessions = append(sessions, &gotmux.Session{
			Name:     strings.TrimSpace(parts[0]),
			Windows:  windows,
			Attached: attached,
		})
	}
	return sessions, nil
}

func fetchSessionLabels(client tmuxClient, envFormat string) map[string]string {
	labelExpr := strings.TrimSpace(envFormat)
	if labelExpr != "" {
		labelExpr = fmt.Sprintf("#S: %s", labelExpr)
	} else {
		labelExpr = defaultSessionFormat
	}
	format := fmt.Sprintf("#{session_name}\t%s", labelExpr)
	lines, err := client.ListSessionsFormat(format)
	if err != nil {
		return map[string]string{}
	}
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

// realAttachedClients returns a map from session name to the names of
// non-control-mode clients attached to it. This excludes gotmuxcc's own
// control-mode connection, which would otherwise inflate session_attached counts.
func realAttachedClients(client tmuxClient) map[string][]string {
	clients, err := client.ListClients()
	if err != nil {
		return nil
	}
	result := make(map[string][]string)
	for _, c := range clients {
		if c == nil || c.ControlMode || c.Session == "" {
			continue
		}
		result[c.Session] = append(result[c.Session], c.Name)
	}
	return result
}

// hostPaneID returns the pane ID (e.g. "%4") of the pane that was active
// when the popup was opened. main.sh captures #{pane_id} before opening
// display-popup and passes it as TMUX_POPUP_CONTROL_PANE_ID.
func hostPaneID() string {
	return strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_ID"))
}

// hostSessionID returns the session ID of the popup's host session in
// tmux's "$N" format (e.g. "$1"). It checks TMUX_POPUP_CONTROL_SESSION_ID
// (set by main.sh) first, then falls back to parsing the TMUX env var.
func hostSessionID() string {
	if id := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_SESSION_ID")); id != "" {
		if !strings.HasPrefix(id, "$") {
			id = "$" + id
		}
		return id
	}
	parts := strings.Split(os.Getenv("TMUX"), ",")
	if len(parts) >= 3 {
		if id := strings.TrimSpace(parts[2]); id != "" {
			return "$" + id
		}
	}
	return ""
}

func currentSessionName(client tmuxClient) string {
	// Prefer the session ID — stable across renames.
	if id := hostSessionID(); id != "" {
		if sessions, err := client.ListSessions(); err == nil {
			for _, s := range sessions {
				if s.Id == id {
					return s.Name
				}
			}
		}
	}
	if name := popupSessionName(client); name != "" {
		return name
	}
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

func fetchWindowLines(socketPath string, client tmuxClient) ([]windowLine, error) {
	filter := strings.TrimSpace(envOrOption(socketPath, "TMUX_POPUP_CONTROL_WINDOW_FILTER", "@tmux-popup-control-window-filter"))
	formatExpr := strings.TrimSpace(envOrOption(socketPath, "TMUX_POPUP_CONTROL_WINDOW_FORMAT", "@tmux-popup-control-window-format"))
	if formatExpr == "" {
		formatExpr = "#{window_name}"
	}
	labelFormat := fmt.Sprintf("#S:#{window_index}: %s", formatExpr)
	format := fmt.Sprintf("#{window_id}\t#{session_name}:#{window_index}\t%s", labelFormat)
	rawLines, err := client.ListWindowsFormat("", filter, format)
	if err != nil {
		return nil, err
	}
	result := make([]windowLine, 0, len(rawLines))
	for _, line := range rawLines {
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

func fetchPaneLines(socketPath string, client tmuxClient) ([]paneLine, error) {
	filter := strings.TrimSpace(envOrOption(socketPath, "TMUX_POPUP_CONTROL_PANE_FILTER", "@tmux-popup-control-pane-filter"))
	formatExpr := strings.TrimSpace(envOrOption(socketPath, "TMUX_POPUP_CONTROL_PANE_FORMAT", "@tmux-popup-control-pane-format"))
	if formatExpr == "" {
		formatExpr = "[#{window_name}:#{pane_title}] #{pane_current_command}  [#{pane_width}x#{pane_height}] [history #{history_size}/#{history_limit}, #{history_bytes} bytes] #{?pane_active,[active],[inactive]}"
	}
	labelFormat := fmt.Sprintf("#S:#{window_index}.#{pane_index}: %s", formatExpr)
	format := fmt.Sprintf("#{pane_id}\t#S:#{window_index}.#{pane_index}\t%s\t#{session_name}\t#{window_name}\t#{window_index}\t#{pane_index}\t#{?pane_active&&window_active&&session_attached,1,0}", labelFormat)
	rawLines, err := client.ListPanesFormat("", filter, format)
	if err != nil {
		return nil, err
	}
	result := make([]paneLine, 0, len(rawLines))
	for _, line := range rawLines {
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
