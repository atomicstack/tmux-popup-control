package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// YNPromptMarker is the trailing marker that signals where to colour the
// y/n choice. Build prompts like "remove plugin foo? [y/n]" and pass them
// through renderYNPrompt to apply the project's standard scheme: yellow
// surround, green y, red n. New y/n prompts MUST use this helper for
// visual consistency.
const YNPromptMarker = "[y/n]"

var (
	ynPromptYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	ynPromptGreen  = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	ynPromptRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

// renderYNPrompt paints a prompt that ends in "[y/n]" — surrounding text
// in yellow, the y in green, the n in red. Returns the input unchanged
// (rendered yellow only) when the marker isn't found, or "" when input
// is "".
//
// This is the shared helper for every y/n prompt in the UI; reach for it
// rather than rolling fresh ANSI/Lipgloss in any new flow.
func renderYNPrompt(text string) string {
	if text == "" {
		return ""
	}
	idx := strings.LastIndex(text, YNPromptMarker)
	if idx < 0 {
		return ynPromptYellow.Render(text)
	}
	prefix := text[:idx]
	return ynPromptYellow.Render(prefix+"[") +
		ynPromptGreen.Render("y") +
		ynPromptYellow.Render("/") +
		ynPromptRed.Render("n") +
		ynPromptYellow.Render("]")
}
