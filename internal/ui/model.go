package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/data/dispatcher"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/state"
	"github.com/atomicstack/tmux-popup-control/internal/theme"
	"github.com/atomicstack/tmux-popup-control/internal/ui/command"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

type level struct {
	id             string
	title          string
	items          []menu.Item
	full           []menu.Item
	filter         string
	filterCursor   int
	cursor         int
	multiSelect    bool
	selected       map[string]struct{}
	data           interface{}
	lastCursor     int
	node           *menu.Node
	viewportOffset int
}

type styledLine struct {
	text          string
	style         *lipgloss.Style
	highlightFrom int
}

type Mode int

const (
	ModeMenu Mode = iota
	ModePaneForm
	ModeWindowForm
	ModeSessionForm
)

const (
	menuHeaderSeparator = "→"
)

var styles = theme.Default()

var headerSegmentCleaner = strings.NewReplacer("_", " ", "-", " ")

func newLevel(id, title string, items []menu.Item, node *menu.Node) *level {
	l := &level{id: id, title: title, cursor: -1, lastCursor: -1, selected: make(map[string]struct{}), node: node}
	l.updateItems(items)
	return l
}

// Model implements the Bubble Tea model for the tmux popup menu.
type Model struct {
	stack             []*level
	loading           bool
	pendingID         string
	pendingLabel      string
	errMsg            string
	infoMsg           string
	infoExpire        time.Time
	width             int
	height            int
	fixedWidth        bool
	fixedHeight       bool
	backend           *backend.Watcher
	backendState      map[backend.Kind]error
	backendLastErr    string
	showFooter        bool
	verbose           bool
	sessionForm       *menu.SessionForm
	windowForm        *menu.WindowRenameForm
	paneForm          *menu.PaneRenameForm
	pendingWindowSwap *menu.Item
	pendingPaneSwap   *menu.Item

	registry   *menu.Registry
	bus        *command.Bus
	mode       Mode
	socketPath string
	sessions   state.SessionStore
	windows    state.WindowStore
	panes      state.PaneStore
	dispatcher *dispatcher.Dispatcher
}

