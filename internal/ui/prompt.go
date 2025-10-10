package ui

import (
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
)

type promptResult struct {
	Cmd  tea.Cmd
	Info string
	Err  error
}

// withPrompt centralises the common prompt flow: close the popup, reset state,
// and execute the provided action. The action can return a promptResult to
// control follow-up behaviour (command to run, informational message, or
// error).
func (m *Model) withPrompt(action func() promptResult) tea.Cmd {
	m.loading = false
	m.pendingID = ""
	m.pendingLabel = ""
	m.forceClearInfo()
	m.errMsg = ""
	if action == nil {
		return nil
	}
	result := action()
	if result.Err != nil {
		m.errMsg = result.Err.Error()
		return nil
	}
	if result.Info != "" && m.verbose {
		m.setInfo(result.Info)
	}
	return result.Cmd
}

func (m *Model) handleWindowPromptMsg(msg tea.Msg) tea.Cmd {
	prompt, ok := msg.(menu.WindowPrompt)
	if !ok {
		return nil
	}
	return m.withPrompt(func() promptResult {
		m.startWindowForm(prompt)
		return promptResult{}
	})
}

func (m *Model) handlePanePromptMsg(msg tea.Msg) tea.Cmd {
	prompt, ok := msg.(menu.PanePrompt)
	if !ok {
		return nil
	}
	return m.withPrompt(func() promptResult {
		m.startPaneForm(prompt)
		return promptResult{}
	})
}

func (m *Model) handleWindowSwapPromptMsg(msg tea.Msg) tea.Cmd {
	prompt, ok := msg.(menu.WindowSwapPrompt)
	if !ok {
		return nil
	}
	return m.withPrompt(func() promptResult {
		m.startWindowSwap(prompt)
		return promptResult{}
	})
}

func (m *Model) handlePaneSwapPromptMsg(msg tea.Msg) tea.Cmd {
	prompt, ok := msg.(menu.PaneSwapPrompt)
	if !ok {
		return nil
	}
	return m.withPrompt(func() promptResult {
		m.startPaneSwap(prompt)
		return promptResult{}
	})
}

func (m *Model) handleSessionPromptMsg(msg tea.Msg) tea.Cmd {
	prompt, ok := msg.(menu.SessionPrompt)
	if !ok {
		return nil
	}
	return m.withPrompt(func() promptResult {
		m.startSessionForm(prompt)
		return promptResult{}
	})
}
