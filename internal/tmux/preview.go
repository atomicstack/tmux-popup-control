package tmux

import (
	"fmt"
	"strings"
)

const panePreviewDefaultLines = 40

var (
	sessionPreviewFormat = "#{?window_active,*, } #{window_index}: #{window_name}"
	windowPreviewFormat  = "#{?pane_active,*, } #{pane_index}: #{pane_title} (#{pane_current_command})"
)

// SessionPreview returns a textual description of the windows that belong to a session.
func SessionPreview(socketPath, session string) ([]string, error) {
	target := strings.TrimSpace(session)
	if target == "" {
		return nil, fmt.Errorf("session name required")
	}
	args := append(baseArgs(socketPath), "list-windows", "-t", target, "-F", sessionPreviewFormat)
	cmd := runExecCommand("tmux", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list-windows %s: %w", target, err)
	}
	lines := splitPreviewLines(string(output), false)
	if len(lines) == 0 {
		return []string{"(no windows)"}, nil
	}
	return lines, nil
}

// WindowPreview returns a textual description of the panes contained in a window.
func WindowPreview(socketPath, window string) ([]string, error) {
	target := strings.TrimSpace(window)
	if target == "" {
		return nil, fmt.Errorf("window target required")
	}
	args := append(baseArgs(socketPath), "list-panes", "-t", target, "-F", windowPreviewFormat)
	cmd := runExecCommand("tmux", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list-panes %s: %w", target, err)
	}
	lines := splitPreviewLines(string(output), false)
	if len(lines) == 0 {
		return []string{"(no panes)"}, nil
	}
	return lines, nil
}

// PanePreview captures the contents of a pane for display.
func PanePreview(socketPath, pane string) ([]string, error) {
	target := strings.TrimSpace(pane)
	if target == "" {
		return nil, fmt.Errorf("pane target required")
	}
	args := append(baseArgs(socketPath), "capture-pane", "-ep", "-S", fmt.Sprintf("-%d", panePreviewDefaultLines), "-t", target)
	cmd := runExecCommand("tmux", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("capture-pane %s: %w", target, err)
	}
	lines := splitPreviewLines(string(output), true)
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
