package tmux

import (
	"fmt"
	"os"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// capturePaneToFileFn is the injectable var for tests.
var capturePaneToFileFn = CapturePaneToFile

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
	return os.WriteFile(filePath, []byte(output), 0644)
}

// expandFormatFn is the injectable var for tests.
var expandFormatFn = ExpandFormat

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
