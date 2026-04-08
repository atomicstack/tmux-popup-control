package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

type pluginInstallStatus int

const (
	pluginInstallQueued pluginInstallStatus = iota
	pluginInstallPreparing
	pluginInstallCloning
	pluginInstallPulling
	pluginInstallSubmodules
	pluginInstallDone
	pluginInstallFailed
)

type pluginInstallEntry struct {
	plugin plugin.Plugin
	status pluginInstallStatus
	err    error
}

type pluginInstallState struct {
	entries         []pluginInstallEntry
	pluginDir       string
	operation       string // "install" or "update"
	finished        bool
	installed       []plugin.Plugin
	summary         string
	progressCurrent int
	progressTotal   int
}

type pluginInstallStageMsg struct {
	index int
	phase pluginInstallStatus
}

type pluginInstallResultMsg struct {
	index int
	err   error
}

var reloadPluginsFn = plugin.Source

func (m *Model) startPluginProgress(plugins []plugin.Plugin, pluginDir, operation string) tea.Cmd {
	entries := make([]pluginInstallEntry, len(plugins))
	for i, p := range plugins {
		entries[i] = pluginInstallEntry{plugin: p, status: pluginInstallQueued}
	}
	s := &pluginInstallState{
		entries:       entries,
		pluginDir:     pluginDir,
		operation:     operation,
		progressTotal: pluginInstallTotalSteps(operation, len(plugins)),
	}
	if len(plugins) == 0 {
		s.finished = true
		s.progressCurrent = s.progressTotal
		s.summary = pluginInstallEmptySummary(operation)
		m.pluginInstallState = s
		m.mode = ModePluginInstall
		m.loading = false
		return nil
	}
	s.progressCurrent = 1 // batch setup
	m.pluginInstallState = s
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
				if err := reloadPluginsFn(pluginDir, installed); err != nil {
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
	return true, nil
}

func (m *Model) handlePluginInstallStageMsg(msg tea.Msg) tea.Cmd {
	stage := msg.(pluginInstallStageMsg)
	s := m.pluginInstallState
	if s == nil {
		return nil
	}
	if stage.index < 0 || stage.index >= len(s.entries) {
		return nil
	}
	s.entries[stage.index].status = stage.phase
	s.entries[stage.index].err = nil
	m.bumpPluginInstallProgress()
	return m.runPluginInstallStage(stage.index, stage.phase)
}

func (m *Model) handlePluginInstallResultMsg(msg tea.Msg) tea.Cmd {
	done := msg.(pluginInstallResultMsg)
	s := m.pluginInstallState
	if s == nil {
		return nil
	}
	if done.index >= 0 && done.index < len(s.entries) {
		if done.err != nil {
			s.entries[done.index].status = pluginInstallFailed
			s.entries[done.index].err = done.err
		} else {
			s.entries[done.index].status = pluginInstallDone
		}
	}
	m.bumpPluginInstallProgress()
	return m.advancePluginInstall()
}

func (m *Model) advancePluginInstall() tea.Cmd {
	s := m.pluginInstallState
	if s == nil {
		return nil
	}
	for i := range s.entries {
		if s.entries[i].status != pluginInstallQueued {
			continue
		}
		s.entries[i].status = pluginInstallPreparing
		s.entries[i].err = nil
		m.bumpPluginInstallProgress()
		idx := i
		nextPhase := pluginInstallCloning
		if s.operation == "update" {
			nextPhase = pluginInstallPulling
		}
		return func() tea.Msg {
			return pluginInstallStageMsg{index: idx, phase: nextPhase}
		}
	}
	return m.finishPluginInstall()
}

func (m *Model) finishPluginInstall() tea.Cmd {
	s := m.pluginInstallState
	if s == nil || s.finished {
		return nil
	}
	s.progressCurrent = s.progressTotal
	s.finished = true

	var installed []plugin.Plugin
	var errCount int
	for _, e := range s.entries {
		switch e.status {
		case pluginInstallDone:
			installed = append(installed, e.plugin)
		case pluginInstallFailed:
			errCount++
		}
	}
	s.installed = sourceableInstalledPlugins(s.pluginDir, installed)

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

func (m *Model) bumpPluginInstallProgress() {
	s := m.pluginInstallState
	if s == nil {
		return
	}
	if s.progressTotal <= 0 {
		s.progressTotal = 1
	}
	if s.progressCurrent < s.progressTotal {
		s.progressCurrent++
	}
}

func (m *Model) runPluginInstallStage(index int, phase pluginInstallStatus) tea.Cmd {
	s := m.pluginInstallState
	if s == nil || index < 0 || index >= len(s.entries) {
		return nil
	}
	p := s.entries[index].plugin

	switch phase {
	case pluginInstallCloning:
		return func() tea.Msg {
			events.Plugins.Install(p.Name)
			return pluginInstallResultMsg{index: index, err: plugin.InstallOne(s.pluginDir, p)}
		}
	case pluginInstallPulling:
		return func() tea.Msg {
			events.Plugins.Update(p.Name)
			if err := plugin.UpdatePullOne(p); err != nil {
				return pluginInstallResultMsg{index: index, err: err}
			}
			return pluginInstallStageMsg{index: index, phase: pluginInstallSubmodules}
		}
	case pluginInstallSubmodules:
		return func() tea.Msg {
			if err := plugin.UpdateSubmodulesOne(p); err != nil {
				return pluginInstallResultMsg{index: index, err: err}
			}
			return pluginInstallResultMsg{index: index, err: nil}
		}
	default:
		return nil
	}
}

func (m *Model) pluginInstallView() string {
	s := m.pluginInstallState
	if s == nil {
		return ""
	}

	bodyRows := m.height - 1
	if bodyRows < 0 {
		bodyRows = 0
	}
	if bodyRows == 0 {
		return m.buildPluginInstallProgressBar(s, m.width)
	}
	bodyLines := m.pluginInstallBodyLines(s, bodyRows)
	bodyLines = applyWidth(bodyLines, m.width)
	if len(bodyLines) > bodyRows {
		bodyLines = bodyLines[len(bodyLines)-bodyRows:]
	}
	for len(bodyLines) < bodyRows {
		bodyLines = append(bodyLines, styledLine{})
	}

	var b strings.Builder
	b.WriteString(renderLines(bodyLines))
	if len(bodyLines) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(m.buildPluginInstallProgressBar(s, m.width))
	return b.String()
}

func (m *Model) pluginInstallBodyLines(s *pluginInstallState, bodyRows int) []styledLine {
	if s.finished {
		return pluginInstallFinishedBodyLines(s, bodyRows, m.width)
	}
	return pluginInstallVisibleEntries(s.entries, bodyRows, m.width)
}

func pluginInstallCompletionLines(s *pluginInstallState) []styledLine {
	if len(s.installed) > 0 {
		return []styledLine{
			{},
			{text: s.summary, style: styles.Info},
			{text: "Reload plugins? [y/n]", style: styles.Info},
		}
	}
	return []styledLine{
		{},
		{text: s.summary, style: styles.Error},
		{text: "Press esc to return.", style: styles.Info},
	}
}

func pluginInstallFinishedBodyLines(s *pluginInstallState, bodyRows, width int) []styledLine {
	completion := pluginInstallCompletionLines(s)
	completionRows := len(completion)
	if completionRows > bodyRows {
		return limitHeight(completion, bodyRows, width)
	}

	pluginRows := bodyRows - completionRows
	lines := pluginInstallVisibleEntries(s.entries, pluginRows, width)
	lines = append(lines, completion...)
	return lines
}

func pluginInstallVisibleEntries(entries []pluginInstallEntry, rows, width int) []styledLine {
	if rows <= 0 || len(entries) == 0 {
		return nil
	}
	blockRows := pluginInstallCellRows()
	visibleCount := rows / blockRows
	if visibleCount <= 0 {
		return nil
	}
	if visibleCount > len(entries) {
		visibleCount = len(entries)
	}
	start := len(entries) - visibleCount
	lines := make([]styledLine, 0, visibleCount*blockRows)
	for i := start; i < len(entries); i++ {
		lines = append(lines, pluginInstallEntryCellLines(entries[i], width)...)
		if i < len(entries)-1 {
			lines = append(lines, styledLine{})
		}
	}
	return lines
}

func shortPluginError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.TrimSpace(err.Error())
	msg = strings.ReplaceAll(msg, "\n", " ")
	if msg == "" {
		return "error"
	}
	const maxLen = 36
	if len([]rune(msg)) <= maxLen {
		return msg
	}
	return truncateText(msg, maxLen)
}

func pluginInstallCellRows() int {
	return 6
}

func pluginInstallEntryCellLines(e pluginInstallEntry, width int) []styledLine {
	if width <= 0 {
		return nil
	}

	const (
		cellBg      = "#1c1c1c"
		borderColor = "238"
		nameColor   = "255"
		sourceColor = "250"
		detailColor = "245"
	)

	contentWidth := width - 2
	if contentWidth < 0 {
		contentWidth = 0
	}
	cellBgStyle := lipgloss.NewStyle().Background(lipgloss.Color(cellBg))
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor)).Background(lipgloss.Color(cellBg))
	nameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(nameColor)).Bold(true)
	sourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(sourceColor))
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(detailColor))

	statusLabel, statusStyle := pluginInstallStatusLabel(e)
	detailText, detailStyleOverride := pluginInstallDetailText(e)
	if detailStyleOverride != nil {
		detailStyle = *detailStyleOverride
	}

	top := borderStyle.Render("┌" + strings.Repeat("─", contentWidth) + "┐")
	bottom := borderStyle.Render("└" + strings.Repeat("─", contentWidth) + "┘")
	line1 := pluginInstallCellTextLine(nameStyle, borderStyle, cellBgStyle, contentWidth, e.plugin.Name)
	line2 := pluginInstallCellSplitLine(sourceStyle, statusStyle, borderStyle, cellBgStyle, contentWidth, pluginInstallSourceLine(e.plugin.Source, e.plugin.Dir), statusLabel)
	line3 := pluginInstallCellTextLine(detailStyle, borderStyle, cellBgStyle, contentWidth, detailText)

	return []styledLine{
		{text: top, raw: true},
		{text: line1, raw: true},
		{text: line2, raw: true},
		{text: line3, raw: true},
		{text: bottom, raw: true},
	}
}

