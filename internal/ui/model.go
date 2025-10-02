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
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

type level struct {
	id          string
	title       string
	items       []menu.Item
	full        []menu.Item
	filter      string
	cursor      int
	multiSelect bool
	selected    map[string]struct{}
	data        interface{}
}

type styledLine struct {
	text          string
	style         *lipgloss.Style
	highlightFrom int
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
	l := &level{id: id, title: title, cursor: -1, selected: make(map[string]struct{})}
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
	verbose        bool
	sessionForm    *menu.SessionForm
	windowForm     *menu.WindowRenameForm
	pendingSwap    *menu.Item

	categoryLoaders map[string]menu.Loader
	actionLoaders   map[string]menu.Loader
	actionHandlers  map[string]menu.Action
}

// NewModel initialises the UI state with the root menu and configuration.
func NewModel(socketPath string, width, height int, showFooter bool, verbose bool, watcher *backend.Watcher) *Model {
	root := newLevel("root", "Main Menu", menu.RootItems())
	m := &Model{
		stack:           []*level{root},
		ctx:             menu.Context{SocketPath: socketPath, IncludeCurrent: true, WindowIncludeCurrent: true},
		categoryLoaders: menu.CategoryLoaders(),
		actionLoaders:   menu.ActionLoaders(),
		actionHandlers:  menu.ActionHandlers(),
		backend:         watcher,
		backendState:    map[backend.Kind]error{},
		showFooter:      showFooter,
		verbose:         verbose,
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
	if handled, cmd := m.handleWindowForm(msg); handled {
		return m, cmd
	}
	if handled, cmd := m.handleSessionForm(msg); handled {
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyTab {
			if current := m.currentLevel(); current != nil && current.multiSelect {
				current.toggleCurrentSelection()
				return m, nil
			}
		}
		if m.handleTextInput(msg) {
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", "backspace", "left":
			if current := m.currentLevel(); current != nil && current.id == "window:swap-target" {
				m.pendingSwap = nil
			}
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
			if current.id == "window:swap-target" && m.pendingSwap != nil {
				first := *m.pendingSwap
				m.pendingSwap = nil
				m.stack = m.stack[:len(m.stack)-1]
				m.loading = true
				m.pendingID = "window:swap"
				m.pendingLabel = fmt.Sprintf("%s ↔ %s", first.Label, item.Label)
				m.errMsg = ""
				m.forceClearInfo()
				return m, menu.WindowSwapCommand(m.ctx, first.ID, item.ID, first.Label, item.Label)
			}
			if current.multiSelect {
				if selected := current.selectedItems(); len(selected) > 0 {
					ids := make([]string, 0, len(selected))
					labels := make([]string, 0, len(selected))
					for _, sel := range selected {
						ids = append(ids, sel.ID)
						labels = append(labels, sel.Label)
					}
					item = menu.Item{ID: strings.Join(ids, "\n"), Label: strings.Join(labels, ", ")}
					current.clearSelection()
				}
			}
			comboKey := fmt.Sprintf("%s:%s", current.id, item.ID)
			if loader, ok := m.actionLoaders[comboKey]; ok {
				m.loading = true
				m.pendingID = comboKey
				m.pendingLabel = item.Label
				m.errMsg = ""
				m.forceClearInfo()
				return m, m.loadMenuCmd(comboKey, item.Label, loader)
			}
			if handler := m.actionHandlers[comboKey]; handler != nil {
				m.loading = true
				m.pendingID = comboKey
				m.pendingLabel = item.Label
				m.errMsg = ""
				m.forceClearInfo()
				return m, handler(m.ctx, item)
			}
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
		switch msg.id {
		case "window:kill":
			level.multiSelect = true
		}
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
		if msg.Info != "" && m.verbose {
			m.setInfo(msg.Info)
		} else {
			m.forceClearInfo()
		}
		logging.Trace("action.success", map[string]interface{}{"info": msg.Info})
		return m, tea.Quit
	case menu.WindowPrompt:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		m.forceClearInfo()
		m.startWindowForm(msg)
		return m, nil
	case menu.WindowSwapPrompt:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		m.forceClearInfo()
		m.startWindowSwap(msg)
		return m, nil
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
	if m.windowForm != nil {
		return m.viewWindowForm()
	}
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
				lineStyle := itemStyle
				selectDisplay := ""
				if current.multiSelect {
					mark := " "
					if current.isSelected(item.ID) {
						mark = "✓"
					}
					selectDisplay = fmt.Sprintf("[%s] ", mark)
				}
				if i == current.cursor {
					prefix = "▌ "
					lineStyle = selectedItemStyle
				}
				lineText := selectDisplay + item.Label
				fullText := fmt.Sprintf("%s%s", prefix, lineText)
				highlightFrom := 0
				if lineStyle == selectedItemStyle {
					highlightFrom = len([]rune(prefix)) + len([]rune(selectDisplay))
				}
				lines = append(lines, styledLine{text: fullText, style: lineStyle, highlightFrom: highlightFrom})
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
		lines = append(lines, styledLine{text: "↑/↓ move  enter select  tab mark  backspace clear  esc back  ctrl+c quit", style: footerStyle})
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

func (l *level) cleanupSelections() {
	if len(l.selected) == 0 {
		return
	}
	valid := make(map[string]struct{}, len(l.full))
	for _, item := range l.full {
		valid[item.ID] = struct{}{}
	}
	for id := range l.selected {
		if _, ok := valid[id]; !ok {
			delete(l.selected, id)
		}
	}
}

func (l *level) isSelected(id string) bool {
	if l.selected == nil {
		return false
	}
	_, ok := l.selected[id]
	return ok
}

func (l *level) toggleSelection(id string) {
	if l.selected == nil {
		l.selected = make(map[string]struct{})
	}
	if _, ok := l.selected[id]; ok {
		delete(l.selected, id)
	} else {
		l.selected[id] = struct{}{}
	}
}

func (l *level) toggleCurrentSelection() {
	if !l.multiSelect || l.cursor < 0 || l.cursor >= len(l.items) {
		return
	}
	l.toggleSelection(l.items[l.cursor].ID)
}

func (l *level) clearSelection() {
	for id := range l.selected {
		delete(l.selected, id)
	}
}

func (l *level) selectedItems() []menu.Item {
	if len(l.selected) == 0 {
		return nil
	}
	selected := make([]menu.Item, 0, len(l.selected))
	for _, item := range l.items {
		if l.isSelected(item.ID) {
			selected = append(selected, item)
		}
	}
	return selected
}

func sessionSwitchItems(ctx menu.Context) []menu.Item {
	items := make([]menu.Item, 0, len(ctx.Sessions))
	for _, sess := range ctx.Sessions {
		if !ctx.IncludeCurrent && sess.Current {
			continue
		}
		items = append(items, menu.Item{ID: sess.Name, Label: sess.Label})
	}
	return items
}

func currentWindowMenuItem(ctx menu.Context) (menu.Item, bool) {
	id := strings.TrimSpace(ctx.CurrentWindowID)
	if id == "" {
		return menu.Item{}, false
	}
	label := strings.TrimSpace(ctx.CurrentWindowLabel)
	if label == "" {
		label = id
	}
	return menu.Item{ID: id, Label: fmt.Sprintf("[current] %s", label)}, true
}

func (l *level) updateItems(items []menu.Item) {
	l.full = cloneItems(items)
	l.cleanupSelections()
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
	if l.cursor < 0 {
		l.cursor = len(l.items) - 1
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
			text:          truncateText(line.text, width),
			style:         line.style,
			highlightFrom: line.highlightFrom,
		}
	}
	return out
}

func renderLines(lines []styledLine) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		text := line.text
		if line.style != nil {
			runes := []rune(text)
			if line.highlightFrom <= 0 || line.highlightFrom >= len(runes) {
				text = line.style.Render(text)
			} else {
				head := string(runes[:line.highlightFrom])
				tail := string(runes[line.highlightFrom:])
				text = head + line.style.Render(tail)
			}
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
		if snapshot, ok := evt.Data.(tmux.SessionSnapshot); ok {
			entries := menu.SessionEntriesFromTmux(snapshot.Sessions)
			m.ctx.Sessions = entries
			m.ctx.Current = snapshot.Current
			m.ctx.IncludeCurrent = snapshot.IncludeCurrent
			if lvl := m.findLevelByID("session:switch"); lvl != nil {
				items := sessionSwitchItems(m.ctx)
				lvl.updateItems(items)
				if len(lvl.items) > 0 {
					m.clearInfo()
				}
			}
			items := menu.SessionEntriesToItems(entries)
			for _, id := range []string{"session:rename", "session:detach", "session:kill"} {
				if lvl := m.findLevelByID(id); lvl != nil {
					lvl.updateItems(items)
				}
			}
			if m.sessionForm != nil {
				m.sessionForm.SetSessions(entries)
			}
		}
	case backend.KindWindows:
		if snapshot, ok := evt.Data.(tmux.WindowSnapshot); ok {
			m.windows = snapshot.Windows
			entries := menu.WindowEntriesFromTmux(snapshot.Windows)
			m.ctx.Windows = entries
			m.ctx.CurrentWindowID = snapshot.CurrentID
			m.ctx.CurrentWindowLabel = snapshot.CurrentLabel
			m.ctx.CurrentWindowSession = snapshot.CurrentSession
			m.ctx.WindowIncludeCurrent = snapshot.IncludeCurrent
			m.pendingSwap = nil
			if lvl := m.findLevelByID("window:switch"); lvl != nil {
				items := make([]menu.Item, 0, len(entries))
				for _, entry := range entries {
					if entry.Current && !m.ctx.WindowIncludeCurrent {
						continue
					}
					items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
				}
				lvl.updateItems(items)
			}
			if lvl := m.findLevelByID("window:rename"); lvl != nil {
				items := menu.WindowEntriesToItems(entries)
				if currentItem, ok := currentWindowMenuItem(m.ctx); ok {
					items = append([]menu.Item{currentItem}, items...)
				}
				lvl.updateItems(items)
			}
			if lvl := m.findLevelByID("window:link"); lvl != nil {
				items := make([]menu.Item, 0, len(entries))
				for _, entry := range entries {
					if entry.Session == m.ctx.CurrentWindowSession {
						continue
					}
					items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
				}
				lvl.updateItems(items)
			}
			if lvl := m.findLevelByID("window:move"); lvl != nil {
				items := make([]menu.Item, 0, len(entries))
				for _, entry := range entries {
					if entry.Session == m.ctx.CurrentWindowSession {
						continue
					}
					items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
				}
				lvl.updateItems(items)
			}
			if lvl := m.findLevelByID("window:swap"); lvl != nil {
				items := menu.WindowEntriesToItems(entries)
				if currentItem, ok := currentWindowMenuItem(m.ctx); ok {
					items = append([]menu.Item{currentItem}, items...)
				}
				lvl.updateItems(items)
			}
			if lvl := m.findLevelByID("window:kill"); lvl != nil {
				items := menu.WindowEntriesToItems(entries)
				if currentItem, ok := currentWindowMenuItem(m.ctx); ok {
					items = append([]menu.Item{currentItem}, items...)
				}
				lvl.updateItems(items)
				lvl.multiSelect = true
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
	m.sessionForm = menu.NewSessionForm(prompt)
}

func (m *Model) startWindowForm(prompt menu.WindowPrompt) {
	m.windowForm = menu.NewWindowRenameForm(prompt)
}

func (m *Model) startWindowSwap(prompt menu.WindowSwapPrompt) {
	label := prompt.First.Label
	for _, entry := range m.ctx.Windows {
		if entry.ID == prompt.First.ID {
			label = entry.Label
			break
		}
	}
	items := make([]menu.Item, 0, len(m.ctx.Windows))
	for _, entry := range m.ctx.Windows {
		if entry.ID == prompt.First.ID {
			continue
		}
		items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
	}
	if len(items) == 0 {
		m.setInfo("No windows available to swap with.")
		return
	}
	level := newLevel("window:swap-target", fmt.Sprintf("Swap %s with…", label), items)
	m.pendingSwap = &menu.Item{ID: prompt.First.ID, Label: label}
	m.stack = append(m.stack, level)
}

func (m *Model) viewWindowForm() string {
	lines := []string{
		m.windowForm.Title(),
		"",
		m.windowForm.InputView(),
		"",
		m.windowForm.Help(),
	}
	return strings.Join(lines, "\n")
}

func (m *Model) viewSessionForm() string {
	lines := []string{
		m.sessionForm.Title(),
		"",
		m.sessionForm.InputView(),
	}
	if err := m.sessionForm.Error(); err != "" {
		lines = append(lines, "", errorStyle.Render(err))
	}
	lines = append(lines, "", m.sessionForm.Help())
	return strings.Join(lines, "\n")
}

func (m *Model) handleWindowForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.windowForm == nil {
		return false, nil
	}
	cmd, done, cancel := m.windowForm.Update(msg)
	if cancel {
		m.windowForm = nil
		return true, cmd
	}
	if done {
		ctx := m.windowForm.Context()
		name := m.windowForm.Value()
		target := m.windowForm.Target()
		actionID := m.windowForm.ActionID()
		pendingLabel := m.windowForm.PendingLabel()
		m.windowForm = nil
		m.loading = true
		m.pendingID = actionID
		m.pendingLabel = pendingLabel
		if cmd == nil {
			cmd = menu.WindowRenameCommand(ctx, target, name)
		}
		return true, cmd
	}
	if cmd != nil {
		return true, cmd
	}
	return true, nil
}

func (m *Model) handleSessionForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.sessionForm == nil {
		return false, nil
	}
	cmd, done, cancel := m.sessionForm.Update(msg)
	if cancel {
		m.sessionForm = nil
		return true, cmd
	}
	if done {
		ctx := m.sessionForm.Context()
		name := m.sessionForm.Value()
		target := m.sessionForm.Target()
		actionID := m.sessionForm.ActionID()
		pendingLabel := m.sessionForm.PendingLabel()
		m.sessionForm = nil
		m.loading = true
		m.pendingID = actionID
		m.pendingLabel = pendingLabel
		if cmd == nil {
			sessionCmd := menu.SessionCommandForAction(actionID, ctx, target, name)
			cmd = sessionCmd
		}
		return true, cmd
	}
	if cmd != nil {
		return true, cmd
	}
	return true, nil
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
