package ui

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/ui/command"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) handleEscapeKey() tea.Cmd {
	current := m.currentLevel()
	if current == nil {
		return tea.Quit
	}
	if len(m.stack) <= 1 {
		return tea.Quit
	}
	if current.ID == "window:swap-target" {
		m.pendingWindowSwap = nil
	}
	if current.ID == "pane:swap-target" {
		m.pendingPaneSwap = nil
	}
	parent := m.stack[len(m.stack)-2]
	m.stack = m.stack[:len(m.stack)-1]
	if parent != nil {
		if parent.LastCursor >= 0 && parent.LastCursor < len(parent.Items) {
			parent.Cursor = parent.LastCursor
		} else if idx := parent.IndexOf(current.ID); idx >= 0 {
			parent.Cursor = idx
		} else if len(parent.Items) > 0 {
			parent.Cursor = len(parent.Items) - 1
		}
		parent.LastCursor = -1
		m.syncViewport(parent)
	}
	m.errMsg = ""
	m.forceClearInfo()
	return nil
}

func (m *Model) handleEnterKey() tea.Cmd {
	if m.loading {
		return nil
	}
	current := m.currentLevel()
	if current == nil || len(current.Items) == 0 {
		return nil
	}
	ctx := m.menuContext()
	item := current.Items[current.Cursor]
	events.UI.MenuEnter(current.ID, item.ID, item.Label, current.Filter)
	beforeCursor := current.FilterCursorPos()
	current.SetFilter("", 0)
	m.noteFilterCursorChange(current, beforeCursor)
	if current.ID == "window:swap-target" && m.pendingWindowSwap != nil {
		first := *m.pendingWindowSwap
		m.pendingWindowSwap = nil
		m.stack = m.stack[:len(m.stack)-1]
		m.loading = true
		m.pendingID = "window:swap"
		m.pendingLabel = fmt.Sprintf("%s ↔ %s", first.Label, item.Label)
		m.errMsg = ""
		m.forceClearInfo()
		return menu.WindowSwapCommand(ctx, first.ID, item.ID, first.Label, item.Label)
	}
	if current.ID == "pane:swap-target" && m.pendingPaneSwap != nil {
		first := *m.pendingPaneSwap
		m.pendingPaneSwap = nil
		m.stack = m.stack[:len(m.stack)-1]
		m.loading = true
		m.pendingID = "pane:swap"
		m.pendingLabel = fmt.Sprintf("%s ↔ %s", first.Label, item.Label)
		m.errMsg = ""
		m.forceClearInfo()
		return menu.PaneSwapCommand(ctx, first, item)
	}
	node := current.Node
	if node == nil {
		node, _ = m.registry.Find(current.ID)
	}
	if current.MultiSelect {
		if selected := current.SelectedItems(); len(selected) > 0 {
			ids := make([]string, 0, len(selected))
			labels := make([]string, 0, len(selected))
			for _, sel := range selected {
				ids = append(ids, sel.ID)
				labels = append(labels, sel.Label)
			}
			item = menu.Item{ID: strings.Join(ids, "\n"), Label: strings.Join(labels, ", ")}
			current.ClearSelection()
		}
	}
	if node != nil {
		if child, ok := node.Children[item.ID]; ok {
			if child.Loader != nil {
				current.LastCursor = current.Cursor
				m.loading = true
				m.pendingID = child.ID
				m.pendingLabel = item.Label
				m.errMsg = ""
				m.forceClearInfo()
				return m.loadMenuCmd(child.ID, item.Label, child.Loader)
			}
			if child.Action != nil {
				m.loading = true
				m.pendingID = child.ID
				m.pendingLabel = item.Label
				m.errMsg = ""
				m.forceClearInfo()
				return m.bus.Execute(ctx, command.Request{ID: child.ID, Label: item.Label, Handler: child.Action, Item: item})
			}
		}
		if node.Action != nil {
			m.loading = true
			m.pendingID = node.ID
			m.pendingLabel = item.Label
			m.errMsg = ""
			m.forceClearInfo()
			return m.bus.Execute(ctx, command.Request{ID: node.ID, Label: item.Label, Handler: node.Action, Item: item})
		}
	}
	m.setInfo(fmt.Sprintf("Selected %s (no action defined yet)", item.Label))
	return nil
}

func (m *Model) moveCursorUp() {
	if current := m.currentLevel(); current != nil {
		if n := len(current.Items); n > 0 {
			if current.Cursor > 0 {
				current.Cursor--
			} else {
				current.Cursor = n - 1
			}
			events.UI.MenuCursor(current.ID, current.Cursor)
			m.syncViewport(current)
		}
	}
}

func (m *Model) moveCursorDown() {
	if current := m.currentLevel(); current != nil {
		if n := len(current.Items); n > 0 {
			if current.Cursor < n-1 {
				current.Cursor++
			} else {
				current.Cursor = 0
			}
			events.UI.MenuCursor(current.ID, current.Cursor)
			m.syncViewport(current)
		}
	}
}

