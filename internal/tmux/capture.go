package tmux

import (
	"fmt"
	"os"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// CapturePaneToFile captures the full scrollback of a pane and writes it to a
// file. escSeqs controls whether ANSI escape sequences are included (-e flag).
func CapturePaneToFile(socketPath, paneTarget, filePath string, escSeqs bool) error {
	target := strings.TrimSpace(paneTarget)
	if target == "" {
		return fmt.Errorf("pane target required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	output, err := client.CapturePane(target, &gotmux.CaptureOptions{
		EscTxtNBgAttr: escSeqs,
		StartLine:     "-",
	})
	if err != nil {
		return fmt.Errorf("capture-pane %s: %w", target, err)
	}
	cleaned := trimCaptureOutput(output)
	return os.WriteFile(filePath, []byte(cleaned), 0644)
}

// trimCaptureOutput removes trailing whitespace and blank lines from the end
// of the captured output so the file doesn't end with endless empty space.
func trimCaptureOutput(s string) string {
	trimmed := strings.TrimRight(s, " \t\n\r")
	if trimmed == "" {
		return ""
	}
	return trimmed + "\n"
}

// ExpandFormat resolves a tmux format string against a target pane via
// display-message. Used for the live filename preview.
func ExpandFormat(socketPath, target, format string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	result, err := client.DisplayMessage(target, format)
	if err != nil {
		return "", fmt.Errorf("expand format %q: %w", format, err)
	}
	return strings.TrimSpace(result), nil
}
