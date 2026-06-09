package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/ui/command"
)

func waitForBackendEvent(w *backend.Watcher) tea.Cmd {
	return func() tea.Msg {
		evt, ok := <-w.Events()
		if !ok {
			return backendDoneMsg{}
		}
		return backendEventMsg{event: evt}
	}
}

type backendEventMsg struct {
	event backend.Event
}

type backendDoneMsg struct{}

func (m *Model) handleBackendEventMsg(msg tea.Msg) tea.Cmd {
	eventMsg, ok := msg.(backendEventMsg)
	if !ok {
		return nil
	}
	cmd := m.applyBackendEvent(eventMsg.event)
	if m.backend != nil {
		waitCmd := waitForBackendEvent(m.backend)
		if cmd != nil {
			return tea.Batch(cmd, waitCmd)
		}
		return waitCmd
	}
	return cmd
}

func (m *Model) handleBackendDoneMsg(msg tea.Msg) tea.Cmd {
	m.backend = nil
	return nil
}

// levelUpdate pairs a menu level ID with the function that produces its items
// from a menu.Context. Used to table-drive the find-update-sync cases in
// applyBackendEvent; an ordered slice keeps side-effect determinism (cursor /
// viewport sync order matters).
type levelUpdate struct {
	id      string
	itemsFn func(menu.Context) []menu.Item
}

// applySimpleLevelUpdates runs the standard find-update-sync sequence for each
// update whose level is currently present in the stack.
func (m *Model) applySimpleLevelUpdates(ctx menu.Context, updates []levelUpdate) {
	for _, u := range updates {
		if lvl := m.findLevelByID(u.id); lvl != nil {
			lvl.UpdateItems(u.itemsFn(ctx))
			m.syncViewport(lvl)
		}
	}
}

// rebuildPullTree re-populates and rebuilds the window:pull-from-session tree
// level, if present.
func (m *Model) rebuildPullTree() {
	lvl := m.findLevelByID("window:pull-from-session")
	if lvl == nil {
		return
	}
	m.populatePullTreeData()
	if ts, ok := lvl.Data.(*menu.TreeState); ok && ts != nil {
		m.rebuildTreeItems(lvl, ts)
	}
}

// applyDeferredSessionRename handles a pending session:rename triggered via
// direct invocation, once session data has arrived. Returns a focus command to
// batch into the caller's previewCmd, or nil.
func (m *Model) applyDeferredSessionRename(ctx menu.Context) tea.Cmd {
	if m.deferredRename == nil || m.deferredRename.ID != "session:rename" {
		return nil
	}
	m.deferredRename = nil
	target := strings.TrimSpace(m.menuArgs)
	return m.withPrompt(func() promptResult {
		return promptResult{Cmd: m.startSessionForm(menu.SessionPrompt{
			Context: ctx,
			Action:  "session:rename",
			Target:  target,
			Initial: target,
		})}
	})
}

// applyDeferredWindowRename handles a pending window:rename triggered via direct
// invocation, once window data has arrived. Returns a focus command to batch
// into the caller's previewCmd, or nil.
func (m *Model) applyDeferredWindowRename(ctx menu.Context) tea.Cmd {
	if m.deferredRename == nil || m.deferredRename.ID != "window:rename" {
		return nil
	}
	m.deferredRename = nil
	target := strings.TrimSpace(m.menuArgs)
	initial := target
	for _, entry := range ctx.Windows {
		if entry.ID == target {
			if entry.Name != "" {
				initial = entry.Name
			}
			break
		}
	}
	return m.withPrompt(func() promptResult {
		return promptResult{Cmd: m.startWindowForm(menu.WindowPrompt{
			Context: ctx,
			Target:  target,
			Initial: initial,
		})}
	})
}

