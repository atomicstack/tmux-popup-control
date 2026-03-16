package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

type pluginConfirmStatus int

const (
	pluginConfirmPending pluginConfirmStatus = iota
	pluginConfirmAsking
	pluginConfirmAccepted
	pluginConfirmSkipped
)

type pluginConfirmPhase int

const (
	pluginConfirmPhaseAsking   pluginConfirmPhase = iota
	pluginConfirmPhaseRemoving
	pluginConfirmPhaseDone
)

type pluginConfirmEntry struct {
	plugin plugin.Plugin
	status pluginConfirmStatus
}

type pluginConfirmState struct {
	entries   []pluginConfirmEntry
	current   int // index of entry currently being asked
	pluginDir string
	operation string // "uninstall" or "tidy"
	phase     pluginConfirmPhase
	summary   string
	confirmed []plugin.Plugin // plugins that were removed (for reload)
}

type pluginRemovalDoneMsg struct {
	err error
}

func (m *Model) handlePluginConfirmPromptMsg(msg tea.Msg) tea.Cmd {
	prompt := msg.(menu.PluginConfirmPrompt)
	if len(prompt.Plugins) == 0 {
		return func() tea.Msg {
			return menu.ActionResult{Info: "Nothing to remove"}
		}
	}
	entries := make([]pluginConfirmEntry, len(prompt.Plugins))
	for i, p := range prompt.Plugins {
		entries[i] = pluginConfirmEntry{plugin: p, status: pluginConfirmPending}
	}
	entries[0].status = pluginConfirmAsking
	m.pluginConfirmState = &pluginConfirmState{
		entries:   entries,
		current:   0,
		pluginDir: prompt.PluginDir,
		operation: prompt.Operation,
	}
	m.mode = ModePluginConfirm
	m.loading = false
	return nil
}

func (m *Model) handlePluginConfirm(msg tea.Msg) (bool, tea.Cmd) {
	if m.pluginConfirmState == nil {
		return false, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}

	s := m.pluginConfirmState

	// Done phase: handle reload prompt.
	if s.phase == pluginConfirmPhaseDone {
		switch keyMsg.String() {
		case "y", "Y":
			if len(s.confirmed) == 0 {
				m.pluginConfirmState = nil
				m.mode = ModeMenu
				return true, nil
			}
			confirmed := s.confirmed
			pluginDir := s.pluginDir
			summary := s.summary
			m.pluginConfirmState = nil
			m.mode = ModeMenu
			m.loading = true
			m.pendingLabel = "reloading plugins"
			return true, func() tea.Msg {
				for _, p := range confirmed {
					events.Plugins.Source(p.Name)
				}
				if err := plugin.Source(pluginDir, confirmed); err != nil {
					return menu.ActionResult{Err: fmt.Errorf("reload failed: %w", err)}
				}
				return menu.ActionResult{Info: summary + " (reloaded)"}
			}
		case "n", "N", "esc":
			summary := s.summary
			m.pluginConfirmState = nil
			m.mode = ModeMenu
			if len(s.confirmed) > 0 {
				return true, func() tea.Msg {
					return menu.ActionResult{Info: summary}
				}
			}
			return true, nil
		}
		return true, nil
	}

	// Removing phase: consume all keys.
	if s.phase == pluginConfirmPhaseRemoving {
		return true, nil
	}

	// Asking phase.
	switch keyMsg.String() {
	case "y", "Y":
		s.entries[s.current].status = pluginConfirmAccepted
		return true, m.advancePluginConfirm()
	case "n", "N":
		s.entries[s.current].status = pluginConfirmSkipped
		return true, m.advancePluginConfirm()
	case "esc":
		m.pluginConfirmState = nil
		m.mode = ModeMenu
		return true, nil
	}
	return true, nil
}

