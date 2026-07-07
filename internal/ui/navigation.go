package ui

import (
	"cmp"
	"fmt"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
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
	if current.ID == "resurrect:restore-from" {
		m.stopRestoreRefresh()
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
		m.kickPreviewBlinkOnFilterChange(current, beforeCursor)
		m.loading = true
		m.pendingID = "command"
		m.pendingLabel = filterText
		m.errMsg = ""
		m.forceClearInfo()
		return menu.RunCommand(m.socketPath, filterText)
	}
	if current == nil || len(current.Items) == 0 {
		return nil
	}
	if current.ID == extractLevelID {
		// Extract reads its own selection (marked items or cursor), unlike
		// the generic multi-select join below, so intercept before the
		// child/action dispatch.
		return m.extractInsert()
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
	if current.ID == deleteSavedLevelID {
		m.startDeleteConfirm(item)
		return nil
	}
	beforeCursor := current.FilterCursorPos()
	current.SetFilter("", 0)
	m.kickPreviewBlinkOnFilterChange(current, beforeCursor)
	if current.ID == "window:swap-target" && m.pendingWindowSwap != nil {
		first := *m.pendingWindowSwap
		m.pendingWindowSwap = nil
		m.stack = m.stack[:len(m.stack)-1]
		m.loading = true
		m.pendingID = "window:swap"
		m.pendingLabel = fmt.Sprintf("%s ↔ %s", first.Label, item.Label)
		m.errMsg = ""
		m.forceClearInfo()
		return menu.WindowSwapCommand(ctx, first, item)
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
				if child.ID == extractLevelID {
					// Reset the active category before the loader is
					// dispatched so the async load reads the reset value,
					// not a stale category from a previous visit. Also bump
					// extractSeq so any ctrl-f reload still in flight from a
					// prior visit is invalidated (see handleExtractReloadMsg).
					m.extractCategory = extract.DefaultCategory
					m.extractSeq++
				}
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

// moveCursor runs the supplied Level cursor-movement function against the
// current level, emitting the cursor event and syncing the viewport when the
// cursor moves. When the cursor does not move, the viewport is re-synced only if
// syncOnNoMove is set (the Home/End/Page handlers do; Up/Down do not). Returns
// whether the cursor moved.
func (m *Model) moveCursor(move func(*level) bool, syncOnNoMove bool) bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if move(current) {
		events.UI.MenuCursor(current.ID, current.Cursor)
		m.syncViewport(current)
		return true
	}
	if syncOnNoMove {
		m.syncViewport(current)
	}
	return false
}

func (m *Model) moveCursorUp() bool {
	return m.moveCursor(func(l *level) bool { return l.MoveCursorUp() }, false)
}

func (m *Model) moveCursorDown() bool {
	return m.moveCursor(func(l *level) bool { return l.MoveCursorDown() }, false)
}

func (m *Model) moveCursorPageUp() bool {
	maxVisible := m.maxVisibleItems()
	return m.moveCursor(func(l *level) bool { return l.MoveCursorPageUp(maxVisible) }, true)
}

func (m *Model) moveCursorPageDown() bool {
	maxVisible := m.maxVisibleItems()
	return m.moveCursor(func(l *level) bool { return l.MoveCursorPageDown(maxVisible) }, true)
}

func (m *Model) moveCursorHome() bool {
	return m.moveCursor(func(l *level) bool { return l.MoveCursorHome() }, true)
}

func (m *Model) moveCursorEnd() bool {
	return m.moveCursor(func(l *level) bool { return l.MoveCursorEnd() }, true)
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
	if m.mode == ModeCommandOutput {
		return m.handleCommandOutputKey(keyMsg)
	}
	if m.mode != ModeMenu {
		return nil
	}
	if m.confirmState != nil {
		return m.handleDeleteConfirmKey(keyMsg)
	}
	if m.completionVisible() {
		switch keyMsg.String() {
		case "up":
			m.completion.moveUp()
			return nil
		case "down":
			m.completion.moveDown()
			return nil
		case "pgup":
			m.completion.movePageUp()
			return nil
		case "pgdown":
			m.completion.movePageDown()
			return nil
		case "tab":
			return m.acceptCompletion()
		case "esc":
			m.dismissCompletionUntilInputChanges()
			return nil
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
							current.SelectAll()
						} else {
							current.ClearAll()
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
				m.kickPreviewBlinkOnFilterChange(current, before)
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
	// Extract level: ctrl-f cycles the token category in place, ctrl-y copies
	// the selected token(s) to the tmux paste buffer.
	if current := m.currentLevel(); current != nil && current.ID == extractLevelID {
		switch keyMsg.String() {
		case "ctrl+f":
			return m.extractCycleCmd()
		case "ctrl+y":
			return m.extractCopy()
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
		m.kickPreviewBlinkOnFilterChange(current, before)
		m.clearCompletionSuppression()
		m.triggerCompletion()
		m.syncFilterViewport(current)
		return true
	}

	pos := min(current.FilterCursorPos(), len(filterRunes))
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
	updated := make([]rune, 0, len(filterRunes)-(end-start)+len(replacement))
	updated = append(updated, filterRunes[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, filterRunes[end:]...)

	before := current.FilterCursorPos()
	current.SetFilter(string(updated), start+len(replacement))
	m.kickPreviewBlinkOnFilterChange(current, before)
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
	if update.id == "resurrect:restore-from" {
		level.Subtitle = restoreRefreshDir(m.socketPath)
	}
	if update.id == "session:tree" {
		allExpanded := strings.TrimSpace(m.menuArgs) == "expanded"
		level.Data = menu.NewTreeState(allExpanded)
		m.treeSessions = m.sessions.Entries()
		m.treeWindows = m.windows.Entries()
		m.treePanes = m.panes.Entries()
		level.Cursor = m.initialSessionTreeCursor(level.Items)
	}
	if update.id == "window:pull-from-session" {
		level.Data = menu.NewTreeState(false)
		level.Cursor = 0
		m.populatePullTreeData()
	}
	if update.id == extractLevelID {
		// Category was already reset to DefaultCategory when navigation into
		// extract was initiated (handleEnterKey / applyRootMenuOverride),
		// before this loader ran. Only the header needs (re)rendering here.
		level.Subtitle = extractSubtitle(m.extractCategory)
	}
	m.applyNodeSettings(level)
	m.syncViewport(level)
	m.stack = append(m.stack, level)
	cmd := m.ensurePreviewForLevel(level)
	if update.id == "resurrect:restore-from" {
		cmd = tea.Batch(cmd, m.startRestoreRefreshIfNeeded())
	}
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

	if node.ID == extractLevelID {
		// Reset the active category before the loader runs (synchronously,
		// here) so it reads the reset value rather than a stale category
		// from a previous visit. Also bump extractSeq so any ctrl-f reload
		// still in flight from a prior visit is invalidated (see
		// handleExtractReloadMsg).
		m.extractCategory = extract.DefaultCategory
		m.extractSeq++
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
	if node.ID == "resurrect:restore-from" {
		root.Subtitle = restoreRefreshDir(m.socketPath)
	}
	if node.ID == "session:tree" {
		allExpanded := strings.TrimSpace(m.menuArgs) == "expanded"
		root.Data = menu.NewTreeState(allExpanded)
		m.treeSessions = m.sessions.Entries()
		m.treeWindows = m.windows.Entries()
		m.treePanes = m.panes.Entries()
		root.Cursor = m.initialSessionTreeCursor(root.Items)
	}
	if node.ID == "window:pull-from-session" {
		root.Data = menu.NewTreeState(false)
		root.Cursor = 0
		m.populatePullTreeData()
	}
	if node.ID == extractLevelID {
		// Category was already reset above, before the loader ran. Only the
		// header needs (re)rendering here.
		root.Subtitle = extractSubtitle(m.extractCategory)
	}
	m.applyNodeSettings(root)
	m.syncViewport(root)
	m.stack = []*level{root}
	m.rootMenuID = node.ID

	m.rootTitle = cmp.Or(headerSegmentForLevel(root), title, node.ID)
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
