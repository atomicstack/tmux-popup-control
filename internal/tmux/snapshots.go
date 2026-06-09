package tmux

import (
	"fmt"
	"os"
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
					idx = atoiOr0(parts[1])
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
		parts := splitTabLine(line, 3)
		if len(parts) < 3 {
			continue
		}
		sessions = append(sessions, &gotmux.Session{
			Name:     parts[0],
			Windows:  atoiOr0(parts[1]),
			Attached: atoiOr0(parts[2]),
		})
	}
	return sessions, nil
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
	listFn := func(filter, format string) ([]string, error) {
		return client.ListWindowsFormat("", filter, format)
	}
	rows, err := fetchFormattedLines(listFn, filter, format, 3, 2)
	if err != nil {
		return nil, err
	}
	result := make([]windowLine, 0, len(rows))
	for _, parts := range rows {
		display := parts[1]
		label := display
		if len(parts) > 2 && parts[2] != "" {
			label = parts[2]
		}
		result = append(result, windowLine{windowID: parts[0], displayID: display, label: label})
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
	listFn := func(filter, format string) ([]string, error) {
		return client.ListPanesFormat("", filter, format)
	}
	rows, err := fetchFormattedLines(listFn, filter, format, 8, 8)
	if err != nil {
		return nil, err
	}
	result := make([]paneLine, 0, len(rows))
	for _, parts := range rows {
		displayID := parts[1]
		label := parts[2]
		if label == "" {
			label = displayID
		}
		result = append(result, paneLine{
			paneID:      parts[0],
			displayID:   displayID,
			label:       label,
			session:     parts[3],
			windowName:  parts[4],
			windowIndex: atoiOr0(parts[5]),
			paneIndex:   atoiOr0(parts[6]),
			current:     parts[7] == "1",
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