// NewModel initialises the UI state with the root menu and configuration.
func NewModel(socketPath string, width, height int, showFooter bool, verbose bool, watcher *backend.Watcher) *Model {
	registry := menu.BuildRegistry()
	sessions := state.NewSessionStore()
	sessions.SetIncludeCurrent(true)
	windows := state.NewWindowStore()
	windows.SetIncludeCurrent(true)
	panes := state.NewPaneStore()
	panes.SetIncludeCurrent(true)
	rootItems := menu.RootItems()
	root := newLevel("root", "Main Menu", rootItems, registry.Root())
	m := &Model{
		stack:        []*level{root},
		registry:     registry,
		bus:          command.New(),
		backend:      watcher,
		backendState: map[backend.Kind]error{},
		showFooter:   showFooter,
		verbose:      verbose,
		mode:         ModeMenu,
		socketPath:   socketPath,
		sessions:     sessions,
		windows:      windows,
		panes:        panes,
		dispatcher:   dispatcher.New(sessions, windows, panes),
	}
	m.applyNodeSettings(root)
	m.syncViewport(root)
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
	switch m.mode {
	case ModePaneForm:
		if handled, cmd := m.handlePaneForm(msg); handled {
			return m, cmd
		}
	case ModeWindowForm:
		if handled, cmd := m.handleWindowForm(msg); handled {
			return m, cmd
		}
	case ModeSessionForm:
		if handled, cmd := m.handleSessionForm(msg); handled {
			return m, cmd
		}
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode != ModeMenu {
			return m, nil
		}
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
		case "esc":
			current := m.currentLevel()
			if current == nil {
				return m, tea.Quit
			}
			if len(m.stack) <= 1 {
				return m, tea.Quit
			}
			if current != nil && current.id == "window:swap-target" {
				m.pendingWindowSwap = nil
			}
			if current != nil && current.id == "pane:swap-target" {
				m.pendingPaneSwap = nil
			}
			parent := m.stack[len(m.stack)-2]
			m.stack = m.stack[:len(m.stack)-1]
			if parent != nil {
				if parent.lastCursor >= 0 && parent.lastCursor < len(parent.items) {
					parent.cursor = parent.lastCursor
				} else if current != nil {
					if idx := parent.indexOf(current.id); idx >= 0 {
						parent.cursor = idx
					} else if len(parent.items) > 0 {
						parent.cursor = len(parent.items) - 1
					}
				}
				parent.lastCursor = -1
				m.syncViewport(parent)
			}
			m.errMsg = ""
			m.forceClearInfo()
		case "enter":
			if m.loading {
				return m, nil
			}
			current := m.currentLevel()
			if current == nil || len(current.items) == 0 {
				return m, nil
			}
			ctx := m.menuContext()
			item := current.items[current.cursor]
			events.UI.MenuEnter(current.id, item.ID, item.Label, current.filter)
			current.setFilter("", 0)
			if current.id == "window:swap-target" && m.pendingWindowSwap != nil {
				first := *m.pendingWindowSwap
				m.pendingWindowSwap = nil
				m.stack = m.stack[:len(m.stack)-1]
				m.loading = true
				m.pendingID = "window:swap"
				m.pendingLabel = fmt.Sprintf("%s ↔ %s", first.Label, item.Label)
				m.errMsg = ""
				m.forceClearInfo()
				return m, menu.WindowSwapCommand(ctx, first.ID, item.ID, first.Label, item.Label)
			}
			if current.id == "pane:swap-target" && m.pendingPaneSwap != nil {
				first := *m.pendingPaneSwap
				m.pendingPaneSwap = nil
				m.stack = m.stack[:len(m.stack)-1]
				m.loading = true
				m.pendingID = "pane:swap"
				m.pendingLabel = fmt.Sprintf("%s ↔ %s", first.Label, item.Label)
				m.errMsg = ""
				m.forceClearInfo()
				return m, menu.PaneSwapCommand(ctx, first, item)
			}
			node := current.node
			if node == nil {
				node, _ = m.registry.Find(current.id)
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
			if node != nil {
				if child, ok := node.Children[item.ID]; ok {
					if child.Loader != nil {
						current.lastCursor = current.cursor
						m.loading = true
						m.pendingID = child.ID
						m.pendingLabel = item.Label
						m.errMsg = ""
						m.forceClearInfo()
						return m, m.loadMenuCmd(child.ID, item.Label, child.Loader)
					}
					if child.Action != nil {
						m.loading = true
						m.pendingID = child.ID
						m.pendingLabel = item.Label
						m.errMsg = ""
						m.forceClearInfo()
						return m, m.bus.Execute(ctx, command.Request{ID: child.ID, Label: item.Label, Handler: child.Action, Item: item})
					}
				}
				if node.Action != nil {
					m.loading = true
					m.pendingID = node.ID
					m.pendingLabel = item.Label
					m.errMsg = ""
					m.forceClearInfo()
					return m, m.bus.Execute(ctx, command.Request{ID: node.ID, Label: item.Label, Handler: node.Action, Item: item})
				}
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
					events.UI.MenuCursor(current.id, current.cursor)
					m.syncViewport(current)
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
					events.UI.MenuCursor(current.id, current.cursor)
					m.syncViewport(current)
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
		if current := m.currentLevel(); current != nil {
			m.syncViewport(current)
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
		node, _ := m.registry.Find(msg.id)
		level := newLevel(msg.id, msg.title, msg.items, node)
		m.applyNodeSettings(level)
		m.syncViewport(level)
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
			events.Action.Error(msg.Err)
			return m, nil
		}
		if msg.Info != "" && m.verbose {
			m.setInfo(msg.Info)
		} else {
			m.forceClearInfo()
		}
		events.Action.Success(msg.Info)
		return m, tea.Quit
	case menu.WindowPrompt:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		m.forceClearInfo()
		m.startWindowForm(msg)
		return m, nil
	case menu.PanePrompt:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		m.forceClearInfo()
		m.startPaneForm(msg)
		return m, nil
	case menu.WindowSwapPrompt:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		m.forceClearInfo()
		m.startWindowSwap(msg)
		return m, nil
	case menu.PaneSwapPrompt:
		m.loading = false
		m.pendingID = ""
		m.pendingLabel = ""
		m.forceClearInfo()
		m.startPaneSwap(msg)
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
	header := m.menuHeader()
	switch m.mode {
	case ModePaneForm:
		if m.paneForm != nil {
			return m.viewPaneFormWithHeader(header)
		}
	case ModeWindowForm:
		if m.windowForm != nil {
			return m.viewWindowFormWithHeader(header)
		}
	case ModeSessionForm:
		if m.sessionForm != nil {
			return m.viewSessionFormWithHeader(header)
		}
	}
	lines := make([]styledLine, 0, 16)
	if header != "" {
		lines = append(lines, styledLine{text: header, style: styles.Header})
	}
	if m.loading {
		label := m.pendingLabel
		if label == "" {
			label = m.pendingID
		}
		if label == "" {
			label = "items"
		}
	} else if current := m.currentLevel(); current != nil {
		m.syncViewport(current)
		start := 0
		displayItems := current.items
		if maxItems := m.maxVisibleItems(); maxItems > 0 && len(displayItems) > maxItems {
			start = current.viewportOffset
			if start < 0 {
				start = 0
			}
			if start+maxItems > len(displayItems) {
				start = len(displayItems) - maxItems
				if start < 0 {
					start = 0
				}
				current.viewportOffset = start
			}
			displayItems = displayItems[start : start+maxItems]
		}
		if len(current.items) == 0 {
			msg := "(no entries)"
			if current.filter != "" {
				msg = fmt.Sprintf("No matches for %q", current.filter)
			}
			lines = append(lines, styledLine{text: msg, style: styles.Info})
		} else {
			for i, item := range displayItems {
				idx := start + i
				prefix := "  "
				lineStyle := styles.Item
				selectDisplay := ""
				if current.multiSelect {
					mark := " "
					if current.isSelected(item.ID) {
						mark = "✓"
					}
					selectDisplay = fmt.Sprintf("[%s] ", mark)
				}
				if idx == current.cursor {
					prefix = "▌ "
					lineStyle = styles.SelectedItem
				}
				lineText := selectDisplay + item.Label
				fullText := fmt.Sprintf("%s%s", prefix, lineText)
				highlightFrom := 0
				if lineStyle == styles.SelectedItem {
					highlightFrom = len([]rune(prefix)) + len([]rune(selectDisplay))
				}
				lines = append(lines, styledLine{text: fullText, style: lineStyle, highlightFrom: highlightFrom})
			}
		}
	}
	if m.errMsg != "" {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: fmt.Sprintf("Error: %s", m.errMsg), style: styles.Error})
	}
	if info := m.currentInfo(); info != "" {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: info, style: styles.Info})
	}
	if m.showFooter {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: "↑/↓ move  enter select  tab mark  backspace clear  esc back  ctrl+c quit", style: styles.Footer})
	}

	promptText, _ := m.filterPrompt()
	lines = append(lines, styledLine{text: promptText})

	lines = limitHeight(lines, m.height, m.width)
	lines = applyWidth(lines, m.width)
	return renderLines(lines)
}

