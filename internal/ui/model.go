package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

type level struct {
	id     string
	title  string
	items  []menu.Item
	full   []menu.Item
	filter string
	cursor int
}

type styledLine struct {
	text  string
	style *lipgloss.Style
}

type sessionFormState struct {
	input    textinput.Model
	existing map[string]struct{}
	err      string
	ctx      menu.Context
}

var (
	loadingStyle      = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Italic(true))
	itemStyle         = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("249")))
	selectedItemStyle = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Underline(true))
	errorStyle        = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true))
	infoStyle         = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("249")))
	footerStyle       = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("249")))
	filterStyle       = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("249")))
	cursorStyle       = stylePtr(lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Blink(true))
)

func newLevel(id, title string, items []menu.Item) *level {
	l := &level{id: id, title: title}
	l.updateItems(items)
	return l
}

// Model implements the Bubble Tea model for the tmux popup menu.
type Model struct {
	stack          []*level
	loading        bool
	pendingID      string
	pendingLabel   string
	errMsg         string
	infoMsg        string
	infoExpire     time.Time
	width          int
	height         int
	fixedWidth     bool
	fixedHeight    bool
	ctx            menu.Context
	backend        *backend.Watcher
	backendState   map[backend.Kind]error
	windows        []tmux.Window
	panes          []tmux.Pane
	backendLastErr string
	showFooter     bool
	sessionForm    *sessionFormState

	categoryLoaders map[string]menu.Loader
	actionLoaders   map[string]menu.Loader
	actionHandlers  map[string]menu.Action
}

// NewModel initialises the UI state with the root menu and configuration.
func NewModel(socketPath string, width, height int, showFooter bool, watcher *backend.Watcher) *Model {
	root := newLevel("root", "Main Menu", menu.RootItems())
	m := &Model{
		stack:           []*level{root},
		ctx:             menu.Context{SocketPath: socketPath},
		categoryLoaders: menu.CategoryLoaders(),
		actionLoaders:   menu.ActionLoaders(),
		actionHandlers:  menu.ActionHandlers(),
		backend:         watcher,
		backendState:    map[backend.Kind]error{},
		showFooter:      showFooter,
	}
	if width > 0 {
		m.width = width
		m.fixedWidth = true
	}
	if height > 0 {
		m.height = height
		m.fixedHeight = true
	}
	return m
}

// Init is part of the tea.Model interface.
func (m *Model) Init() tea.Cmd {
	if m.backend != nil {
		return waitForBackendEvent(m.backend)
	}
	return nil
}

// Update responds to Bubble Tea messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if handled, cmd := m.handleSessionForm(msg); handled {
        return m, cmd
    }
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.handleTextInput(msg) {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", "backspace", "left":
			if len(m.stack) > 1 {
				m.stack = m.stack[:len(m.stack)-1]
				m.errMsg = ""
				m.forceClearInfo()
			}
		case "enter":
			if m.loading {
				return m, nil
			}
			current := m.currentLevel()
			if current == nil || len(current.items) == 0 {
				return m, nil
			}
			item := current.items[current.cursor]
			logging.Trace("menu.enter", map[string]interface{}{
				"level":  current.id,
				"item":   item.ID,
				"label":  item.Label,
				"filter": current.filter,
			})
			current.setFilter("")
			if handler := m.actionHandlers[current.id]; handler != nil {
				m.loading = true
				m.pendingID = current.id
				m.pendingLabel = item.Label
				m.errMsg = ""
				m.forceClearInfo()
				return m, handler(m.ctx, item)
			}
			if current.id == "root" {
				loader, ok := m.categoryLoaders[item.ID]
				if !ok {
					m.setInfo(fmt.Sprintf("No loader defined for %s", item.Label))
					return m, nil
				}
				m.loading = true
				m.pendingID = item.ID
				m.pendingLabel = item.Label
				m.errMsg = ""
				m.forceClearInfo()
				return m, m.loadMenuCmd(item.ID, item.Label, loader)
			}
			key := fmt.Sprintf("%s:%s", current.id, item.ID)
			if loader, ok := m.actionLoaders[key]; ok {
				m.loading = true
				m.pendingID = key
				m.pendingLabel = item.Label
				m.errMsg = ""
				m.forceClearInfo()
				return m, m.loadMenuCmd(key, item.Label, loader)
			}
			m.setInfo(fmt.Sprintf("Selected %s (no action defined yet)", item.Label))
		case "up":
			if current := m.currentLevel(); current != nil {
				if n := len(current.items); n > 0 {
					if current.cursor > 0 {
						current.cursor--
					} else {
						current.cursor = n - 1
					}
					logging.Trace("menu.cursor", map[string]interface{}{"level": current.id, "cursor": current.cursor})
				}
			}
		case "down":
			if current := m.currentLevel(); current != nil {
				if n := len(current.items); n > 0 {
					if current.cursor < n-1 {
						current.cursor++
					} else {
						current.cursor = 0
					}
					logging.Trace("menu.cursor", map[string]interface{}{"level": current.id, "cursor": current.cursor})
				}
			}
		}
	case tea.WindowSizeMsg:
		if !m.fixedWidth {
			m.width = msg.Width
		}
		if !m.fixedHeight {
			m.height = msg.Height
		}
	case categoryLoadedMsg:
		if msg.id != m.pendingID {
			return m, nil
		}
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		level := newLevel(msg.id, msg.title, msg.items)
		m.stack = append(m.stack, level)
		if len(level.items) == 0 {
			m.setInfo("No entries found.")
		} else if m.infoMsg != "" {
			m.clearInfo()
		}
	case menu.ActionResult:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
			m.forceClearInfo()
			logging.Trace("action.error", map[string]interface{}{"error": msg.Err.Error()})
			return m, nil
		}
		if msg.Info != "" {
			m.setInfo(msg.Info)
		} else {
			m.forceClearInfo()
		}
		logging.Trace("action.success", map[string]interface{}{"info": msg.Info})
		return m, tea.Quit
	case menu.SessionPrompt:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		m.forceClearInfo()
		m.startSessionForm(msg)
		return m, nil
	case backendEventMsg:
		m.applyBackendEvent(msg.event)
		if m.backend != nil {
			return m, waitForBackendEvent(m.backend)
		}
		return m, nil
	case backendDoneMsg:
		m.backend = nil
		return m, nil
	}
	return m, nil
}

