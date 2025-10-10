package ui

import (
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
)

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
			cmd = menu.SessionCommandForAction(actionID, ctx, target, name)
		}
		return true, cmd
	}
	if cmd != nil {
		return true, cmd
	}
	return true, nil
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
