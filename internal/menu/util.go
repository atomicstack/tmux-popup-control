package menu

import (
	"bufio"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// styleFormInput applies consistent prompt and cursor styling to a textinput,
// matching the filter prompt's coloured chevron and cursor.
func styleFormInput(ti *textinput.Model) {
	ti.Prompt = "» "
	s := ti.Styles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	s.Cursor.Color = lipgloss.Color("33")
	ti.SetStyles(s)
}

func splitLines(input string) []string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
