package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type previewKind int

const (
	previewKindNone previewKind = iota
	previewKindSession
	previewKindWindow
	previewKindPane
)

type previewData struct {
	kind         previewKind
	target       string
	label        string
	lines        []string
	err          string
	loading      bool
	seq          int
	scrollOffset int  // position within lines; clamped by renderPreviewPanel
	rawANSI      bool // true when lines contain ANSI escape sequences (pane captures)
}

type previewLoadedMsg struct {
	levelID string
	kind    previewKind
	target  string
	seq     int
	lines   []string
	err     error
	rawANSI bool // true when lines contain ANSI escape sequences
}

var (
	panePreviewFn   = tmux.PanePreview
	layoutPreviewFn = tmux.SelectLayout
)

type layoutAppliedMsg struct {
	levelID string
	seq     int
	err     error
}

func (m *Model) ensurePreviewForLevel(level *level) tea.Cmd {
	if level == nil {
		return nil
	}
	kind := previewKindForLevel(level.ID)
	if kind == previewKindNone {
		m.clearPreview(level.ID)
		return nil
	}
	if len(level.Items) == 0 {
		m.clearPreview(level.ID)
		return nil
	}
	if level.Cursor < 0 || level.Cursor >= len(level.Items) {
		level.Cursor = 0
	}
	item := level.Items[level.Cursor]
	if item.ID == "" {
		m.clearPreview(level.ID)
		return nil
	}
	if m.preview == nil {
		m.preview = make(map[string]*previewData)
	}

	// Plugin preview is a static overview — same content regardless of cursor.
	// Rebuilt each time to reflect changes from install/uninstall/etc.
	if kind == previewKindPlugin {
		lines := m.pluginPreviewLines()
		m.previewSeq++
		m.preview[level.ID] = &previewData{
			kind:    kind,
			target:  "__plugins__",
			label:   "Plugins",
			lines:   lines,
			seq:     m.previewSeq,
			rawANSI: true,
		}
		return nil
	}

	existing, ok := m.preview[level.ID]
	if ok && existing.target == item.ID && existing.loading {
		return nil // already fetching this target
	}
	m.previewSeq++
	seq := m.previewSeq
	if ok {
		// Reuse entry — old lines stay visible until the new data arrives.
		existing.kind = kind
		existing.target = item.ID
		existing.label = item.Label
		existing.loading = true
		existing.seq = seq
	} else {
		m.preview[level.ID] = &previewData{
			kind:    kind,
			target:  item.ID,
			label:   item.Label,
			loading: true,
			seq:     seq,
		}
	}
	socket := m.socketPath
	levelID := level.ID
	target := item.ID
	switch kind {
	case previewKindTree:
		return m.treePreviewCmd(levelID, target, seq, socket)
	case previewKindPane:
		return func() tea.Msg {
			lines, err := panePreviewFn(socket, target)
			return previewLoadedMsg{
				levelID: levelID,
				kind:    kind,
				target:  target,
				seq:     seq,
				lines:   lines,
				err:     err,
				rawANSI: true,
			}
		}
	case previewKindSession:
		paneID := m.activePaneIDForSession(target)
		if paneID == "" {
			lines := m.sessionPreviewLines(target)
			return func() tea.Msg {
				return previewLoadedMsg{levelID: levelID, kind: kind, target: target, seq: seq, lines: lines}
			}
		}
		return func() tea.Msg {
			lines, err := panePreviewFn(socket, paneID)
			return previewLoadedMsg{levelID: levelID, kind: kind, target: target, seq: seq, lines: lines, err: err, rawANSI: true}
		}
	case previewKindWindow:
		paneID := m.activePaneIDForWindow(target)
		if paneID == "" {
			lines := m.windowPreviewLines(target)
			return func() tea.Msg {
				return previewLoadedMsg{levelID: levelID, kind: kind, target: target, seq: seq, lines: lines}
			}
		}
		return func() tea.Msg {
			lines, err := panePreviewFn(socket, paneID)
			return previewLoadedMsg{levelID: levelID, kind: kind, target: target, seq: seq, lines: lines, err: err, rawANSI: true}
		}
	case previewKindLayout:
		// Save original layout on first visit.
		if level.Data == nil {
			for _, it := range level.Items {
				if it.Label == "current layout" {
					level.Data = it.ID
					break
				}
			}
			if level.Data == nil {
				level.Data = ""
			}
		}
		return func() tea.Msg {
			err := layoutPreviewFn(socket, target)
			return layoutAppliedMsg{levelID: levelID, seq: seq, err: err}
		}
	default:
		return nil
	}
}

