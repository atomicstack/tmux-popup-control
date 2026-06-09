package ui

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// gradientBarEighths are the 1/8th block characters used for the sub-cell edge
// of a gradient progress bar.
var gradientBarEighths = []string{" ", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}

// renderGradientBar renders a horizontal progress bar barWidth columns wide,
// filled to exactFilled (a fractional cell count in 0..barWidth). Filled cells
// use a per-column colour interpolated across the bar from start to end via
// lipgloss.Blend1D (CIELAB); the leading edge uses an eighth-block partial cell,
// and unfilled cells are rendered with emptyStyle. Foreground styles are
// precomputed once per column rather than per frame.
func renderGradientBar(barWidth int, exactFilled float64, start, end color.Color, emptyStyle lipgloss.Style) string {
	if barWidth <= 0 {
		return ""
	}
	exactFilled = max(exactFilled, 0)
	exactFilled = min(exactFilled, float64(barWidth))
	wholeFilled := int(exactFilled)
	frac := exactFilled - float64(wholeFilled)

	// Precompute one foreground style per column from the blended gradient.
	stops := lipgloss.Blend1D(barWidth, start, end)
	cellStyles := make([]lipgloss.Style, len(stops))
	for i, c := range stops {
		cellStyles[i] = lipgloss.NewStyle().Foreground(c)
	}
	colorStyle := func(i int) lipgloss.Style {
		if i < 0 {
			i = 0
		}
		if i >= len(cellStyles) {
			i = len(cellStyles) - 1
		}
		return cellStyles[i]
	}

	var bar strings.Builder
	for i := range wholeFilled {
		bar.WriteString(colorStyle(i).Render("█"))
	}
	if wholeFilled < barWidth {
		idx := min(int(math.Round(frac*8)), 7)
		if idx > 0 {
			bar.WriteString(colorStyle(wholeFilled).Inherit(emptyStyle).Render(gradientBarEighths[idx]))
		} else {
			bar.WriteString(emptyStyle.Render(" "))
		}
		if rest := barWidth - wholeFilled - 1; rest > 0 {
			bar.WriteString(emptyStyle.Render(strings.Repeat(" ", rest)))
		}
	}
	return bar.String()
}
