package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
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
		if m.rootMenuID == "window:rename" {
			return true, tea.Quit
		}
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
		if m.rootMenuID == "session:rename" {
			return true, tea.Quit
		}
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

func (m *Model) startSessionForm(prompt menu.SessionPrompt) tea.Cmd {
	m.sessionForm = menu.NewSessionForm(prompt)
	m.mode = ModeSessionForm
	return m.sessionForm.FocusCmd()
}

func (m *Model) startWindowForm(prompt menu.WindowPrompt) tea.Cmd {
	m.windowForm = menu.NewWindowRenameForm(prompt)
	m.mode = ModeWindowForm
	return m.windowForm.FocusCmd()
}

func (m *Model) startPaneForm(prompt menu.PanePrompt) tea.Cmd {
	m.paneForm = menu.NewPaneRenameForm(prompt)
	m.mode = ModePaneForm
	return m.paneForm.FocusCmd()
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
	title := m.sessionForm.Title()
	if header != "" && title != "" {
		lines = append(lines, header+menuHeaderSeparator+strings.ToLower(title))
	} else if header != "" {
		lines = append(lines, header)
	} else {
		lines = append(lines, title)
	}
	lines = append(lines, "", m.sessionForm.InputView())
	if err := m.sessionForm.Error(); err != "" {
		style := styles.Error
		if m.sessionForm.ErrorIsWarning() && styles.Warning != nil {
			style = styles.Warning
		}
		lines = append(lines, "", style.Render(err))
	}
	lines = append(lines, "", m.sessionForm.Help())
	return strings.Join(lines, "\n")
}

func (m *Model) handleSaveForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.saveForm == nil {
		return false, nil
	}
	cmd, done, cancel := m.saveForm.Update(msg)
	if cancel {
		m.saveForm = nil
		m.mode = ModeMenu
		return true, cmd
	}
	if done {
		name := m.saveForm.Value()
		ctx := m.saveForm.Context()
		saveDir := m.saveForm.SaveDir()
		m.saveForm = nil
		m.mode = ModeMenu
		return true, func() tea.Msg {
			return menu.ResurrectStart{
				Operation: "save",
				Name:      name,
				Config: resurrect.Config{
					SocketPath:          ctx.SocketPath,
					SaveDir:             saveDir,
					Name:                name,
					CapturePaneContents: resurrect.ResolvePaneContents(ctx.SocketPath),
					ClientID:            ctx.ClientID,
				},
			}
		}
	}
	if cmd != nil {
		return true, cmd
	}
	return true, nil
}

func (m *Model) startSaveForm(prompt menu.SaveAsPrompt) tea.Cmd {
	m.loading = false
	m.saveForm = menu.NewSaveForm(prompt)
	m.mode = ModeSessionSaveForm
	return m.saveForm.FocusCmd()
}

func (m *Model) handleSaveAsPromptMsg(msg tea.Msg) tea.Cmd {
	prompt := msg.(menu.SaveAsPrompt)
	return m.startSaveForm(prompt)
}

func (m *Model) viewSaveForm() string {
	subtitle := lipgloss.NewStyle().Faint(true).Render(m.saveForm.Subtitle())
	lines := []string{m.saveForm.Title(), subtitle, "", m.saveForm.InputView()}
	if err := m.saveForm.Error(); err != "" {
		lines = append(lines, "", styles.Info.Render(err))
	}
	lines = append(lines, "", m.saveForm.Help())
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

func (m *Model) handlePaneCaptureForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.paneCaptureForm == nil {
		return false, nil
	}
	// Preview results arrive while the form is active; apply them here before
	// the form's own Update can consume the message as raw textinput input.
	if preview, ok := msg.(menu.PaneCapturePreviewMsg); ok {
		if preview.Seq == m.paneCaptureForm.Seq() {
			m.paneCaptureForm.SetPreview(preview.Path, preview.Err)
		}
		return true, nil
	}
	seqBefore := m.paneCaptureForm.Seq()
	cmd, done, cancel := m.paneCaptureForm.Update(msg)
	if cancel {
		m.paneCaptureForm = nil
		m.mode = ModeMenu
		return true, cmd
	}
	if done {
		ctx := m.paneCaptureForm.Context()
		template := m.paneCaptureForm.Value()
		escSeqs := m.paneCaptureForm.EscSeqs()
		actionID := m.paneCaptureForm.ActionID()
		pendingLabel := m.paneCaptureForm.PendingLabel()
		m.paneCaptureForm = nil
		m.mode = ModeMenu
		m.loading = true
		m.pendingID = actionID
		m.pendingLabel = pendingLabel
		return true, menu.PaneCaptureCommand(ctx, template, escSeqs)
	}
	// Only fire preview expansion when the input actually changed (seq advanced).
	if m.paneCaptureForm.Seq() != seqBefore {
		cmds := []tea.Cmd{}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.paneCaptureForm.ExpandPreviewCmd())
		return true, tea.Batch(cmds...)
	}
	return true, cmd
}

func (m *Model) startPaneCaptureForm(prompt menu.PaneCapturePrompt) tea.Cmd {
	m.paneCaptureForm = menu.NewPaneCaptureForm(prompt)
	m.mode = ModePaneCaptureForm
	return m.paneCaptureForm.FocusCmd()
}

func (m *Model) viewPaneCaptureForm(header string) string {
	f := m.paneCaptureForm
	lines := []string{}
	if header != "" {
		title := f.Title()
		lines = append(lines, header+menuHeaderSeparator+title)
	} else {
		lines = append(lines, f.Title())
	}
	lines = append(lines, "", f.InputView(), "")

	// Checkbox line.
	checkboxLine := f.CheckboxView()
	if f.EscSeqs() && styles.CheckboxChecked != nil {
		checkboxLine = styles.CheckboxChecked.Render("■") + " capture escape sequences"
	} else if styles.Checkbox != nil {
		checkboxLine = styles.Checkbox.Render("□") + " capture escape sequences"
	}
	lines = append(lines, checkboxLine, "")

	// Preview line.
	if f.PreviewErr() != "" {
		lines = append(lines, styles.Error.Render(f.PreviewErr()))
	} else if f.Preview() != "" {
		preview := lipgloss.NewStyle().Faint(true).Render(f.Preview())
		lines = append(lines, preview)
	}
	lines = append(lines, "", f.Help())
	return strings.Join(lines, "\n")
}