func (m *Model) ensurePreviewForCurrentLevel() tea.Cmd {
	return m.ensurePreviewForLevel(m.currentLevel())
}

// refreshPreviewForLevel triggers a re-fetch for the level's preview while
// keeping existing content visible until the new data arrives.
func (m *Model) refreshPreviewForLevel(level *level) tea.Cmd {
	if level == nil {
		return nil
	}
	// Mark existing preview as stale (not loading) so ensurePreviewForLevel
	// will issue a new fetch, but do NOT delete the entry — its lines remain
	// visible until the fresh data arrives.
	if existing, ok := m.preview[level.ID]; ok {
		existing.loading = false
	}
	return m.ensurePreviewForLevel(level)
}

// activePaneIDForSession returns the pane item ID to capture for a session preview.
// It prefers the session's currently active pane; falls back to the first pane found.
func (m *Model) activePaneIDForSession(session string) string {
	target := strings.TrimSpace(session)
	if target == "" {
		return ""
	}
	var fallback string
	for _, entry := range m.panes.Entries() {
		if strings.TrimSpace(entry.Session) != target {
			continue
		}
		if entry.Current {
			return entry.ID
		}
		if fallback == "" {
			fallback = entry.ID
		}
	}
	return fallback
}

// activePaneIDForWindow returns the pane item ID to capture for a window preview.
// It prefers the window's currently active pane; falls back to the first pane found.
func (m *Model) activePaneIDForWindow(window string) string {
	target := strings.TrimSpace(window)
	if target == "" {
		return ""
	}
	var fallback string
	for _, entry := range m.panes.Entries() {
		if !windowMatchesTarget(entry, target) {
			continue
		}
		if entry.Current {
			return entry.ID
		}
		if fallback == "" {
			fallback = entry.ID
		}
	}
	return fallback
}

// treePreviewCmd returns a preview command appropriate for the tree item type.
func (m *Model) treePreviewCmd(levelID, target string, seq int, socket string) tea.Cmd {
	kind := menu.TreeItemKind(target)
	switch kind {
	case "pane":
		// tree:p:session:windowIndex:paneDisplayID
		parts := strings.SplitN(strings.TrimPrefix(target, menu.TreePrefixPane), ":", 3)
		if len(parts) < 3 {
			return nil
		}
		paneTarget := parts[2] // display ID like "test00:0.0"
		return func() tea.Msg {
			lines, err := panePreviewFn(socket, paneTarget)
			return previewLoadedMsg{levelID: levelID, kind: previewKindPane, target: target, seq: seq, lines: lines, err: err, rawANSI: true}
		}
	case "window":
		// tree:w:session:windowIndex
		parts := strings.SplitN(strings.TrimPrefix(target, menu.TreePrefixWindow), ":", 2)
		if len(parts) < 2 {
			return nil
		}
		session, windowIdx := parts[0], parts[1]
		windowTarget := session + ":" + windowIdx
		paneID := m.activePaneIDForWindow(windowTarget)
		if paneID == "" {
			lines := m.windowPreviewLines(windowTarget)
			return func() tea.Msg {
				return previewLoadedMsg{levelID: levelID, kind: previewKindWindow, target: target, seq: seq, lines: lines}
			}
		}
		return func() tea.Msg {
			lines, err := panePreviewFn(socket, paneID)
			return previewLoadedMsg{levelID: levelID, kind: previewKindWindow, target: target, seq: seq, lines: lines, err: err, rawANSI: true}
		}
	case "session":
		session := strings.TrimPrefix(target, menu.TreePrefixSession)
		paneID := m.activePaneIDForSession(session)
		if paneID == "" {
			lines := m.sessionPreviewLines(session)
			return func() tea.Msg {
				return previewLoadedMsg{levelID: levelID, kind: previewKindSession, target: target, seq: seq, lines: lines}
			}
		}
		return func() tea.Msg {
			lines, err := panePreviewFn(socket, paneID)
			return previewLoadedMsg{levelID: levelID, kind: previewKindSession, target: target, seq: seq, lines: lines, err: err, rawANSI: true}
		}
	default:
		return nil
	}
}

