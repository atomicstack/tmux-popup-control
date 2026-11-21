package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// YNPromptMarker ends the text of a y/n prompt, e.g. "remove plugin foo?
// [y/n]". renderYNPrompt replaces it with the yes / no buttons.
const YNPromptMarker = "[y/n]"

// ynPromptRows is the height of a rendered y/n prompt: the buttons are
// bordered, so the question sits on the middle row of three.
const ynPromptRows = 3

// ynChoice is one of the two buttons of a y/n prompt (or neither).
type ynChoice int

const (
	ynNone ynChoice = iota
	ynYes
	ynNo
)

var (
	ynPromptYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	ynPromptGreen  = lipgloss.Color("34")
	ynPromptRed    = lipgloss.Color("160")
	ynPromptWhite  = lipgloss.Color("231")
)

// The text of each button's middle row. locateYNButtons finds the buttons in
// the rendered screen by these, so every prompt is clickable wherever its
// caller placed it.
const (
	ynButtonPad = 2
	ynYesWord   = "yes"
	ynNoWord    = "no"
)

var (
	ynYesFace = buttonFace(ynYesWord)
	ynNoFace  = buttonFace(ynNoWord)
)

func buttonFace(word string) string {
	pad := strings.Repeat(" ", ynButtonPad)
	border := lipgloss.NormalBorder()
	return border.Left + pad + word + pad + border.Right
}

// renderYNButton draws one square-bordered button. The first letter is
// underlined as the keyboard hint; hot fills the button, for mouse hover.
func renderYNButton(word string, colour color.Color, hot bool) string {
	inner := lipgloss.NewStyle().Foreground(colour).Bold(true)
	if hot {
		inner = lipgloss.NewStyle().Background(colour).Foreground(ynPromptWhite).Bold(true)
	}
	pad := strings.Repeat(" ", ynButtonPad)
	label := inner.Render(pad) + inner.Underline(true).Render(word[:1]) + inner.Render(word[1:]+pad)
	return lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colour).Render(label)
}

// renderYNPrompt renders a prompt that ends in "[y/n]" as ynPromptRows rows:
// the question in yellow beside a green yes button and a red no button, each
// three rows tall so they are easy to click. hover names the button under the
// mouse. It returns nil for empty text.
//
// This is the shared helper for every y/n prompt in the UI; reach for it
// rather than rolling fresh ANSI/Lipgloss in any new flow. The y and n keys
// answer the prompt as before; handleMouseClick answers it from a click.
func renderYNPrompt(text string, hover ynChoice) []string {
	if text == "" {
		return nil
	}
	question := strings.TrimRight(strings.TrimSuffix(text, YNPromptMarker), " ")
	block := lipgloss.JoinHorizontal(lipgloss.Center,
		ynPromptYellow.Render(question+"  "),
		renderYNButton(ynYesWord, ynPromptGreen, hover == ynYes),
		" ",
		renderYNButton(ynNoWord, ynPromptRed, hover == ynNo),
	)
	return strings.Split(block, "\n")
}

// ynPromptLines is renderYNPrompt as raw styledLines.
func ynPromptLines(text string, hover ynChoice) []styledLine {
	rows := renderYNPrompt(text, hover)
	lines := make([]styledLine, len(rows))
	for i, row := range rows {
		lines[i] = styledLine{text: row, raw: true}
	}
	return lines
}

// ynButtons is where a y/n prompt's buttons landed on screen.
type ynButtons struct {
	yes, no hitRect
}

func (b ynButtons) at(x, y int) ynChoice {
	switch {
	case b.yes.contains(x, y):
		return ynYes
	case b.no.contains(x, y):
		return ynNo
	}
	return ynNone
}

// locateYNButtons finds the lowest y/n prompt in rendered screen content and
// returns the cells its two buttons occupy. ok is false when no complete
// prompt is on screen (for example when it was truncated to fit).
func locateYNButtons(content string) (ynButtons, bool) {
	lines := strings.Split(content, "\n")
	for y := len(lines) - 1; y >= 1; y-- {
		plain := ansi.Strip(lines[y])
		yesAt := strings.Index(plain, ynYesFace)
		if yesAt < 0 {
			continue
		}
		noAt := strings.Index(plain[yesAt:], ynNoFace)
		if noAt < 0 {
			continue
		}
		noAt += yesAt
		rect := func(at int, face string) hitRect {
			x := ansi.StringWidth(plain[:at])
			return hitRect{x0: x, y0: y - 1, x1: x + ansi.StringWidth(face), y1: y + 2}
		}
		return ynButtons{yes: rect(yesAt, ynYesFace), no: rect(noAt, ynNoFace)}, true
	}
	return ynButtons{}, false
}