func (m *Model) moveCursorPageUp() {
	if current := m.currentLevel(); current != nil {
		if moved := current.MoveCursorPageUp(m.maxVisibleItems()); moved {
			events.UI.MenuCursor(current.ID, current.Cursor)
		}
		m.syncViewport(current)
	}
}

func (m *Model) moveCursorPageDown() {
	if current := m.currentLevel(); current != nil {
		if moved := current.MoveCursorPageDown(m.maxVisibleItems()); moved {
			events.UI.MenuCursor(current.ID, current.Cursor)
		}
		m.syncViewport(current)
	}
}

func (m *Model) moveCursorHome() {
	if current := m.currentLevel(); current != nil {
		if moved := current.MoveCursorHome(); moved {
			events.UI.MenuCursor(current.ID, current.Cursor)
		}
		m.syncViewport(current)
	}
}

func (m *Model) moveCursorEnd() {
	if current := m.currentLevel(); current != nil {
		if moved := current.MoveCursorEnd(); moved {
			events.UI.MenuCursor(current.ID, current.Cursor)
		}
		m.syncViewport(current)
	}
}

func (m *Model) syncViewport(l *level) {
	if l == nil {
		return
	}
	l.EnsureCursorVisible(m.maxVisibleItems())
}

func (m *Model) handleKeyMsg(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}
	if m.mode != ModeMenu {
		return nil
	}
	if keyMsg.Type == tea.KeyTab {
		if current := m.currentLevel(); current != nil && current.MultiSelect {
			current.ToggleCurrentSelection()
		}
		return nil
	}
	if m.handleTextInput(keyMsg) {
		return nil
	}
	switch keyMsg.String() {
	case "ctrl+c", "q":
		return tea.Quit
	case "esc":
		return m.handleEscapeKey()
	case "enter":
		return m.handleEnterKey()
	case "up":
		m.moveCursorUp()
	case "down":
		m.moveCursorDown()
	case "pgup":
		m.moveCursorPageUp()
	case "pgdown":
		m.moveCursorPageDown()
	case "home":
		m.moveCursorHome()
	case "end":
		m.moveCursorEnd()
	}
	return nil
}

func (m *Model) handleCategoryLoadedMsg(msg tea.Msg) tea.Cmd {
	update, ok := msg.(categoryLoadedMsg)
	if !ok {
		return nil
	}
	if update.id != m.pendingID {
		return nil
	}
	m.loading = false
	m.pendingID = ""
	m.pendingLabel = ""
	if update.err != nil {
		m.errMsg = update.err.Error()
		return nil
	}
	m.errMsg = ""
	node, _ := m.registry.Find(update.id)
	level := newLevel(update.id, update.title, update.items, node)
	m.applyNodeSettings(level)
	m.syncViewport(level)
	m.stack = append(m.stack, level)
	if len(level.Items) == 0 {
		m.setInfo("No entries found.")
	} else if m.infoMsg != "" {
		m.clearInfo()
	}
	return nil
}

func (m *Model) applyNodeSettings(l *level) {
	if l == nil {
		return
	}
	if l.Node == nil {
		if node, ok := m.registry.Find(l.ID); ok {
			l.Node = node
		}
	}
	if l.Node != nil {
		l.MultiSelect = l.Node.MultiSelect
	}
}

func (m *Model) findLevelByID(id string) *level {
	for _, lvl := range m.stack {
		if lvl.ID == id {
			m.applyNodeSettings(lvl)
			return lvl
		}
	}
	return nil
}

func (m *Model) applyRootMenuOverride(requested string) {
	trimmed := strings.TrimSpace(requested)
	if trimmed == "" {
		m.rootMenuID = ""
		m.rootTitle = defaultRootTitle
		return
	}
	if m.registry == nil {
		return
	}
	id := strings.ToLower(trimmed)
	node, ok := m.registry.Find(id)
	if !ok {
		m.errMsg = fmt.Sprintf("Unknown root menu %q", trimmed)
		m.rootMenuID = ""
		m.rootTitle = defaultRootTitle
		return
	}

	items := []menu.Item(nil)
	if node.Loader != nil {
		loaded, err := node.Loader(m.menuContext())
		if err != nil {
			logging.Error(err)
			m.errMsg = fmt.Sprintf("Failed to load %s menu: %v", id, err)
		} else {
			items = loaded
			m.errMsg = ""
		}
	} else {
		m.errMsg = ""
	}

	title := headerSegmentCleaner.Replace(node.ID)
	title = strings.TrimSpace(title)
	root := newLevel(node.ID, title, items, node)
	m.applyNodeSettings(root)
	m.syncViewport(root)
	m.stack = []*level{root}
	m.rootMenuID = node.ID

	segment := headerSegmentForLevel(root)
	if segment == "" {
		segment = title
	}
	if segment == "" {
		segment = node.ID
	}
	m.rootTitle = segment
}

func (m *Model) currentLevel() *level {
	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
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
		parent.LastCursor = parent.Cursor
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
		parent.LastCursor = parent.Cursor
	}
	m.pendingPaneSwap = &menu.Item{ID: prompt.First.ID, Label: label}
	m.stack = append(m.stack, level)
}