func (m *Model) clearPreview(levelID string) {
	if levelID == "" || m.preview == nil {
		return
	}
	delete(m.preview, levelID)
}

func (m *Model) activePreview() *previewData {
	if len(m.stack) == 0 || m.preview == nil {
		return nil
	}
	current := m.currentLevel()
	if current == nil {
		return nil
	}
	return m.preview[current.ID]
}

// previewKindTree uses item-type-specific previews for tree items.
const previewKindTree previewKind = 10

const previewKindLayout previewKind = 11
const previewKindPlugin previewKind = 12

func previewKindForLevel(id string) previewKind {
	switch id {
	case "session:switch":
		return previewKindSession
	case "window:switch":
		return previewKindWindow
	case "pane:switch", "pane:join":
		return previewKindPane
	case "session:tree":
		return previewKindTree
	case "window:layout":
		return previewKindLayout
	case "plugins":
		return previewKindPlugin
	default:
		return previewKindNone
	}
}

func (m *Model) handleLayoutAppliedMsg(msg tea.Msg) tea.Cmd {
	update, ok := msg.(layoutAppliedMsg)
	if !ok {
		return nil
	}
	if m.preview == nil {
		return nil
	}
	data, ok := m.preview[update.levelID]
	if !ok {
		return nil
	}
	if data.seq != update.seq {
		return nil
	}
	data.loading = false
	if update.err != nil {
		data.err = update.err.Error()
	}
	return nil
}

func (m *Model) handlePreviewLoadedMsg(msg tea.Msg) tea.Cmd {
	update, ok := msg.(previewLoadedMsg)
	if !ok {
		return nil
	}
	if m.preview == nil {
		return nil
	}
	data, ok := m.preview[update.levelID]
	if !ok {
		return nil
	}
	if data.seq != update.seq || data.target != update.target {
		return nil
	}
	data.loading = false
	data.rawANSI = update.rawANSI
	if update.err != nil {
		data.err = update.err.Error()
		data.lines = nil
		data.scrollOffset = 0
	} else {
		data.err = ""
		data.lines = update.lines
		// For pane captures start at the bottom so the most recent output is visible.
		// renderPreviewPanel clamps this to the actual visible range.
		if update.kind == previewKindPane {
			data.scrollOffset = len(data.lines)
		} else {
			data.scrollOffset = 0
		}
	}
	// Re-sync the viewport so the cursor stays visible with the updated item height budget.
	m.syncViewport(m.currentLevel())
	return nil
}

func (m *Model) sessionPreviewLines(session string) []string {
	target := strings.TrimSpace(session)
	if target == "" {
		return nil
	}
	windows := m.windows.Entries()
	lines := make([]string, 0, len(windows))
	for _, entry := range windows {
		if strings.TrimSpace(entry.Session) != target {
			continue
		}
		marker := " "
		if entry.Current {
			marker = "*"
		}
		label := strings.TrimSpace(entry.Name)
		if label == "" {
			label = strings.TrimSpace(entry.Label)
		}
		lines = append(lines, fmt.Sprintf("%s %d: %s", marker, entry.Index, label))
	}
	if len(lines) == 0 {
		return []string{"(no windows)"}
	}
	return lines
}