func (m *Model) menuHeader() string {
	segments := m.headerSegments()
	if len(segments) == 0 {
		return ""
	}
	return strings.Join(segments, menuHeaderSeparator)
}

func (m *Model) headerSegments() []string {
	depth := len(m.stack)
	if depth == 0 {
		return nil
	}
	if depth == 1 {
		return []string{"main menu"}
	}
	segments := make([]string, 0, depth-1)
	for i := 1; i < depth; i++ {
		segment := headerSegmentForLevel(m.stack[i])
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
	}
	if len(segments) == 0 {
		return []string{"main menu"}
	}
	return segments
}

func headerSegmentForLevel(l *level) string {
	if l == nil {
		return ""
	}
	candidate := strings.TrimSpace(l.id)
	if candidate == "" {
		candidate = strings.TrimSpace(l.title)
	}
	if candidate == "" {
		return ""
	}
	if idx := strings.LastIndex(candidate, ":"); idx >= 0 {
		candidate = candidate[idx+1:]
	}
	candidate = headerSegmentCleaner.Replace(candidate)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	fields := strings.Fields(strings.ToLower(candidate))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func (m *Model) loadMenuCmd(id, title string, loader menu.Loader) tea.Cmd {
	return func() tea.Msg {
		items, err := loader(m.menuContext())
		if err != nil {
			logging.Error(err)
		}
		return categoryLoadedMsg{id: id, title: title, items: items, err: err}
	}
}

func (m *Model) applyNodeSettings(l *level) {
	if l == nil {
		return
	}
	if l.node == nil {
		if node, ok := m.registry.Find(l.id); ok {
			l.node = node
		}
	}
	if l.node != nil {
		l.multiSelect = l.node.MultiSelect
	}
}

func (m *Model) currentLevel() *level {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

func (m *Model) menuContext() menu.Context {
	ctx := menu.Context{
		SocketPath:           m.socketPath,
		Sessions:             m.sessions.Entries(),
		Current:              m.sessions.Current(),
		IncludeCurrent:       m.sessions.IncludeCurrent(),
		Windows:              m.windows.Entries(),
		CurrentWindowID:      m.windows.CurrentID(),
		CurrentWindowLabel:   m.windows.CurrentLabel(),
		CurrentWindowSession: m.windows.CurrentSession(),
		WindowIncludeCurrent: m.windows.IncludeCurrent(),
		Panes:                m.panes.Entries(),
		CurrentPaneID:        m.panes.CurrentID(),
		CurrentPaneLabel:     m.panes.CurrentLabel(),
		PaneIncludeCurrent:   m.panes.IncludeCurrent(),
	}
	return ctx
}

func (m *Model) maxVisibleItems() int {
	if m.height <= 0 {
		return -1
	}
	used := 1 // filter prompt
	if header := m.menuHeader(); header != "" {
		used++
	}
	if m.errMsg != "" {
		used += 2
	}
	if info := m.currentInfo(); info != "" {
		used += 2
	}
	if m.showFooter {
		used += 2
	}
	remain := m.height - used
	if remain < 1 {
		return 1
	}
	return remain
}

func (m *Model) syncViewport(l *level) {
	if l == nil {
		return
	}
	l.ensureCursorVisible(m.maxVisibleItems())
}

func (m *Model) handleTextInput(msg tea.KeyMsg) bool {
	if m.loading {
		return false
	}
	current := m.currentLevel()
	if current == nil {
		return false
	}
	switch msg.String() {
	case "ctrl+u":
		if current.filter == "" {
			return false
		}
		current.setFilter("", 0)
		m.forceClearInfo()
		m.errMsg = ""
		events.Filter.Cleared(current.id)
		m.syncViewport(current)
		return true
	case "ctrl+w":
		if !current.deleteFilterWordBackward() {
			return false
		}
		m.forceClearInfo()
		m.errMsg = ""
		events.Filter.WordBackspace(current.id, current.filter)
		m.syncViewport(current)
		return true
	case "ctrl+a":
		if !current.moveFilterCursorStart() {
			return false
		}
		events.Filter.Cursor(current.id, current.filterCursor)
		return true
	case "ctrl+e":
		if !current.moveFilterCursorEnd() {
			return false
		}
		events.Filter.Cursor(current.id, current.filterCursor)
		return true
	case "alt+b":
		if !current.moveFilterCursorWordBackward() {
			return false
		}
		events.Filter.CursorWord(current.id, current.filterCursor)
		return true
	case "alt+f":
		if !current.moveFilterCursorWordForward() {
			return false
		}
		events.Filter.CursorWord(current.id, current.filterCursor)
		return true
	}
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyCtrlH:
		return m.removeFilterRune()
	case tea.KeyRunes:
		if msg.Alt {
			return false
		}
		if len(msg.Runes) == 0 {
			return false
		}
		for _, r := range msg.Runes {
			if unicode.IsControl(r) {
				return false
			}
			if unicode.IsSpace(r) {
				// allow the dedicated space handler to manage spaces
				return false
			}
		}
		return m.appendToFilter(string(msg.Runes))
	case tea.KeySpace:
		return m.appendToFilter(" ")
	case tea.KeyLeft:
		if !current.moveFilterCursorRuneBackward() {
			return false
		}
		events.Filter.Cursor(current.id, current.filterCursor)
		return true
	case tea.KeyRight:
		if !current.moveFilterCursorRuneForward() {
			return false
		}
		events.Filter.Cursor(current.id, current.filterCursor)
		return true
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
	if !current.insertFilterText(text) {
		return false
	}
	m.forceClearInfo()
	m.errMsg = ""
	events.Filter.Append(current.id, current.filter)
	m.syncViewport(current)
	return true
}

func (m *Model) removeFilterRune() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if !current.deleteFilterRuneBackward() {
		return false
	}
	m.forceClearInfo()
	m.errMsg = ""
	events.Filter.Backspace(current.id, current.filter)
	m.syncViewport(current)
	return true
}

func cloneItems(items []menu.Item) []menu.Item {
	dup := make([]menu.Item, len(items))
	copy(dup, items)
	return dup
}

func (l *level) indexOf(id string) int {
	if id == "" {
		return -1
	}
	for i, item := range l.items {
		if item.ID == id {
			return i
		}
	}
	if idx := strings.LastIndex(id, ":"); idx >= 0 {
		suffix := id[idx+1:]
		for i, item := range l.items {
			if item.ID == suffix {
				return i
			}
		}
	}
	return -1
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

func (l *level) ensureCursorVisible(maxVisible int) {
	if len(l.items) == 0 {
		l.cursor = 0
		l.viewportOffset = 0
		return
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
	if maxVisible <= 0 {
		l.viewportOffset = 0
		return
	}
	maxOffset := len(l.items) - maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if l.viewportOffset > maxOffset {
		l.viewportOffset = maxOffset
	}
	if l.viewportOffset < 0 {
		l.viewportOffset = 0
	}
	if l.cursor < l.viewportOffset {
		l.viewportOffset = l.cursor
	}
	upper := l.viewportOffset + maxVisible - 1
	if l.cursor > upper {
		l.viewportOffset = l.cursor - maxVisible + 1
		if l.viewportOffset < 0 {
			l.viewportOffset = 0
		}
		if l.viewportOffset > maxOffset {
			l.viewportOffset = maxOffset
		}
	}
}

func sessionSwitchItems(ctx menu.Context) []menu.Item {
	return menu.SessionSwitchMenuItems(ctx)
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

func currentPaneMenuItem(ctx menu.Context) (menu.Item, bool) {
	id := strings.TrimSpace(ctx.CurrentPaneID)
	if id == "" {
		return menu.Item{}, false
	}
	label := strings.TrimSpace(ctx.CurrentPaneLabel)
	if label == "" {
		label = id
	}
	return menu.Item{ID: id, Label: fmt.Sprintf("[current] %s", label)}, true
}

func paneItems(ctx menu.Context) []menu.Item {
	items := make([]menu.Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}

func paneSwitchItems(ctx menu.Context) []menu.Item {
	items := make([]menu.Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		if entry.Current && !ctx.PaneIncludeCurrent {
			continue
		}
		items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}

func paneBreakItems(ctx menu.Context) []menu.Item {
	items := paneItems(ctx)
	if current, ok := currentPaneMenuItem(ctx); ok {
		items = append([]menu.Item{current}, items...)
	}
	return items
}

func paneJoinItems(ctx menu.Context) []menu.Item {
	items := make([]menu.Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		if entry.Current {
			continue
		}
		items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}

func paneSwapItems(ctx menu.Context) []menu.Item {
	items := paneItems(ctx)
	if current, ok := currentPaneMenuItem(ctx); ok {
		items = append([]menu.Item{current}, items...)
	}
	return items
}

func paneKillItems(ctx menu.Context) []menu.Item {
	items := paneItems(ctx)
	if current, ok := currentPaneMenuItem(ctx); ok {
		items = append([]menu.Item{current}, items...)
	}
	return items
}

func (l *level) updateItems(items []menu.Item) {
	prevOffset := l.viewportOffset
	l.full = cloneItems(items)
	l.cleanupSelections()
	l.applyFilter()
	if len(l.items) == 0 {
		l.viewportOffset = 0
		return
	}
	if prevOffset < 0 {
		prevOffset = 0
	}
	if prevOffset > len(l.items)-1 {
		l.viewportOffset = 0
		return
	}
	l.viewportOffset = prevOffset
}

func (l *level) setFilter(query string, cursor int) {
	trimmed := strings.TrimSpace(query)
	prevTrimmed := strings.TrimSpace(l.filter)
	restore := -1
	l.filter = query
	runes := []rune(l.filter)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	l.filterCursor = cursor
	if trimmed != "" {
		if prevTrimmed == "" {
			l.lastCursor = l.cursor
		}
		l.cursor = 0
	} else if prevTrimmed != "" {
		restore = l.lastCursor
	}
	l.applyFilter()
	if trimmed != "" && len(l.items) > 0 {
		if idx := bestMatchIndex(l.items, trimmed); idx >= 0 {
			l.cursor = idx
		}
	}
	if trimmed == "" && prevTrimmed != "" {
		if restore >= 0 && restore < len(l.items) {
			l.cursor = restore
		} else if len(l.items) > 0 {
			l.cursor = len(l.items) - 1
		}
		l.lastCursor = -1
	}
}

func (l *level) applyFilter() {
	l.items = filterItems(l.full, l.filter)
	if len(l.items) == 0 {
		l.cursor = 0
		l.viewportOffset = 0
		return
	}
	if l.cursor < 0 {
		l.cursor = len(l.items) - 1
		return
	}
	if l.cursor >= len(l.items) {
		l.cursor = len(l.items) - 1
	}
	if l.viewportOffset > len(l.items)-1 {
		l.viewportOffset = 0
	}
}

func (l *level) filterCursorPos() int {
	runes := []rune(l.filter)
	if l.filterCursor < 0 {
		return 0
	}
	if l.filterCursor > len(runes) {
		return len(runes)
	}
	return l.filterCursor
}

func (l *level) insertFilterText(text string) bool {
	if text == "" {
		return false
	}
	insert := []rune(text)
	if len(insert) == 0 {
		return false
	}
	runes := []rune(l.filter)
	pos := l.filterCursorPos()
	updated := make([]rune, 0, len(runes)+len(insert))
	updated = append(updated, runes[:pos]...)
	updated = append(updated, insert...)
	updated = append(updated, runes[pos:]...)
	l.setFilter(string(updated), pos+len(insert))
	return true
}

func (l *level) deleteFilterRuneBackward() bool {
	runes := []rune(l.filter)
	pos := l.filterCursorPos()
	if pos == 0 || len(runes) == 0 {
		return false
	}
	updated := append(runes[:pos-1], runes[pos:]...)
	l.setFilter(string(updated), pos-1)
	return true
}

func (l *level) deleteFilterWordBackward() bool {
	runes := []rune(l.filter)
	pos := l.filterCursorPos()
	if pos == 0 || len(runes) == 0 {
		return false
	}
	i := pos
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	updated := append(runes[:i], runes[pos:]...)
	l.setFilter(string(updated), i)
	return true
}

func (l *level) moveFilterCursorStart() bool {
	if l.filterCursorPos() == 0 {
		return false
	}
	l.filterCursor = 0
	return true
}

func (l *level) moveFilterCursorEnd() bool {
	end := len([]rune(l.filter))
	if l.filterCursorPos() == end {
		return false
	}
	l.filterCursor = end
	return true
}

func (l *level) moveFilterCursorWordBackward() bool {
	runes := []rune(l.filter)
	pos := l.filterCursorPos()
	if pos == 0 || len(runes) == 0 {
		return false
	}
	i := pos
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	if i == pos {
		return false
	}
	l.filterCursor = i
	return true
}

func (l *level) moveFilterCursorWordForward() bool {
	runes := []rune(l.filter)
	pos := l.filterCursorPos()
	if pos >= len(runes) {
		return false
	}
	i := pos
	for i < len(runes) && unicode.IsSpace(runes[i]) {
		i++
	}
	for i < len(runes) && !unicode.IsSpace(runes[i]) {
		i++
	}
	if i == pos {
		return false
	}
	l.filterCursor = i
	return true
}

func (l *level) moveFilterCursorRuneBackward() bool {
	if l.filterCursorPos() == 0 {
		return false
	}
	l.filterCursor = l.filterCursorPos() - 1
	return true
}

func (l *level) moveFilterCursorRuneForward() bool {
	runes := []rune(l.filter)
	pos := l.filterCursorPos()
	if pos >= len(runes) {
		return false
	}
	l.filterCursor = pos + 1
	return true
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

	res := m.dispatcher.Handle(evt)
	ctx := m.menuContext()

	if res.SessionsUpdated {
		if lvl := m.findLevelByID("session:switch"); lvl != nil {
			items := sessionSwitchItems(ctx)
			lvl.updateItems(items)
			if len(lvl.items) > 0 {
				m.clearInfo()
			}
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("session:rename"); lvl != nil {
			lvl.updateItems(menu.SessionRenameItems(ctx.Sessions))
			m.syncViewport(lvl)
		}
		base := menu.SessionEntriesToItems(ctx.Sessions)
		for _, id := range []string{"session:detach", "session:kill"} {
			if lvl := m.findLevelByID(id); lvl != nil {
				lvl.updateItems(base)
				m.syncViewport(lvl)
			}
		}
		if m.sessionForm != nil {
			m.sessionForm.SetSessions(ctx.Sessions)
		}
	}

	if res.WindowsUpdated {
		m.pendingWindowSwap = nil
		if lvl := m.findLevelByID("window:switch"); lvl != nil {
			lvl.updateItems(menu.WindowSwitchItems(ctx))
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("window:move"); lvl != nil {
			items := make([]menu.Item, 0, len(ctx.Windows))
			for _, entry := range ctx.Windows {
				if entry.Session == ctx.CurrentWindowSession {
					continue
				}
				items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
			}
			lvl.updateItems(items)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("window:swap"); lvl != nil {
			items := menu.WindowEntriesToItems(ctx.Windows)
			if currentItem, ok := currentWindowMenuItem(ctx); ok {
				items = append([]menu.Item{currentItem}, items...)
			}
			lvl.updateItems(items)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("window:kill"); lvl != nil {
			items := menu.WindowEntriesToItems(ctx.Windows)
			if currentItem, ok := currentWindowMenuItem(ctx); ok {
				items = append([]menu.Item{currentItem}, items...)
			}
			lvl.updateItems(items)
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
	}

	if res.PanesUpdated {
		m.pendingPaneSwap = nil
		if lvl := m.findLevelByID("pane:switch"); lvl != nil {
			lvl.updateItems(paneSwitchItems(ctx))
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:break"); lvl != nil {
			lvl.updateItems(paneBreakItems(ctx))
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:join"); lvl != nil {
			lvl.updateItems(paneJoinItems(ctx))
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:swap"); lvl != nil {
			lvl.updateItems(paneSwapItems(ctx))
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:kill"); lvl != nil {
			lvl.updateItems(paneKillItems(ctx))
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:rename"); lvl != nil {
			lvl.updateItems(menu.PaneEntriesToItems(ctx.Panes))
			m.syncViewport(lvl)
		}
		if m.paneForm != nil {
			m.paneForm.SyncContext(ctx)
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
			m.applyNodeSettings(lvl)
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
	if len(ranks) > 0 {
		matches := make(map[int]struct{}, len(ranks))
		for _, rank := range ranks {
			matches[rank.OriginalIndex] = struct{}{}
		}
		filtered := make([]menu.Item, 0, len(matches))
		for idx, item := range items {
			if _, ok := matches[idx]; ok {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) > 0 {
			return cloneItems(filtered)
		}
	}
	lower := strings.ToLower(trimmed)
	filtered := make([]menu.Item, 0, len(items))
	for _, item := range items {
		labelLower := strings.ToLower(item.Label)
		idLower := strings.ToLower(item.ID)
		if strings.Contains(labelLower, lower) || strings.Contains(idLower, lower) {
			filtered = append(filtered, item)
		}
	}
	return cloneItems(filtered)
}

func bestMatchIndex(items []menu.Item, query string) int {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		if len(items) == 0 {
			return -1
		}
		return 0
	}
	lower := strings.ToLower(trimmed)
	for i, item := range items {
		labelLower := strings.ToLower(item.Label)
		if strings.HasPrefix(labelLower, lower) {
			return i
		}
	}
	for i, item := range items {
		idLower := strings.ToLower(item.ID)
		if strings.HasPrefix(idLower, lower) {
			return i
		}
	}
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	ranks := fuzzy.RankFindNormalizedFold(trimmed, labels)
	if len(ranks) == 0 {
		if len(items) == 0 {
			return -1
		}
		return 0
	}
	best := ranks[0]
	for _, rank := range ranks[1:] {
		if rank.Distance < best.Distance {
			best = rank
			continue
		}
		if rank.Distance == best.Distance && rank.OriginalIndex < best.OriginalIndex {
			best = rank
		}
	}
	if best.OriginalIndex < 0 || best.OriginalIndex >= len(items) {
		if len(items) == 0 {
			return -1
		}
		return 0
	}
	return best.OriginalIndex
}

func (m *Model) filterPrompt() (string, *lipgloss.Style) {
	current := m.currentLevel()
	if current == nil {
		return ">", styles.Filter
	}
	cursor := styles.Cursor.Render(" ")
	text := current.filter
	prompt := styles.FilterPrompt.Render("» ")
	if text == "" {
		placeholderRunes := []rune("(type to search)")
		if len(placeholderRunes) == 0 {
			return prompt + cursor, nil
		}
		first := styles.Cursor.Render(string(placeholderRunes[0]))
		tail := styles.FilterPlaceholder.Render(string(placeholderRunes[1:]))
		return prompt + first + tail, nil
	}
	runes := []rune(text)
	pos := current.filterCursorPos()
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	before := string(runes[:pos])
	if pos < len(runes) {
		highlight := styles.Cursor.Render(string(runes[pos]))
		after := string(runes[pos+1:])
		return prompt + before + highlight + after, nil
	}
	return prompt + string(runes) + cursor, nil
}

func (m *Model) startSessionForm(prompt menu.SessionPrompt) {
	m.sessionForm = menu.NewSessionForm(prompt)
	m.mode = ModeSessionForm
}

func (m *Model) startWindowForm(prompt menu.WindowPrompt) {
	m.windowForm = menu.NewWindowRenameForm(prompt)
	m.mode = ModeWindowForm
}

func (m *Model) startPaneForm(prompt menu.PanePrompt) {
	m.paneForm = menu.NewPaneRenameForm(prompt)
	m.mode = ModePaneForm
}

func (m *Model) startWindowSwap(prompt menu.WindowSwapPrompt) {
	parent := m.currentLevel()
	label := prompt.First.Label
	for _, entry := range m.windows.Entries() {
		if entry.ID == prompt.First.ID {
			label = entry.Label
			break
		}
	}
	entries := m.windows.Entries()
	items := make([]menu.Item, 0, len(entries))
	for _, entry := range entries {
		if entry.ID == prompt.First.ID {
			continue
		}
		items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
	}
	if len(items) == 0 {
		m.setInfo("No windows available to swap with.")
		return
	}
	level := newLevel("window:swap-target", fmt.Sprintf("Swap %s with…", label), items, nil)
	if parent != nil {
		parent.lastCursor = parent.cursor
	}
	m.pendingWindowSwap = &menu.Item{ID: prompt.First.ID, Label: label}
	m.stack = append(m.stack, level)
}

func (m *Model) startPaneSwap(prompt menu.PaneSwapPrompt) {
	parent := m.currentLevel()
	label := prompt.First.Label
	for _, entry := range m.panes.Entries() {
		if entry.ID == prompt.First.ID {
			label = entry.Label
			break
		}
	}
	entries := m.panes.Entries()
	items := make([]menu.Item, 0, len(entries))
	for _, entry := range entries {
		if entry.ID == prompt.First.ID {
			continue
		}
		items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
	}
	if len(items) == 0 {
		m.setInfo("No panes available to swap with.")
		return
	}
	level := newLevel("pane:swap-target", fmt.Sprintf("Swap %s with…", label), items, nil)
	if parent != nil {
		parent.lastCursor = parent.cursor
	}
	m.pendingPaneSwap = &menu.Item{ID: prompt.First.ID, Label: label}
	m.stack = append(m.stack, level)
}

func (m *Model) viewPaneForm() string {
	return m.viewFormWithHeader(m.paneForm.Title(), m.paneForm.InputView(), m.paneForm.Help(), "")
}

func (m *Model) viewPaneFormWithHeader(header string) string {
	return m.viewFormWithHeader(m.paneForm.Title(), m.paneForm.InputView(), m.paneForm.Help(), header)
}

func (m *Model) viewWindowFormWithHeader(header string) string {
	return m.viewFormWithHeader(m.windowForm.Title(), m.windowForm.InputView(), m.windowForm.Help(), header)
}

func (m *Model) viewSessionFormWithHeader(header string) string {
	lines := []string{}
	if header != "" {
		lines = append(lines, header)
	}
	lines = append(lines, m.sessionForm.Title(), "", m.sessionForm.InputView())
	if err := m.sessionForm.Error(); err != "" {
		lines = append(lines, "", styles.Error.Render(err))
	}
	lines = append(lines, "", m.sessionForm.Help())
	return strings.Join(lines, "\n")
}

func (m *Model) viewFormWithHeader(title, input, help, header string) string {
	lines := []string{
		title,
		"",
		input,
		"",
		help,
	}
	if header != "" {
		lines = append([]string{header}, lines...)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) handlePaneForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.paneForm == nil {
		return false, nil
	}
	cmd, done, cancel := m.paneForm.Update(msg)
	if cancel {
		m.paneForm = nil
		m.mode = ModeMenu
		return true, cmd
	}
	if done {
		ctx := m.paneForm.Context()
		title := m.paneForm.Value()
		target := m.paneForm.Target()
		actionID := m.paneForm.ActionID()
		pendingLabel := m.paneForm.PendingLabel()
		m.paneForm = nil
		m.mode = ModeMenu
		m.loading = true
		m.pendingID = actionID
		m.pendingLabel = pendingLabel
		if cmd == nil {
			cmd = menu.PaneRenameCommand(ctx, target, title)
		}
		return true, cmd
	}
	if cmd != nil {
		return true, cmd
	}
	return true, nil
}

func (m *Model) handleWindowForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.windowForm == nil {
		return false, nil
	}
	cmd, done, cancel := m.windowForm.Update(msg)
	if cancel {
		m.windowForm = nil
		m.mode = ModeMenu
		return true, cmd
	}
	if done {
		ctx := m.windowForm.Context()
		name := m.windowForm.Value()
		target := m.windowForm.Target()
		actionID := m.windowForm.ActionID()
		pendingLabel := m.windowForm.PendingLabel()
		m.windowForm = nil
		m.mode = ModeMenu
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
		m.mode = ModeMenu
		return true, cmd
	}
	if done {
		ctx := m.sessionForm.Context()
		name := m.sessionForm.Value()
		target := m.sessionForm.Target()
		actionID := m.sessionForm.ActionID()
		pendingLabel := m.sessionForm.PendingLabel()
		m.sessionForm = nil
		m.mode = ModeMenu
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