// View renders the current menu state.
func (m *Model) View() string {
	if m.sessionForm != nil {
		return m.viewSessionForm()
	}
	lines := make([]styledLine, 0, 16)
	if m.loading {
		label := m.pendingLabel
		if label == "" {
			label = m.pendingID
		}
		if label == "" {
			label = "items"
		}
	} else if current := m.currentLevel(); current != nil {
		if len(current.items) == 0 {
			msg := "(no entries)"
			if current.filter != "" {
				msg = fmt.Sprintf("No matches for %q", current.filter)
			}
			lines = append(lines, styledLine{text: msg, style: infoStyle})
		} else {
			for i, item := range current.items {
				prefix := "  "
				lineText := item.Label
				lineStyle := itemStyle
				if i == current.cursor {
					prefix = "▌ "
					lineText = selectedItemStyle.Render(item.Label)
					lineStyle = nil
				}
				fullText := fmt.Sprintf("%s%s", prefix, lineText)
				lines = append(lines, styledLine{text: fullText, style: lineStyle})
			}
		}
	}
	if m.errMsg != "" {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: fmt.Sprintf("Error: %s", m.errMsg), style: errorStyle})
	}
	if info := m.currentInfo(); info != "" {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: info, style: infoStyle})
	}
	if m.showFooter {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: "↑/↓ move  enter select  backspace clear  esc back  ctrl+c quit", style: footerStyle})
	}

	promptText, _ := m.filterPrompt()
	lines = append(lines, styledLine{text: promptText})

	lines = limitHeight(lines, m.height, m.width)
	lines = applyWidth(lines, m.width)
	return renderLines(lines)
}

func (m *Model) loadMenuCmd(id, title string, loader menu.Loader) tea.Cmd {
	return func() tea.Msg {
		items, err := loader(m.ctx)
		if err != nil {
			logging.Error(err)
		}
		return categoryLoadedMsg{id: id, title: title, items: items, err: err}
	}
}

func (m *Model) currentLevel() *level {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

func (m *Model) handleTextInput(msg tea.KeyMsg) bool {
	if m.loading {
		return false
	}
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyCtrlH:
		return m.removeFilterRune()
	case tea.KeyRunes:
		if msg.Alt {
			return false
		}
		return m.appendToFilter(string(msg.Runes))
	case tea.KeySpace:
		return m.appendToFilter(" ")
	}
	return false
}

func (m *Model) appendToFilter(text string) bool {
	if text == "" {
		return false
	}
	current := m.currentLevel()
	if current == nil {
		return false
	}
	current.setFilter(current.filter + text)
	m.forceClearInfo()
	m.errMsg = ""
	logging.Trace("filter.append", map[string]interface{}{"level": current.id, "filter": current.filter})
	return true
}