func (m *Model) windowPreviewLines(window string) []string {
	target := strings.TrimSpace(window)
	if target == "" {
		return nil
	}
	panes := m.panes.Entries()
	lines := make([]string, 0, len(panes))
	for _, entry := range panes {
		if !windowMatchesTarget(entry, target) {
			continue
		}
		marker := " "
		if entry.Current {
			marker = "*"
		}
		title := strings.TrimSpace(entry.Title)
		if title == "" {
			title = strings.TrimSpace(entry.Label)
		}
		lines = append(lines, fmt.Sprintf("%s %d: %s", marker, entry.Index, title))
	}
	if len(lines) == 0 {
		return []string{"(no panes)"}
	}
	return lines
}

func (m *Model) pluginPreviewLines() []string {
	pluginDir := plugin.PluginDir()
	installed, _ := plugin.Installed(pluginDir)
	installedSet := make(map[string]plugin.Plugin, len(installed))
	for _, p := range installed {
		installedSet[p.Name] = p
	}

	type entry struct {
		name    string
		status  string
		updated string
	}
	var entries []entry
	declaredSet := make(map[string]struct{})

	if m.socketPath != "" {
		declared, err := plugin.ParseConfig(m.socketPath)
		if err == nil {
			for _, p := range declared {
				declaredSet[p.Name] = struct{}{}
				e := entry{name: p.Name}
				if ip, ok := installedSet[p.Name]; ok {
					e.status = "installed"
					if !ip.UpdatedAt.IsZero() {
						e.updated = ip.UpdatedAt.Format(time.DateOnly)
					}
				} else {
					e.status = "not installed"
				}
				entries = append(entries, e)
			}
		}
	}

	for _, p := range installed {
		if _, declared := declaredSet[p.Name]; declared {
			continue
		}
		if p.Name == "tmux-popup-control" {
			continue
		}
		e := entry{name: p.Name, status: "undeclared"}
		if !p.UpdatedAt.IsZero() {
			e.updated = p.UpdatedAt.Format(time.DateOnly)
		}
		entries = append(entries, e)
	}

	if len(entries) == 0 {
		return []string{"(no plugins)"}
	}

	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{e.name, e.status, e.updated}
	}
	aligned := table.Format(rows, []table.Alignment{table.AlignLeft, table.AlignLeft, table.AlignRight})

	// Apply colour to the status column after alignment so ANSI codes
	// don't interfere with column width calculation.
	for i, e := range entries {
		if styled := pluginStatusStyled(e.status); styled != "" {
			// Search after the plugin name to avoid false matches.
			if idx := strings.Index(aligned[i][len(e.name):], e.status); idx >= 0 {
				pos := len(e.name) + idx
				aligned[i] = aligned[i][:pos] + styled + aligned[i][pos+len(e.status):]
			}
		}
	}
	return aligned
}

var (
	pluginStatusInstalled   = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	pluginStatusUninstalled = lipgloss.NewStyle().Foreground(lipgloss.Color("172"))
	pluginStatusUndeclared  = lipgloss.NewStyle().Foreground(lipgloss.Color("93"))
)

func pluginStatusStyled(status string) string {
	switch status {
	case "installed":
		return pluginStatusInstalled.Render(status)
	case "not installed":
		return pluginStatusUninstalled.Render(status)
	case "undeclared":
		return pluginStatusUndeclared.Render(status)
	default:
		return ""
	}
}

func windowMatchesTarget(entry menu.PaneEntry, target string) bool {
	displayID := fmt.Sprintf("%s:%d", strings.TrimSpace(entry.Session), entry.WindowIdx)
	if displayID == target {
		return true
	}
	if strings.TrimSpace(entry.Window) == target {
		return true
	}
	return false
}
