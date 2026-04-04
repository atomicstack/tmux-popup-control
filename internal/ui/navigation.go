package ui

import (
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
	"github.com/atomicstack/tmux-popup-control/internal/ui/command"
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

	// Revert layout preview on escape.
	var revertCmd tea.Cmd
	if current.ID == "window:layout" {
		if original, ok := current.Data.(string); ok && original != "" {
			socket := m.socketPath
			revertCmd = func() tea.Msg {
				err := layoutPreviewFn(socket, original)
				return layoutAppliedMsg{levelID: "window:layout", err: err}
			}
		}
	}

	m.clearPreview(current.ID)
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
	parentPreviewCmd := m.ensurePreviewForLevel(parent)
	if revertCmd != nil && parentPreviewCmd != nil {
		return tea.Batch(revertCmd, parentPreviewCmd)
	}
	if revertCmd != nil {
		return revertCmd
	}
	return parentPreviewCmd
}

func (m *Model) handleEnterKey() tea.Cmd {
	if m.loading {
		return nil
	}
	current := m.currentLevel()
	if current != nil && current.Node != nil && current.Node.FilterCommand {
		filterText := strings.TrimSpace(current.Filter)
		if filterText == "" {
			return nil
		}
		beforeCursor := current.FilterCursorPos()
		current.SetFilter("", 0)
		m.noteFilterCursorChange(current, beforeCursor)
		m.loading = true
		m.pendingID = "command"
		m.pendingLabel = filterText
		m.errMsg = ""
		m.forceClearInfo()
		target := m.sessionName
		if target == "" {
			target = m.sessions.Current()
		}
		return menu.RunCommand(m.socketPath, filterText, target)
	}
	if current == nil || len(current.Items) == 0 {
		return nil
	}
	ctx := m.menuContext()
	item := current.Items[current.Cursor]
	if item.Header {
		return nil
	}
	events.UI.MenuEnter(current.ID, item.ID, item.Label, current.Filter)
	// In pull-from-session tree, Enter on session nodes toggles
	// expand/collapse instead of dispatching the action.
	if current.ID == "window:pull-from-session" && strings.HasPrefix(item.ID, menu.TreePrefixSession) {
		if ts, ok := current.Data.(*menu.TreeState); ok && ts != nil {
			ts.Toggle(item.ID)
			m.rebuildTreeItems(current, ts)
		}
		return nil
	}
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
				if child.FilterCommand && m.commandItemsCache != nil {
					current.LastCursor = current.Cursor
					m.errMsg = ""
					m.forceClearInfo()
					lvl := newLevel(child.ID, item.Label, m.commandItemsCache, child)
					m.applyNodeSettings(lvl)
					m.syncViewport(lvl)
					m.stack = append(m.stack, lvl)
					return nil
				}
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

func (m *Model) moveCursorUp() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if n := len(current.Items); n > 0 {
		old := current.Cursor
		if current.Cursor > 0 {
			current.Cursor--
			current.SkipHeaders(-1)
			if current.Cursor == old {
				// couldn't move up (hit headers); wrap to end
				current.Cursor = n - 1
				current.SkipHeaders(-1)
			}
		} else {
			current.Cursor = n - 1
			current.SkipHeaders(-1)
		}
		if old != current.Cursor {
			events.UI.MenuCursor(current.ID, current.Cursor)
			m.syncViewport(current)
			return true
		}
	}
	return false
}

func (m *Model) moveCursorDown() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if n := len(current.Items); n > 0 {
		old := current.Cursor
		if current.Cursor < n-1 {
			current.Cursor++
		} else {
			current.Cursor = 0
		}
		current.SkipHeaders(1)
		if old != current.Cursor {
			events.UI.MenuCursor(current.ID, current.Cursor)
			m.syncViewport(current)
			return true
		}
	}
	return false
}

func (m *Model) moveCursorPageUp() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if moved := current.MoveCursorPageUp(m.maxVisibleItems()); moved {
		events.UI.MenuCursor(current.ID, current.Cursor)
		m.syncViewport(current)
		return true
	}
	m.syncViewport(current)
	return false
}

func (m *Model) moveCursorPageDown() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if moved := current.MoveCursorPageDown(m.maxVisibleItems()); moved {
		events.UI.MenuCursor(current.ID, current.Cursor)
		m.syncViewport(current)
		return true
	}
	m.syncViewport(current)
	return false
}

func (m *Model) moveCursorHome() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if moved := current.MoveCursorHome(); moved {
		events.UI.MenuCursor(current.ID, current.Cursor)
		m.syncViewport(current)
		return true
	}
	m.syncViewport(current)
	return false
}