func (m *Model) removeFilterRune() bool {
	current := m.currentLevel()
	if current == nil || current.filter == "" {
		return false
	}
	runes := []rune(current.filter)
	current.setFilter(string(runes[:len(runes)-1]))
	m.forceClearInfo()
	m.errMsg = ""
	logging.Trace("filter.backspace", map[string]interface{}{"level": current.id, "filter": current.filter})
	return true
}

func cloneItems(items []menu.Item) []menu.Item {
	dup := make([]menu.Item, len(items))
	copy(dup, items)
	return dup
}

func (l *level) updateItems(items []menu.Item) {
	l.full = cloneItems(items)
	l.applyFilter()
}

func (l *level) setFilter(query string) {
	l.filter = query
	l.applyFilter()
}

func (l *level) applyFilter() {
	l.items = filterItems(l.full, l.filter)
	if len(l.items) == 0 {
		l.cursor = 0
		return
	}
	if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
}

func limitHeight(lines []styledLine, height, width int) []styledLine {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	if height == 1 {
		return []styledLine{{text: truncateText("…", width)}}
	}
	trimmed := make([]styledLine, 0, height)
	trimmed = append(trimmed, lines[:height-1]...)
	trimmed = append(trimmed, styledLine{text: truncateText("…", width)})
	return trimmed
}

func applyWidth(lines []styledLine, width int) []styledLine {
	if width <= 0 {
		return lines
	}
	out := make([]styledLine, len(lines))
	for i, line := range lines {
		out[i] = styledLine{
			text:  truncateText(line.text, width),
			style: line.style,
		}
	}
	return out
}

func renderLines(lines []styledLine) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		text := line.text
		if line.style != nil {
			text = line.style.Render(text)
		}
		out[i] = text
	}
	return strings.Join(out, "\n")
}

func truncateText(text string, width int) string {
	if width <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

func stylePtr(s lipgloss.Style) *lipgloss.Style {
	return &s
}

func waitForBackendEvent(w *backend.Watcher) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-w.Events()
		if !ok {
			return backendDoneMsg{}
		}
		return backendEventMsg{event: evt}
	}
}

func (m *Model) applyBackendEvent(evt backend.Event) {
	if m.backendState == nil {
		m.backendState = make(map[backend.Kind]error)
	}
	m.backendState[evt.Kind] = evt.Err
	if evt.Err != nil {
		m.backendLastErr = evt.Err.Error()
		return
	}

	switch evt.Kind {
	case backend.KindSessions:
		if names, ok := evt.Data.([]string); ok {
			m.ctx.Sessions = append([]string(nil), names...)
			if lvl := m.findLevelByID("session:switch"); lvl != nil {
				lvl.updateItems(itemsFromStrings(names))
				if len(lvl.items) > 0 {
					m.clearInfo()
				}
			}
			if m.sessionForm != nil {
				m.sessionForm.setExisting(names)
				m.sessionForm.validate()
			}
		}
	case backend.KindWindows:
		if windows, ok := evt.Data.([]tmux.Window); ok {
			m.windows = windows
			entries := menu.WindowEntriesFromTmux(windows)
			m.ctx.Windows = entries
			if lvl := m.findLevelByID("window:switch"); lvl != nil {
				lvl.updateItems(menu.WindowEntriesToItems(entries))
			}
			if lvl := m.findLevelByID("window:kill"); lvl != nil {
				lvl.updateItems(menu.WindowEntriesToItems(entries))
			}
		}
	case backend.KindPanes:
		if panes, ok := evt.Data.([]tmux.Pane); ok {
			m.panes = panes
		}
	}

	if warn, _ := m.hasBackendIssue(); !warn {
		m.backendLastErr = ""
	}
}

func (m *Model) hasBackendIssue() (bool, string) {
	for _, err := range m.backendState {
		if err != nil {
			msg := m.backendLastErr
			if msg == "" {
				msg = err.Error()
			}
			return true, msg
		}
	}
	return false, ""
}

func (m *Model) findLevelByID(id string) *level {
	for _, lvl := range m.stack {
		if lvl.id == id {
			return lvl
		}
	}
	return nil
}

func itemsFromStrings(values []string) []menu.Item {
	items := make([]menu.Item, 0, len(values))
	for _, v := range values {
		items = append(items, menu.Item{ID: v, Label: v})
	}
	return items
}

