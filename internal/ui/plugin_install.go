package ui

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

type pluginInstallStatus int

const (
	pluginInstallPending pluginInstallStatus = iota
	pluginInstallActive
	pluginInstallDone
	pluginInstallError
)

type pluginInstallEntry struct {
	plugin plugin.Plugin
	status pluginInstallStatus
	err    error
}

type pluginInstallState struct {
	entries   []pluginInstallEntry
	pluginDir string
	operation string // "install" or "update"
	finished  bool
	installed []plugin.Plugin // plugins that succeeded (for reload)
	summary   string
}

type pluginInstallDoneMsg struct {
	index int
	err   error
}

func (m *Model) startPluginProgress(plugins []plugin.Plugin, pluginDir, operation string) tea.Cmd {
	entries := make([]pluginInstallEntry, len(plugins))
	for i, p := range plugins {
		entries[i] = pluginInstallEntry{plugin: p}
	}
	m.pluginInstallState = &pluginInstallState{
		entries:   entries,
		pluginDir: pluginDir,
		operation: operation,
	}
	m.mode = ModePluginInstall
	m.loading = false
	return m.advancePluginInstall()
}

func (m *Model) handlePluginInstallStartMsg(msg tea.Msg) tea.Cmd {
	start := msg.(menu.PluginInstallStart)
	return m.startPluginProgress(start.Plugins, start.PluginDir, "install")
}

func (m *Model) handlePluginUpdateStartMsg(msg tea.Msg) tea.Cmd {
	start := msg.(menu.PluginUpdateStart)
	return m.startPluginProgress(start.Plugins, start.PluginDir, "update")
}

func (m *Model) handlePluginInstallKey(msg tea.Msg) (bool, tea.Cmd) {
	s := m.pluginInstallState
	if s == nil {
		return false, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}

	if s.finished {
		switch keyMsg.String() {
		case "y", "Y":
			if len(s.installed) == 0 {
				m.pluginInstallState = nil
				m.mode = ModeMenu
				return true, nil
			}
			installed := s.installed
			pluginDir := s.pluginDir
			summary := s.summary
			m.pluginInstallState = nil
			m.mode = ModeMenu
			m.loading = true
			m.pendingLabel = "reloading plugins"
			return true, func() tea.Msg {
				for _, p := range installed {
					events.Plugins.Source(p.Name)
				}
				if err := plugin.Source(pluginDir, installed); err != nil {
					return menu.ActionResult{Err: fmt.Errorf("reload failed: %w", err)}
				}
				return menu.ActionResult{Info: summary + " (reloaded)"}
			}
		case "n", "N", "esc":
			summary := s.summary
			m.pluginInstallState = nil
			m.mode = ModeMenu
			if len(s.installed) > 0 {
				return true, func() tea.Msg {
					return menu.ActionResult{Info: summary}
				}
			}
			return true, nil
		}
		return true, nil
	}

	if keyMsg.String() == "esc" {
		m.pluginInstallState = nil
		m.mode = ModeMenu
		return true, nil
	}
	return true, nil // consume all keys during operation
}

func (m *Model) handlePluginInstallDoneMsg(msg tea.Msg) tea.Cmd {
	done := msg.(pluginInstallDoneMsg)
	s := m.pluginInstallState
	if s == nil {
		return nil
	}
	if done.index >= 0 && done.index < len(s.entries) {
		if done.err != nil {
			s.entries[done.index].status = pluginInstallError
			s.entries[done.index].err = done.err
		} else {
			s.entries[done.index].status = pluginInstallDone
		}
	}
	return m.advancePluginInstall()
}

func (m *Model) advancePluginInstall() tea.Cmd {
	s := m.pluginInstallState
	if s == nil {
		return nil
	}
	for i := range s.entries {
		if s.entries[i].status == pluginInstallPending {
			s.entries[i].status = pluginInstallActive
			p := s.entries[i].plugin
			pluginDir := s.pluginDir
			idx := i
			op := s.operation
			return func() tea.Msg {
				var err error
				if op == "update" {
					events.Plugins.Update(p.Name)
					err = plugin.UpdateOne(p)
				} else {
					events.Plugins.Install(p.Name)
					err = plugin.InstallOne(pluginDir, p)
				}
				return pluginInstallDoneMsg{index: idx, err: err}
			}
		}
	}

	// All done — gather results and stay in progress mode so the
	// view remains visible with the reload prompt appended.
	var installed []plugin.Plugin
	var errCount int
	for _, e := range s.entries {
		if e.status == pluginInstallDone {
			installed = append(installed, e.plugin)
		}
		if e.status == pluginInstallError {
			errCount++
		}
	}
	s.finished = true
	s.installed = installed

	verb := "Installed"
	failVerb := "install"
	if s.operation == "update" {
		verb = "Updated"
		failVerb = "update"
	}
	if len(installed) == 0 {
		s.summary = fmt.Sprintf("All %d plugin(s) failed to %s", errCount, failVerb)
	} else {
		s.summary = fmt.Sprintf("%s %d plugin(s)", verb, len(installed))
		if errCount > 0 {
			s.summary += fmt.Sprintf(" (%d failed)", errCount)
		}
	}
	return nil
}

