package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

type pluginConfirmState struct {
	pending   []plugin.Plugin // plugins remaining to confirm
	confirmed []plugin.Plugin // plugins confirmed for removal
	current   plugin.Plugin   // currently being confirmed
	pluginDir string
	operation string // "uninstall" or "tidy"
}

type pluginReloadState struct {
	plugins   []plugin.Plugin
	pluginDir string
	summary   string
}

func (m *Model) handlePluginConfirmPromptMsg(msg tea.Msg) tea.Cmd {
	prompt := msg.(menu.PluginConfirmPrompt)
	if len(prompt.Plugins) == 0 {
		return func() tea.Msg {
			return menu.ActionResult{Info: "Nothing to remove"}
		}
	}
	m.pluginConfirmState = &pluginConfirmState{
		pending:   prompt.Plugins[1:],
		current:   prompt.Plugins[0],
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
	switch keyMsg.String() {
	case "y", "Y":
		s.confirmed = append(s.confirmed, s.current)
		return true, m.advancePluginConfirm()
	case "n", "N":
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
	if len(s.pending) > 0 {
		s.current = s.pending[0]
		s.pending = s.pending[1:]
		return nil
	}

	// All confirmed — execute removal
	confirmed := s.confirmed
	pluginDir := s.pluginDir
	operation := s.operation
	m.pluginConfirmState = nil
	m.mode = ModeMenu

	if len(confirmed) == 0 {
		return func() tea.Msg {
			return menu.ActionResult{Info: "No plugins removed"}
		}
	}

	m.loading = true
	m.pendingLabel = fmt.Sprintf("removing %d plugin(s)", len(confirmed))
	return func() tea.Msg {
		for _, p := range confirmed {
			events.Plugins.Uninstall(p.Name)
		}
		if err := plugin.Uninstall(pluginDir, confirmed); err != nil {
			return menu.ActionResult{Err: err}
		}
		action := "Uninstalled"
		if operation == "tidy" {
			action = "Tidied"
		}
		return menu.PluginReloadPrompt{
			Plugins:   confirmed,
			PluginDir: pluginDir,
			Summary:   fmt.Sprintf("%s %d plugin(s)", action, len(confirmed)),
		}
	}
}

func (m *Model) handlePluginReloadPromptMsg(msg tea.Msg) tea.Cmd {
	prompt := msg.(menu.PluginReloadPrompt)
	m.pluginReloadState = &pluginReloadState{
		plugins:   prompt.Plugins,
		pluginDir: prompt.PluginDir,
		summary:   prompt.Summary,
	}
	m.mode = ModePluginReload
	m.loading = false
	return nil
}

func (m *Model) handlePluginReload(msg tea.Msg) (bool, tea.Cmd) {
	if m.pluginReloadState == nil {
		return false, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}

	s := m.pluginReloadState
	switch keyMsg.String() {
	case "y", "Y":
		plugins := s.plugins
		pluginDir := s.pluginDir
		summary := s.summary
		m.pluginReloadState = nil
		m.mode = ModeMenu
		m.loading = true
		m.pendingLabel = "reloading plugins"
		return true, func() tea.Msg {
			for _, p := range plugins {
				events.Plugins.Source(p.Name)
			}
			if err := plugin.Source(pluginDir, plugins); err != nil {
				return menu.ActionResult{Err: fmt.Errorf("reload failed: %w", err)}
			}
			return menu.ActionResult{Info: summary + " (reloaded)"}
		}
	case "n", "N":
		summary := s.summary
		m.pluginReloadState = nil
		m.mode = ModeMenu
		return true, func() tea.Msg {
			return menu.ActionResult{Info: summary}
		}
	case "esc":
		summary := s.summary
		m.pluginReloadState = nil
		m.mode = ModeMenu
		return true, func() tea.Msg {
			return menu.ActionResult{Info: summary}
		}
	}
	return true, nil
}

// pluginConfirmView renders the confirmation prompt.
func (m *Model) pluginConfirmView() string {
	if m.pluginConfirmState == nil {
		return ""
	}
	s := m.pluginConfirmState
	return fmt.Sprintf(
		"Are you sure you want to remove the plugin named %s in the directory %s? [y/n]",
		s.current.Name,
		s.current.Dir,
	)
}

// pluginReloadView renders the reload prompt.
func (m *Model) pluginReloadView() string {
	if m.pluginReloadState == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s. Reload plugins? [y/n]",
		m.pluginReloadState.summary,
	)
}
