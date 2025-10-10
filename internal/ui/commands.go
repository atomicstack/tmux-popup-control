package ui

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
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

func (m *Model) handleCommandPromptMsg(msg tea.Msg) tea.Cmd {
	prompt, ok := msg.(menu.CommandPromptMsg)
	if !ok {
		return nil
	}
	return m.withPrompt(func() promptResult {
		if err := menu.CommandPrompt(m.socketPath, prompt.Command); err != nil {
			events.Action.Error(err)
			return promptResult{Err: err}
		}
		info := fmt.Sprintf("Prompted command %s", strings.TrimSpace(prompt.Command))
		events.Action.Success(info)
		return promptResult{Cmd: tea.Quit, Info: info}
	})
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
