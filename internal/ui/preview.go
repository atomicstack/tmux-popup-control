package ui

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	tea "github.com/charmbracelet/bubbletea"
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
	panePreviewFn = tmux.PanePreview
)

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
	if existing, ok := m.preview[level.ID]; ok && existing.target == item.ID && !existing.loading {
		return nil
	}
	m.previewSeq++
	seq := m.previewSeq
	m.preview[level.ID] = &previewData{
		kind:    kind,
		target:  item.ID,
		label:   item.Label,
		loading: true,
		seq:     seq,
	}
	socket := m.socketPath
	levelID := level.ID
	target := item.ID
	switch kind {
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
	default:
		return nil
	}
}

func (m *Model) ensurePreviewForCurrentLevel() tea.Cmd {
	return m.ensurePreviewForLevel(m.currentLevel())
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

func previewKindForLevel(id string) previewKind {
	switch id {
	case "session:switch":
		return previewKindSession
	case "window:switch":
		return previewKindWindow
	case "pane:switch", "pane:join":
		return previewKindPane
	default:
		return previewKindNone
	}
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