func (m *Model) advancePluginConfirm() tea.Cmd {
	s := m.pluginConfirmState

	// Find next pending entry.
	for i := s.current + 1; i < len(s.entries); i++ {
		if s.entries[i].status == pluginConfirmPending {
			s.entries[i].status = pluginConfirmAsking
			s.current = i
			return nil
		}
	}

	// All answered — collect confirmed plugins.
	var confirmed []plugin.Plugin
	for _, e := range s.entries {
		if e.status == pluginConfirmAccepted {
			confirmed = append(confirmed, e.plugin)
		}
	}

	if len(confirmed) == 0 {
		s.phase = pluginConfirmPhaseDone
		s.summary = "No plugins removed"
		return nil
	}

	s.phase = pluginConfirmPhaseRemoving
	pluginDir := s.pluginDir
	return func() tea.Msg {
		for _, p := range confirmed {
			events.Plugins.Uninstall(p.Name)
		}
		if err := plugin.Uninstall(pluginDir, confirmed); err != nil {
			return pluginRemovalDoneMsg{err: err}
		}
		return pluginRemovalDoneMsg{}
	}
}

func (m *Model) handlePluginRemovalDoneMsg(msg tea.Msg) tea.Cmd {
	done := msg.(pluginRemovalDoneMsg)
	s := m.pluginConfirmState
	if s == nil {
		return nil
	}
	if done.err != nil {
		m.pluginConfirmState = nil
		m.mode = ModeMenu
		return func() tea.Msg {
			return menu.ActionResult{Err: done.err}
		}
	}

	var confirmed []plugin.Plugin
	for _, e := range s.entries {
		if e.status == pluginConfirmAccepted {
			confirmed = append(confirmed, e.plugin)
		}
	}

	action := "Uninstalled"
	if s.operation == "tidy" {
		action = "Tidied"
	}
	s.phase = pluginConfirmPhaseDone
	s.summary = fmt.Sprintf("%s %d plugin(s)", action, len(confirmed))
	s.confirmed = confirmed
	return nil
}

// pluginConfirmView renders the confirmation flow with per-plugin status.
func (m *Model) pluginConfirmView() string {
	s := m.pluginConfirmState
	if s == nil {
		return ""
	}

	headerText := "Uninstalling plugins..."
	if s.operation == "tidy" {
		headerText = "Tidying plugins..."
	}
	if s.phase == pluginConfirmPhaseDone {
		headerText = "Uninstall complete"
		if s.operation == "tidy" {
			headerText = "Tidy complete"
		}
	}

	var b strings.Builder
	b.WriteString(styles.Header.Render(headerText))
	b.WriteString("\n\n")

	for _, e := range s.entries {
		b.WriteString("  ")
		switch e.status {
		case pluginConfirmAccepted:
			b.WriteString(styles.CheckboxChecked.Render("✓"))
			b.WriteString(" ")
			b.WriteString(styles.CheckboxChecked.Render(e.plugin.Name))
		case pluginConfirmSkipped:
			b.WriteString(styles.Checkbox.Render("–"))
			b.WriteString(" ")
			b.WriteString(styles.Checkbox.Render(e.plugin.Name))
			b.WriteString(styles.Info.Render("  skipped"))
		case pluginConfirmAsking:
			b.WriteString(styles.CheckboxAll.Render("◆"))
			b.WriteString(" ")
			b.WriteString(e.plugin.Name)
			b.WriteString(styles.Info.Render(fmt.Sprintf("  remove %s? [y/n]", e.plugin.Dir)))
		default: // pending
			b.WriteString(styles.Checkbox.Render("○"))
			b.WriteString(" ")
			b.WriteString(styles.Checkbox.Render(e.plugin.Name))
		}
		b.WriteString("\n")
	}

	switch s.phase {
	case pluginConfirmPhaseRemoving:
		b.WriteString("\n")
		b.WriteString(styles.Info.Render("removing..."))
	case pluginConfirmPhaseDone:
		b.WriteString("\n")
		if len(s.confirmed) > 0 {
			b.WriteString(fmt.Sprintf("%s. Reload plugins? [y/n]", s.summary))
		} else {
			b.WriteString(styles.Info.Render(s.summary))
			b.WriteString("\nPress esc to return.")
		}
	}

	return lipgloss.Wrap(b.String(), m.width, "")
}
