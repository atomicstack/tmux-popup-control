package ui

import "charm.land/lipgloss/v2"

// scrollbarStyle is the lipgloss style used to render scrollbar thumb/track
// cells. Matches the grey used for list trimmings and borders (color 240).
var scrollbarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// renderScrollbar returns a slice of runes (length = visible) representing a
// vertical scrollbar for a list of `total` items whose visible window starts
// at `start`. The thumb length is proportional to visible/total (clamped to
// at least 1), and its top position corresponds to `start` within
// [0, total-visible].
//
// Returns nil when total <= visible — the caller should render no scrollbar
// in that case.
func renderScrollbar(total, visible, start int) []rune {
	if total <= 0 || visible <= 0 || total <= visible {
		return nil
	}
	maxStart := total - visible
	start = min(max(start, 0), maxStart)

	thumbLen := visible * visible / total
	thumbLen = min(max(thumbLen, 1), visible)

	maxThumbStart := visible - thumbLen
	thumbStart := 0
	if maxStart > 0 {
		thumbStart = start * maxThumbStart / maxStart
	}
	thumbStart = min(max(thumbStart, 0), maxThumbStart)

	const (
		thumbRune = '│' // thin vertical line (U+2502)
		trackRune = ' '
	)
	cells := make([]rune, visible)
	for i := range cells {
		if i >= thumbStart && i < thumbStart+thumbLen {
			cells[i] = thumbRune
		} else {
			cells[i] = trackRune
		}
	}
	return cells
}