func (m *Model) moveCursorEnd() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if moved := current.MoveCursorEnd(); moved {
		events.UI.MenuCursor(current.ID, current.Cursor)
		m.syncViewport(current)
		return true
	}
	m.syncViewport(current)
	return false
}

func (m *Model) syncViewport(l *level) {
	if l == nil {
		return
	}
	l.EnsureCursorVisible(m.maxVisibleItems())
}

func (m *Model) syncFilterViewport(l *level) {
	if l == nil {
		return
	}
	maxVisible := m.maxVisibleItems()
	if maxVisible <= 0 {
		l.EnsureCursorVisible(maxVisible)
		return
	}
	anchorRow := (maxVisible - 1) / 3
	l.EnsureCursorVisibleWithAnchor(maxVisible, anchorRow)
}

func (m *Model) handleKeyMsg(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return nil
	}
	if m.mode != ModeMenu {
		return nil
	}
	if m.completionVisible() {
		switch keyMsg.String() {
		case "up":
			m.completion.moveUp()
			return nil
		case "down":
			m.completion.moveDown()
			return nil
		case "tab":
			return m.acceptCompletion()
		case "esc":
			m.dismissCompletionUntilInputChanges()
			return nil
		case "enter":
			return m.acceptCompletion()
		}
	}
	if keyMsg.String() == "tab" {
		if current := m.currentLevel(); current != nil {
			if current.MultiSelect {
				current.ToggleCurrentSelection()
				// Sync "all" sentinel: toggling "all" on selects everything,
				// toggling "all" off clears everything, deselecting any item
				// while "all" is active deselects "all".
				if current.Cursor >= 0 && current.Cursor < len(current.Items) {
					item := current.Items[current.Cursor]
					if item.ID == menu.AllPluginsSentinel {
						if current.IsSelected(item.ID) {
							for _, it := range current.Items {
								if !current.IsSelected(it.ID) {
									current.ToggleSelection(it.ID)
								}
							}
						} else {
							current.ClearSelection()
						}
					} else if !current.IsSelected(item.ID) && current.IsSelected(menu.AllPluginsSentinel) {
						current.ToggleSelection(menu.AllPluginsSentinel)
					}
				}
				return nil
			}
			if current.Node != nil && current.Node.FilterCommand {
				if m.replaceCommandTokenUnderCursor() {
					return nil
				}
			}
			if ghost := m.autoCompleteGhost(); ghost != "" {
				before := current.FilterCursorPos()
				current.SetFilter(current.Filter+ghost, len([]rune(current.Filter+ghost)))
				m.noteFilterCursorChange(current, before)
				m.syncFilterViewport(current)
				return nil
			}
		}
		return nil
	}
	// Handle quit and escape before filter input so they always work.
	switch keyMsg.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		return m.handleEscapeKey()
	}
	if handled, cmd := m.handleTextInput(keyMsg); handled {
		return cmd
	}
	// Tree-level left/right for expand/collapse.
	if current := m.currentLevel(); current != nil && isTreeLevel(current.ID) {
		if ts, ok := current.Data.(*menu.TreeState); ok && ts != nil {
			switch keyMsg.String() {
			case "left":
				return m.treeCollapse(current, ts)
			case "right":
				return m.treeExpand(current, ts)
			}
		}
	}
	var previewCmd tea.Cmd
	switch keyMsg.String() {
	case "enter":
		return m.handleEnterKey()
	case "up":
		if m.moveCursorUp() {
			previewCmd = m.ensurePreviewForCurrentLevel()
		}
	case "down":
		if m.moveCursorDown() {
			previewCmd = m.ensurePreviewForCurrentLevel()
		}
	case "pgup":
		if m.moveCursorPageUp() {
			previewCmd = m.ensurePreviewForCurrentLevel()
		}
	case "pgdown":
		if m.moveCursorPageDown() {
			previewCmd = m.ensurePreviewForCurrentLevel()
		}
	case "home":
		if m.moveCursorHome() {
			previewCmd = m.ensurePreviewForCurrentLevel()
		}
	case "end":
		if m.moveCursorEnd() {
			previewCmd = m.ensurePreviewForCurrentLevel()
		}
	}
	return previewCmd
}