func (m *Model) pluginInstallView() string {
	s := m.pluginInstallState
	if s == nil {
		return ""
	}

	headerText := "Installing plugins..."
	activeText := "cloning..."
	if s.operation == "update" {
		headerText = "Updating plugins..."
		activeText = "pulling..."
	}

	total := len(s.entries)
	done := 0
	for _, e := range s.entries {
		if e.status == pluginInstallDone || e.status == pluginInstallError {
			done++
		}
	}

	var b strings.Builder

	b.WriteString(styles.Header.Render(headerText))
	b.WriteString("\n\n")

	// Progress bar.
	barWidth := 30
	if m.width > 50 {
		barWidth = m.width - 20
	}
	if barWidth > 50 {
		barWidth = 50
	}
	exactFilled := 0.0
	if total > 0 {
		exactFilled = float64(barWidth) * float64(done) / float64(total)
		if exactFilled > float64(barWidth) {
			exactFilled = float64(barWidth)
		}
	}
	wholeFilled := int(exactFilled)
	frac := exactFilled - float64(wholeFilled)

	var filledStyle lipgloss.Style
	if styles.ProgressFilled != nil {
		filledStyle = *styles.ProgressFilled
	}
	var bgStyle lipgloss.Style
	if styles.ProgressEmptyBg != nil {
		bgStyle = *styles.ProgressEmptyBg
	}

	eighths := []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

	var bar strings.Builder
	bar.WriteString(filledStyle.Render(strings.Repeat("█", wholeFilled)))
	if wholeFilled < barWidth {
		idx := int(math.Round(frac * 8))
		if idx > 7 {
			idx = 7
		}
		if idx > 0 {
			bar.WriteString(filledStyle.Inherit(bgStyle).Render(eighths[idx]))
		} else {
			bar.WriteString(bgStyle.Render(" "))
		}
		if barWidth-wholeFilled-1 > 0 {
			bar.WriteString(bgStyle.Render(strings.Repeat(" ", barWidth-wholeFilled-1)))
		}
	}
	counter := fmt.Sprintf(" %d/%d", done, total)
	b.WriteString("  ")
	b.WriteString(bar.String())
	b.WriteString(styles.Info.Render(counter))
	b.WriteString("\n\n")

	// Per-plugin status lines.
	for _, e := range s.entries {
		b.WriteString("  ")
		switch e.status {
		case pluginInstallDone:
			b.WriteString(styles.CheckboxChecked.Render("✓"))
			b.WriteString(" ")
			b.WriteString(styles.CheckboxChecked.Render(e.plugin.Name))
		case pluginInstallActive:
			b.WriteString(styles.CheckboxAll.Render("◆"))
			b.WriteString(" ")
			b.WriteString(e.plugin.Name)
			b.WriteString(styles.Info.Render("  " + activeText))
		case pluginInstallError:
			b.WriteString(styles.Error.Render("✗"))
			b.WriteString(" ")
			b.WriteString(e.plugin.Name)
			if e.err != nil {
				b.WriteString(styles.Error.Render(fmt.Sprintf("  %s", e.err)))
			}
		default:
			b.WriteString(styles.Checkbox.Render("○"))
			b.WriteString(" ")
			b.WriteString(styles.Checkbox.Render(e.plugin.Name))
		}
		b.WriteString("\n")
	}

	if s.finished {
		b.WriteString("\n")
		if len(s.installed) > 0 {
			b.WriteString(fmt.Sprintf("%s. Reload plugins? [y/n]", s.summary))
		} else {
			b.WriteString(styles.Error.Render(s.summary))
			b.WriteString("\nPress esc to return.")
		}
	}

	return b.String()
}