func pluginInstallStatusLabel(e pluginInstallEntry) (string, lipgloss.Style) {
	switch e.status {
	case pluginInstallQueued:
		return "queued", statusLabelStyle("241")
	case pluginInstallPreparing, pluginInstallCloning, pluginInstallPulling, pluginInstallSubmodules:
		return pluginInstallPhaseLabel(e.status), statusLabelStyle("33")
	case pluginInstallDone:
		return "done", statusLabelStyle("34")
	case pluginInstallFailed:
		return "failed", statusLabelStyle("196")
	default:
		return "queued", statusLabelStyle("241")
	}
}

func pluginInstallPhaseLabel(status pluginInstallStatus) string {
	switch status {
	case pluginInstallPreparing:
		return "preparing"
	case pluginInstallCloning:
		return "cloning"
	case pluginInstallPulling:
		return "pulling"
	case pluginInstallSubmodules:
		return "submodules"
	default:
		return "queued"
	}
}

func statusLabelStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true)
}

func pluginInstallDetailText(e pluginInstallEntry) (string, *lipgloss.Style) {
	switch e.status {
	case pluginInstallQueued:
		return "waiting to start", nil
	case pluginInstallPreparing:
		if e.plugin.Installed {
			return "preparing update", nil
		}
		return "creating plugin directory", nil
	case pluginInstallCloning:
		return "cloning repository", nil
	case pluginInstallPulling:
		return "pulling latest changes", nil
	case pluginInstallSubmodules:
		return "updating submodules", nil
	case pluginInstallDone:
		return "install complete", nil
	case pluginInstallFailed:
		if e.err != nil {
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			return shortPluginError(e.err), &style
		}
		return "install failed", nil
	default:
		return "waiting to start", nil
	}
}