func filterItems(items []menu.Item, query string) []menu.Item {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return cloneItems(items)
	}
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	ranks := fuzzy.RankFindNormalizedFold(trimmed, labels)
	sort.Sort(ranks)
	filtered := make([]menu.Item, 0, len(ranks))
	for _, rank := range ranks {
		filtered = append(filtered, items[rank.OriginalIndex])
	}
	if len(filtered) == 0 {
		lower := strings.ToLower(trimmed)
		for _, item := range items {
			labelLower := strings.ToLower(item.Label)
			idLower := strings.ToLower(item.ID)
			if strings.Contains(labelLower, lower) || strings.Contains(idLower, lower) {
				filtered = append(filtered, item)
			}
		}
	}
	return cloneItems(filtered)
}

func (m *Model) filterPrompt() (string, *lipgloss.Style) {
	current := m.currentLevel()
	if current == nil {
		return ">", filterStyle
	}
	cursor := cursorStyle.Render("█")
	text := current.filter
	prompt := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render("> ")
	if text == "" {
		placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(type to search)")
		return prompt + placeholder + cursor, nil
	}
	return prompt + text + cursor, nil
}

func (m *Model) startSessionForm(prompt menu.SessionPrompt) {
	ti := textinput.New()
	ti.Placeholder = "session-name"
	ti.Focus()
	ti.CharLimit = 64
	state := &sessionFormState{
		input: ti,
		ctx:   prompt.Context,
	}
	state.setExisting(prompt.Context.Sessions)
	state.validate()
	m.sessionForm = state
}

func (m *Model) viewSessionForm() string {
	lines := []string{
		"Create Session",
		"",
		m.sessionForm.input.View(),
	}
	if m.sessionForm.err != "" {
		lines = append(lines, "", errorStyle.Render(m.sessionForm.err))
	}
	lines = append(lines, "", "Press Enter to create. Esc to cancel.")
	return strings.Join(lines, "\n")
}

func (m *Model) handleSessionForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.sessionForm == nil {
		return false, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			logging.Trace("session.new.cancel", nil)
			m.sessionForm = nil
			return true, nil
		case tea.KeyEnter:
			name := strings.TrimSpace(m.sessionForm.input.Value())
			if err := m.sessionForm.validate(); err != "" {
				m.sessionForm.err = err
				return true, nil
			}
			logging.Trace("session.new.submit", map[string]interface{}{"name": name})
			ctx := m.sessionForm.ctx
			m.sessionForm = nil
			m.loading = true
			m.pendingID = "session:new"
			m.pendingLabel = name
			return true, menu.SessionCreateCommand(ctx, name)
		default:
			var cmd tea.Cmd
			m.sessionForm.input, cmd = m.sessionForm.input.Update(msg)
			m.sessionForm.err = m.sessionForm.validate()
			return true, cmd
		}
	default:
		var cmd tea.Cmd
		m.sessionForm.input, cmd = m.sessionForm.input.Update(msg)
		if cmd != nil {
			return true, cmd
		}
		return false, nil
	}
}

func (s *sessionFormState) setExisting(names []string) {
	s.existing = make(map[string]struct{}, len(names))
	for _, name := range names {
		s.existing[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}
}

func (s *sessionFormState) validate() string {
	name := strings.TrimSpace(s.input.Value())
	if name == "" {
		s.err = "Session name required"
		return s.err
	}
	if s.existing != nil {
		if _, ok := s.existing[strings.ToLower(name)]; ok {
			s.err = "Session already exists"
			return s.err
		}
	}
	s.err = ""
	return ""
}

func (m *Model) setInfo(message string) {
	m.infoMsg = message
	m.infoExpire = time.Now().Add(5 * time.Second)
}

func (m *Model) clearInfo() {
	if m.infoMsg == "" {
		return
	}
	if !m.infoExpire.IsZero() && time.Now().Before(m.infoExpire) {
		return
	}
	m.infoMsg = ""
	m.infoExpire = time.Time{}
}

func (m *Model) forceClearInfo() {
	m.infoMsg = ""
	m.infoExpire = time.Time{}
}

func (m *Model) currentInfo() string {
	if m.infoMsg != "" && !m.infoExpire.IsZero() && time.Now().After(m.infoExpire) {
		m.infoMsg = ""
		m.infoExpire = time.Time{}
	}
	return m.infoMsg
}

type backendEventMsg struct {
	event backend.Event
}

type backendDoneMsg struct{}

// categoryLoadedMsg mirrors the async loader response.
type categoryLoadedMsg struct {
	id    string
	title string
	items []menu.Item
	err   error
}