func (m *Model) replaceCommandTokenUnderCursor() bool {
	current := m.currentLevel()
	if current == nil || current.Node == nil || !current.Node.FilterCommand {
		return false
	}
	if current.Cursor < 0 || current.Cursor >= len(current.Items) {
		return false
	}

	filterRunes := []rune(current.Filter)
	if len(filterRunes) == 0 {
		replacement := current.Items[current.Cursor].ID
		before := current.FilterCursorPos()
		current.SetFilter(replacement, len([]rune(replacement)))
		m.noteFilterCursorChange(current, before)
		m.clearCompletionSuppression()
		m.triggerCompletion()
		m.syncFilterViewport(current)
		return true
	}

	pos := current.FilterCursorPos()
	if pos > len(filterRunes) {
		pos = len(filterRunes)
	}
	if pos > 0 && (pos == len(filterRunes) || unicode.IsSpace(filterRunes[pos])) && !unicode.IsSpace(filterRunes[pos-1]) {
		pos--
	}
	if pos >= len(filterRunes) || unicode.IsSpace(filterRunes[pos]) {
		return false
	}

	start := pos
	for start > 0 && !unicode.IsSpace(filterRunes[start-1]) {
		start--
	}
	if start != 0 {
		return false
	}
	end := pos + 1
	for end < len(filterRunes) && !unicode.IsSpace(filterRunes[end]) {
		end++
	}

	replacement := []rune(current.Items[current.Cursor].ID)
	updated := make([]rune, 0, len(filterRunes)-((end-start))+len(replacement))
	updated = append(updated, filterRunes[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, filterRunes[end:]...)

	before := current.FilterCursorPos()
	current.SetFilter(string(updated), start+len(replacement))
	m.noteFilterCursorChange(current, before)
	m.clearCompletionSuppression()
	m.triggerCompletion()
	m.syncFilterViewport(current)
	return true
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
	if update.id == "session:restore-from" {
		if dir, err := resurrect.ResolveDir(m.socketPath); err == nil {
			level.Subtitle = dir
		}
	}
	if update.id == "session:tree" {
		allExpanded := strings.TrimSpace(m.menuArgs) == "expanded"
		level.Data = menu.NewTreeState(allExpanded)
		level.Cursor = 0
		m.treeSessions = m.sessions.Entries()
		m.treeWindows = m.windows.Entries()
		m.treePanes = m.panes.Entries()
	}
	if update.id == "window:pull-from-session" {
		level.Data = menu.NewTreeState(false)
		level.Cursor = 0
		m.populatePullTreeData()
	}
	m.applyNodeSettings(level)
	m.syncViewport(level)
	m.stack = append(m.stack, level)
	cmd := m.ensurePreviewForLevel(level)
	if len(level.Items) == 0 {
		m.setInfo("No entries found.")
	} else if m.infoMsg != "" {
		m.clearInfo()
	}
	return cmd
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

	// If the node is a rename action (session:rename or window:rename) and
	// menuArgs provides a target, defer the rename form until backend data
	// is available. The form needs session/window entries for duplicate
	// name validation and for resolving the initial value.
	if m.menuArgs != "" && (id == "session:rename" || id == "window:rename") {
		m.loading = true
		m.pendingID = node.ID
		m.deferredRename = node
		m.rootMenuID = node.ID
		title := node.ID
		if idx := strings.LastIndex(title, ":"); idx >= 0 {
			title = title[:idx]
		}
		m.rootTitle = headerSegmentCleaner.Replace(title)
		return
	}

	// If the node is a leaf action with no loader, defer it until backend
	// data is available. Executing immediately would use an empty context
	// (e.g. CurrentPaneID not yet populated by the backend poller).
	if node.Loader == nil && node.Action != nil {
		m.loading = true
		m.pendingID = node.ID
		m.deferredAction = node
		m.rootMenuID = node.ID
		title := node.ID
		if idx := strings.LastIndex(title, ":"); idx >= 0 {
			title = title[:idx]
		}
		m.rootTitle = headerSegmentCleaner.Replace(title)
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
			if node.FilterCommand {
				m.commandItemsCache = items
				labels := make([]string, 0, len(items))
				for _, item := range items {
					labels = append(labels, item.Label)
				}
				m.commandSchemas = cmdparse.BuildRegistry(labels)
			}
		}
	} else {
		m.errMsg = ""
	}

	title := headerSegmentCleaner.Replace(node.ID)
	title = strings.TrimSpace(title)
	root := newLevel(node.ID, title, items, node)
	if node.ID == "session:tree" {
		allExpanded := strings.TrimSpace(m.menuArgs) == "expanded"
		root.Data = menu.NewTreeState(allExpanded)
		root.Cursor = 0
		m.treeSessions = m.sessions.Entries()
		m.treeWindows = m.windows.Entries()
		m.treePanes = m.panes.Entries()
	}
	if node.ID == "window:pull-from-session" {
		root.Data = menu.NewTreeState(false)
		root.Cursor = 0
		m.populatePullTreeData()
	}
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
