package tmux

import (
	"fmt"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

const panePreviewDefaultLines = 40

// PanePreview captures the contents of a pane for display via control-mode.
func PanePreview(socketPath, pane string) ([]string, error) {
	target := strings.TrimSpace(pane)
	if target == "" {
		return nil, fmt.Errorf("pane target required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	output, err := client.CapturePane(target, &gotmux.CaptureOptions{
		EscTxtNBgAttr: true,
		StartLine:     fmt.Sprintf("-%d", panePreviewDefaultLines),
	})
	if err != nil {
		return nil, fmt.Errorf("capture-pane %s: %w", target, err)
	}
	lines := splitPreviewLines(output, true)
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