func pluginInstallSourceLine(source, dir string) string {
	displayDir := pluginInstallDisplayPath(dir)
	switch {
	case source != "" && displayDir != "":
		return fmt.Sprintf("%s -> %s", source, displayDir)
	case source != "":
		return source
	case displayDir != "":
		return displayDir
	default:
		return "source unavailable"
	}
}

func pluginInstallDisplayPath(path string) string {
	if path == "" {
		return ""
	}
	path = filepath.ToSlash(path)
	for _, env := range []string{"TMPDIR", "TMP"} {
		prefix := filepath.ToSlash(os.Getenv(env))
		if prefix == "" {
			continue
		}
		if strings.HasSuffix(prefix, "/") && path == strings.TrimSuffix(prefix, "/") {
			path = "$" + env
			break
		}
		if path == prefix {
			path = "$" + env
			break
		}
		if strings.HasPrefix(path, strings.TrimRight(prefix, "/")+"/") {
			path = "$" + env + strings.TrimPrefix(path, strings.TrimRight(prefix, "/"))
			break
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.ToSlash(home)
		if path == home {
			path = "~"
		} else if strings.HasPrefix(path, home+"/") {
			path = "~" + strings.TrimPrefix(path, home)
		}
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}

func pluginInstallCellSplitLine(leftStyle, rightStyle, borderStyle, bgStyle lipgloss.Style, contentWidth int, left, right string) string {
	if contentWidth <= 0 {
		return ""
	}
	rightWidth := lipgloss.Width(right)
	statusText := pluginInstallCellStyledSegment(right, rightStyle, bgStyle)
	statusWidth := lipgloss.Width(right)
	leftWidth := contentWidth - statusWidth - 1
	if leftWidth < 0 {
		leftWidth = 0
	}
	left = truncateText(left, leftWidth)
	leftRendered := pluginInstallCellStyledSegment(left, leftStyle, bgStyle)
	spacerWidth := contentWidth - lipgloss.Width(left) - rightWidth
	if spacerWidth < 1 {
		spacerWidth = 1
	}
	interior := leftRendered + bgStyle.Render(strings.Repeat(" ", spacerWidth)) + statusText
	return borderStyle.Render("│") + interior + borderStyle.Render("│")
}

func pluginInstallCellTextLine(style, borderStyle, bgStyle lipgloss.Style, contentWidth int, text string) string {
	if contentWidth <= 0 {
		return ""
	}
	text = truncateText(text, contentWidth)
	rendered := pluginInstallCellStyledSegment(text, style, bgStyle)
	paddingWidth := contentWidth - lipgloss.Width(text)
	if paddingWidth < 0 {
		paddingWidth = 0
	}
	return borderStyle.Render("│") + rendered + bgStyle.Render(strings.Repeat(" ", paddingWidth)) + borderStyle.Render("│")
}

func pluginInstallCellStyledSegment(text string, style, bgStyle lipgloss.Style) string {
	segmentStyle := style.Copy()
	if bg := bgStyle.GetBackground(); bg != nil {
		segmentStyle = segmentStyle.Background(bg)
	}
	return segmentStyle.Render(text)
}

func (m *Model) buildPluginInstallProgressBar(s *pluginInstallState, width int) string {
	if width <= 0 {
		return ""
	}
	counter := fmt.Sprintf(" %d/%d", s.progressCurrent, s.progressTotal)
	barWidth := width - lipgloss.Width(counter) - 2
	if barWidth < 1 {
		barWidth = 1
	}

	exactFilled := 0.0
	if s.progressTotal > 0 {
		exactFilled = float64(barWidth) * float64(s.progressCurrent) / float64(s.progressTotal)
		if exactFilled > float64(barWidth) {
			exactFilled = float64(barWidth)
		}
	}
	wholeFilled := int(exactFilled)
	frac := exactFilled - float64(wholeFilled)

	type rgb struct{ r, g, b uint8 }
	var startColor, endColor rgb
	if s.operation == "update" {
		startColor = rgb{0x00, 0x87, 0xff}
		endColor = rgb{0xff, 0xff, 0xff}
	} else {
		startColor = rgb{0xff, 0xff, 0xff}
		endColor = rgb{0x00, 0x87, 0xff}
	}
	colorAt := func(i int) string {
		if barWidth <= 1 {
			return fmt.Sprintf("#%02x%02x%02x", startColor.r, startColor.g, startColor.b)
		}
		t := float64(i) / float64(barWidth-1)
		r := uint8(float64(startColor.r) + t*float64(int(endColor.r)-int(startColor.r)))
		g := uint8(float64(startColor.g) + t*float64(int(endColor.g)-int(startColor.g)))
		b := uint8(float64(startColor.b) + t*float64(int(endColor.b)-int(startColor.b)))
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}

	var bgStyle lipgloss.Style
	if styles.ProgressEmptyBg != nil {
		bgStyle = *styles.ProgressEmptyBg
	}

	eighths := []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

	var bar strings.Builder
	bar.WriteString("  ")
	for i := 0; i < wholeFilled; i++ {
		bar.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorAt(i))).Render("█"))
	}
	if wholeFilled < barWidth {
		idx := min(int(math.Round(frac*8)), 7)
		if idx > 0 {
			bar.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorAt(wholeFilled))).
				Inherit(bgStyle).
				Render(eighths[idx]))
		} else {
			bar.WriteString(bgStyle.Render(" "))
		}
		if barWidth-wholeFilled-1 > 0 {
			bar.WriteString(bgStyle.Render(strings.Repeat(" ", barWidth-wholeFilled-1)))
		}
	}
	bar.WriteString(styles.Info.Render(counter))
	rendered := bar.String()
	if lipgloss.Width(rendered) > width {
		rendered = ansi.Truncate(rendered, width, "")
	}
	return rendered
}

func pluginInstallTotalSteps(operation string, count int) int {
	if count <= 0 {
		return 1
	}
	if operation == "update" {
		return 2 + count*4
	}
	return 2 + count*3
}

func pluginInstallEmptySummary(operation string) string {
	if operation == "update" {
		return "No plugins to update"
	}
	return "No plugins to install"
}

func sourceableInstalledPlugins(pluginDir string, plugins []plugin.Plugin) []plugin.Plugin {
	if len(plugins) == 0 {
		return nil
	}
	out := make([]plugin.Plugin, len(plugins))
	for i, p := range plugins {
		if p.Dir == "" && pluginDir != "" {
			p.Dir = filepath.Join(pluginDir, p.Name)
		}
		p.Installed = true
		out[i] = p
	}
	return out
}
