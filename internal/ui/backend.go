package ui

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
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
			items := sessionSwitchItems(ctx)
			lvl.UpdateItems(items)
			if len(lvl.Items) > 0 {
				m.clearInfo()
			}
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("session:rename"); lvl != nil {
			lvl.UpdateItems(menu.SessionRenameItems(ctx.Sessions))
			m.syncViewport(lvl)
		}
		base := menu.SessionEntriesToItems(ctx.Sessions)
		for _, id := range []string{"session:detach", "session:kill"} {
			if lvl := m.findLevelByID(id); lvl != nil {
				lvl.UpdateItems(base)
				m.syncViewport(lvl)
			}
		}
		if m.sessionForm != nil {
			m.sessionForm.SetSessions(ctx.Sessions)
		}
		if currentLvl != nil && currentLvl.ID == "session:switch" {
			previewCmd = m.refreshPreviewForLevel(currentLvl)
		}
	}

	if res.WindowsUpdated {
		m.pendingWindowSwap = nil
		if lvl := m.findLevelByID("window:switch"); lvl != nil {
			lvl.UpdateItems(menu.WindowSwitchItems(ctx))
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
			lvl.UpdateItems(items)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("window:swap"); lvl != nil {
			items := menu.WindowEntriesToItems(ctx.Windows)
			if currentItem, ok := currentWindowMenuItem(ctx); ok {
				items = append([]menu.Item{currentItem}, items...)
			}
			lvl.UpdateItems(items)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("window:kill"); lvl != nil {
			items := menu.WindowEntriesToItems(ctx.Windows)
			if currentItem, ok := currentWindowMenuItem(ctx); ok {
				items = append([]menu.Item{currentItem}, items...)
			}
			lvl.UpdateItems(items)
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		if currentLvl != nil && currentLvl.ID == "window:switch" {
			previewCmd = m.refreshPreviewForLevel(currentLvl)
		}
	}

	if res.PanesUpdated {
		m.pendingPaneSwap = nil
		if lvl := m.findLevelByID("pane:switch"); lvl != nil {
			lvl.UpdateItems(paneSwitchItems(ctx))
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:break"); lvl != nil {
			lvl.UpdateItems(paneBreakItems(ctx))
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:join"); lvl != nil {
			lvl.UpdateItems(paneJoinItems(ctx))
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:swap"); lvl != nil {
			lvl.UpdateItems(paneSwapItems(ctx))
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:kill"); lvl != nil {
			lvl.UpdateItems(paneKillItems(ctx))
			m.applyNodeSettings(lvl)
			m.syncViewport(lvl)
		}
		if lvl := m.findLevelByID("pane:rename"); lvl != nil {
			lvl.UpdateItems(menu.PaneEntriesToItems(ctx.Panes))
			m.syncViewport(lvl)
		}
		if m.paneForm != nil {
			m.paneForm.SyncContext(ctx)
		}
		// Pane content changes affect all pane-capture-based previews.
		if currentLvl != nil && previewCmd == nil {
			switch currentLvl.ID {
			case "pane:switch", "pane:join", "session:switch", "window:switch":
				previewCmd = m.refreshPreviewForLevel(currentLvl)
			}
		}
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
