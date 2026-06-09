package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// resurrectView renders the progress UI for a save or restore operation.
func (m *Model) resurrectView() string {
	s := m.resurrectState
	if s == nil {
		return ""
	}

	const margin = 1

	// inner dimensions after 1-cell margin on each side
	innerWidth := m.width - 2*margin
	innerWidth = max(innerWidth, 10)
	innerHeight := m.height - 2*margin
	innerHeight = max(innerHeight, 3)

	// last row is the progress bar; everything above is the log
	logHeight := innerHeight - 1
	logHeight = max(logHeight, 1)

	// ── log area ────────────────────────────────────────────────────────────

	logLines := buildResurrectLogLines(s.log, s.total)

	// show only the last logHeight lines
	if len(logLines) > logHeight {
		logLines = logLines[len(logLines)-logHeight:]
	}
	// pad with blank lines at the top
	for len(logLines) < logHeight {
		logLines = append([]string{""}, logLines...)
	}

	var b strings.Builder
	for _, line := range logLines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	// ── progress bar ────────────────────────────────────────────────────────

	b.WriteString(m.buildResurrectProgressBar(s, innerWidth))

	return lipgloss.NewStyle().Padding(margin).Render(b.String())
}

// buildResurrectLogLines returns styled log lines for the given entries.
func buildResurrectLogLines(entries []logEntry, total int) []string {
	if len(entries) == 0 && total == 0 {
		// discovering phase — no events yet
		return []string{
			lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true).Render("discovering..."),
		}
	}

	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		line := styledResurrectLine(e)
		lines = append(lines, line)
	}
	return lines
}

// styledResurrectLine applies hierarchical blue colouring based on entry kind.
// session uses colour 33 (#0087ff), window and pane use progressively
// desaturated variants of the same hue.
func styledResurrectLine(e logEntry) string {
	switch e.kind {
	case "error":
		if styles.Error != nil {
			return styles.Error.Render(e.message)
		}
		return e.message
	case "session":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#0087ff")).Render(e.message)
	case "window":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#3d8fe0")).Render(e.message)
	case "pane":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#5c9bd5")).Render(e.message)
	default:
		// "info" and anything else — no special colouring
		return e.message
	}
}

// buildResurrectProgressBar renders the gradient progress bar line.
// availWidth is the usable width inside margins.
func (m *Model) buildResurrectProgressBar(s *resurrectState, availWidth int) string {
	// counter is " N/N" — reserve space for it
	counterWidth := len(fmt.Sprintf(" %d/%d", s.step, s.total))
	barWidth := availWidth - counterWidth
	barWidth = max(barWidth, 10)

	// sub-cell precision: use the smoothed displayStep for the bar fill
	exactFilled := 0.0
	if s.total > 0 {
		exactFilled = float64(barWidth) * s.displayStep / float64(s.total)
		exactFilled = min(exactFilled, float64(barWidth))
	}

	// gradient colours (colour 33 = #0087ff)
	// save:    white #ffffff → blue #0087ff
	// restore: blue #0087ff → white #ffffff
	startColor := lipgloss.Color("#ffffff")
	endColor := lipgloss.Color("#0087ff")
	if s.operation == "restore" {
		startColor, endColor = endColor, startColor
	}

	// background style for unfilled cells
	var bgStyle lipgloss.Style
	if styles.ProgressEmptyBg != nil {
		bgStyle = *styles.ProgressEmptyBg
	}

	bar := renderGradientBar(barWidth, exactFilled, startColor, endColor, bgStyle)

	// counter: step in #0087ff, "/" dim, total in #777777
	stepStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#0087ff")).Render(fmt.Sprintf("%d", s.step))
	sep := lipgloss.NewStyle().Faint(true).Render("/")
	totalStr := lipgloss.NewStyle().Foreground(lipgloss.Color("#777777")).Render(fmt.Sprintf("%d", s.total))
	counter := " " + stepStr + sep + totalStr

	return bar + counter
}
