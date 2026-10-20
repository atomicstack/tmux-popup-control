package ui

import (
	"slices"
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

// refreshLiveLevels rebuilds every level in the stack that the menu package
// declares as live for src, in table order. session:switch additionally clears
// the info line once it has entries (before the viewport sync, since the info
// line affects the visible height).
func (m *Model) refreshLiveLevels(ctx menu.Context, src menu.LiveSource) {
	for _, live := range menu.LiveLevels(src) {
		lvl := m.findLevelByID(live.ID)
		if lvl == nil {
			continue
		}
		lvl.UpdateItems(live.Items(ctx))
		if live.ID == "session:switch" && len(lvl.Items) > 0 {
			m.clearInfo()
		}
		m.syncViewport(lvl)
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
		m.refreshLiveLevels(ctx, menu.LiveSessions)
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
		if m.pendingWindowSwap != nil && !slices.ContainsFunc(ctx.Windows, func(entry menu.WindowEntry) bool { return entry.ID == m.pendingWindowSwap.ID }) {
			m.pendingWindowSwap = nil
		}
		m.refreshLiveLevels(ctx, menu.LiveWindows)
		m.rebuildPullTree()
		if focusCmd := m.applyDeferredWindowRename(ctx); focusCmd != nil {
			previewCmd = tea.Batch(previewCmd, focusCmd)
		}
		if currentLvl != nil && currentLvl.ID == "window:switch" {
			previewCmd = m.refreshPreviewForLevel(currentLvl)
		}
	}

	if res.PanesUpdated {
		if m.pendingPaneSwap != nil && !slices.ContainsFunc(ctx.Panes, func(entry menu.PaneEntry) bool { return entry.ID == m.pendingPaneSwap.ID }) {
			m.pendingPaneSwap = nil
		}
		m.refreshLiveLevels(ctx, menu.LivePanes)
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
		m.refreshWindowMove()
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