func (m *Model) applyBackendEvent(evt backend.Event) tea.Cmd {
	if m.backendState == nil {
		m.backendState = make(map[backend.Kind]error)
	}
	m.backendState[evt.Kind] = evt.Err
	if evt.Err != nil {
		m.backendLastErr = evt.Err.Error()
		return nil
	}

	res := m.dispatcher.Handle(evt)
	ctx := m.menuContext()
	var previewCmd tea.Cmd

	currentLvl := m.currentLevel()

	if res.SessionsUpdated {
		if lvl := m.findLevelByID("session:switch"); lvl != nil {
			lvl.UpdateItems(sessionSwitchItems(ctx))
			if len(lvl.Items) > 0 {
				m.clearInfo()
			}
			m.syncViewport(lvl)
		}
		m.applySimpleLevelUpdates(ctx, []levelUpdate{
			{"session:rename", func(c menu.Context) []menu.Item { return menu.SessionRenameItems(c.Sessions) }},
			{"session:detach", func(c menu.Context) []menu.Item { return menu.SessionEntriesToItems(c.Sessions) }},
			{"session:kill", func(c menu.Context) []menu.Item { return menu.SessionEntriesToItems(c.Sessions) }},
			{"window:push-to-session", windowPushToSessionItems},
		})
		m.rebuildPullTree()
		if m.sessionForm != nil {
			m.sessionForm.SetSessions(ctx.Sessions)
		}
		if focusCmd := m.applyDeferredSessionRename(ctx); focusCmd != nil {
			previewCmd = tea.Batch(previewCmd, focusCmd)
		}
		if currentLvl != nil && currentLvl.ID == "session:switch" {
			previewCmd = m.refreshPreviewForLevel(currentLvl)
		}
	}

	if res.WindowsUpdated {
		m.pendingWindowSwap = nil
		m.applySimpleLevelUpdates(ctx, []levelUpdate{
			{"window:switch", menu.WindowSwitchItems},
		})
		m.rebuildPullTree()
		m.applySimpleLevelUpdates(ctx, []levelUpdate{
			{"window:swap", windowSwapItems},
		})
		if lvl := m.findLevelByID("window:kill"); lvl != nil {
			lvl.UpdateItems(windowSwapItems(ctx))
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		if focusCmd := m.applyDeferredWindowRename(ctx); focusCmd != nil {
			previewCmd = tea.Batch(previewCmd, focusCmd)
		}
		if currentLvl != nil && currentLvl.ID == "window:switch" {
			previewCmd = m.refreshPreviewForLevel(currentLvl)
		}
	}

	if res.PanesUpdated {
		m.pendingPaneSwap = nil
		m.applySimpleLevelUpdates(ctx, []levelUpdate{
			{"pane:switch", paneSwitchItems},
			{"pane:break", paneBreakItems},
		})
		if lvl := m.findLevelByID("pane:join"); lvl != nil {
			lvl.UpdateItems(paneJoinItems(ctx))
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		m.applySimpleLevelUpdates(ctx, []levelUpdate{
			{"pane:swap", paneSwapItems},
		})
		if lvl := m.findLevelByID("pane:kill"); lvl != nil {
			lvl.UpdateItems(paneKillItems(ctx))
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		m.applySimpleLevelUpdates(ctx, []levelUpdate{
			{"pane:rename", func(c menu.Context) []menu.Item { return menu.PaneEntriesToItems(c.Panes) }},
		})
		if m.paneForm != nil {
			m.paneForm.SyncContext(ctx)
		}
		// Pane content changes affect all pane-capture-based previews.
		if currentLvl != nil && previewCmd == nil {
			switch currentLvl.ID {
			case "pane:switch", "pane:join", "session:switch", "window:switch", "session:tree":
				previewCmd = m.refreshPreviewForLevel(currentLvl)
			}
		}
	}

	// Refresh tree level if any data source changed.
	if res.SessionsUpdated || res.WindowsUpdated || res.PanesUpdated {
		if lvl := m.findLevelByID("session:tree"); lvl != nil {
			m.treeSessions = ctx.Sessions
			m.treeWindows = ctx.Windows
			m.treePanes = ctx.Panes
			if ts, ok := lvl.Data.(*menu.TreeState); ok && ts != nil {
				m.rebuildTreeItems(lvl, ts)
			}
		}
		if currentLvl != nil && currentLvl.Node != nil && currentLvl.Node.FilterCommand {
			m.triggerCompletion()
		}
	}

	// Execute any deferred leaf action once pane data has arrived.
	// We wait specifically for PanesUpdated because leaf actions like
	// pane:capture need CurrentPaneID, which is only populated after
	// the pane poller's first result. Session or window events arriving
	// first must not trigger the action prematurely.
	if m.deferredAction != nil && res.PanesUpdated {
		node := m.deferredAction
		m.deferredAction = nil
		freshCtx := m.menuContext()
		deferredCmd := m.bus.Execute(freshCtx, command.Request{
			ID:      node.ID,
			Label:   node.ID,
			Handler: node.Action,
			Item:    menu.Item{ID: node.ID, Label: node.ID},
		})
		if previewCmd != nil {
			return tea.Batch(previewCmd, deferredCmd)
		}
		return deferredCmd
	}

	if warn, _ := m.hasBackendIssue(); !warn {
		m.backendLastErr = ""
	}
	return previewCmd
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

func sessionSwitchItems(ctx menu.Context) []menu.Item {
	return menu.SessionSwitchMenuItems(ctx)
}

func windowPushToSessionItems(ctx menu.Context) []menu.Item {
	items := make([]menu.Item, 0, len(ctx.Sessions))
	for _, entry := range ctx.Sessions {
		if entry.Name == ctx.CurrentWindowSession {
			continue
		}
		items = append(items, menu.Item{ID: entry.Name, Label: entry.Label})
	}
	return items
}

func windowSwapItems(ctx menu.Context) []menu.Item {
	items := menu.WindowEntriesToItems(ctx.Windows)
	if currentItem, ok := currentWindowMenuItem(ctx); ok {
		items = append([]menu.Item{currentItem}, items...)
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
