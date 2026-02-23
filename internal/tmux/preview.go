package tmux

import (
	"fmt"
	"regexp"
	"strings"
)

const panePreviewDefaultLines = 40

// ansiEscapeRe matches ANSI/VT100 escape sequences so they can be stripped
// before the captured pane content is shown in the text preview panel.
var ansiEscapeRe = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[A-Za-z]|[A-Za-z=><\\])`)

// PanePreview captures the contents of a pane for display using a direct
// tmux subprocess call (rather than the control-mode transport) so that the
// raw pane content is retrieved reliably regardless of any control-mode
// quirks with capture-pane.
func PanePreview(socketPath, pane string) ([]string, error) {
	target := strings.TrimSpace(pane)
	if target == "" {
		return nil, fmt.Errorf("pane target required")
	}
	args := append(baseArgs(socketPath), "capture-pane", "-p", "-t", target, "-S", fmt.Sprintf("-%d", panePreviewDefaultLines))
	out, err := runExecCommand("tmux", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("capture-pane %s: %w", target, err)
	}
	text := ansiEscapeRe.ReplaceAllString(string(out), "")
	lines := splitPreviewLines(text, true)
	if len(lines) == 0 {
		return []string{"(pane is empty)"}, nil
	}
	if len(lines) > panePreviewDefaultLines {
		lines = lines[len(lines)-panePreviewDefaultLines:]
	}
	return lines, nil
}

func splitPreviewLines(text string, keepEmpty bool) []string {
	if text == "" {
		return nil
	}
	normalised := strings.ReplaceAll(text, "\r\n", "\n")
	normalised = strings.ReplaceAll(normalised, "\r", "\n")
	normalised = strings.TrimRight(normalised, "\n")
	if normalised == "" {
		return nil
	}
	raw := strings.Split(normalised, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "" && !keepEmpty {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}
