package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func (m *Model) handleActionResultMsg(msg tea.Msg) tea.Cmd {
	result, ok := msg.(menu.ActionResult)
	if !ok {
		return nil
	}
	m.loading = false
	m.pendingID = ""
	m.pendingLabel = ""
	if result.Err != nil {
		m.errMsg = result.Err.Error()
		m.forceClearInfo()
		events.Action.Error(result.Err)
		return nil
	}
	if result.Info != "" && m.verbose {
		m.setInfo(result.Info)
	} else {
		m.forceClearInfo()
	}
	events.Action.Success(result.Info)
	return tea.Quit
}

// commandPreloadMsg delivers the pre-fetched command list items.
type commandPreloadMsg struct {
	items []menu.Item
	err   error
}

// preloadCommandList fires an async command to fetch the command list.
func preloadCommandList(socketPath string, loader menu.Loader) tea.Cmd {
	return func() tea.Msg {
		items, err := loader(menu.Context{SocketPath: socketPath})
		return commandPreloadMsg{items: items, err: err}
	}
}

func (m *Model) handleCommandPreloadMsg(msg tea.Msg) tea.Cmd {
	preload, ok := msg.(commandPreloadMsg)
	if !ok {
		return nil
	}
	if preload.err != nil {
		logging.Error(preload.err)
		return nil
	}
	m.commandItemsCache = preload.items
	return nil
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

// categoryLoadedMsg mirrors the async loader response.
type categoryLoadedMsg struct {
	id    string
	title string
	items []menu.Item
	err   error
}

func (m *Model) menuContext() menu.Context {
	ctx := menu.Context{
		SocketPath:           m.socketPath,
		ClientID:             m.clientID,
		MenuArgs:             m.menuArgs,
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
	for _, w := range ctx.Windows {
		if w.Current {
			ctx.CurrentWindowLayout = w.Layout
			break
		}
	}
	return ctx
}
