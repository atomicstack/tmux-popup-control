package tmux

import (
	"fmt"
	"strconv"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
	"github.com/charmbracelet/x/ansi"
)

const panePreviewDefaultLines = 40

type PanePreviewData struct {
	Lines         []string
	RawANSI       bool
	CursorVisible bool
	CursorX       int
	CursorY       int
}

// PanePreview captures the contents of a pane for display via control-mode.
func PanePreview(socketPath, pane string) (PanePreviewData, error) {
	target := strings.TrimSpace(pane)
	if target == "" {
		return PanePreviewData{}, fmt.Errorf("pane target required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return PanePreviewData{}, err
	}
	result := PanePreviewData{
		RawANSI: true,
	}
	cursorX, cursorY, _, ok := paneCursorPosition(client, target)
	output, err := client.CapturePane(target, &gotmux.CaptureOptions{
		EscTxtNBgAttr: true,
		StartLine:     "0",
	})
	if err != nil {
		return PanePreviewData{}, fmt.Errorf("capture-pane %s: %w", target, err)
	}
	rawLines := splitPreviewLinesPreserveTrailing(output)
	if !ok {
		result.Lines = trimPreviewLines(rawLines, -1)
		if len(result.Lines) == 0 {
			result.Lines = []string{"(pane is empty)"}
			return result, nil
		}
		if len(result.Lines) > panePreviewDefaultLines {
			result.Lines = result.Lines[len(result.Lines)-panePreviewDefaultLines:]
		}
		return result, nil
	}
	if len(rawLines) == 0 {
		rawLines = []string{""}
	}
	if needed := cursorY + 1; needed > len(rawLines) {
		padding := make([]string, needed-len(rawLines))
		rawLines = append(rawLines, padding...)
	}
	previewEnd := lastMeaningfulPreviewLine(rawLines) + 1
	if cursorY+1 > previewEnd {
		previewEnd = cursorY + 1
	}
	if previewEnd <= 0 {
		previewEnd = 1
	}
	if previewEnd > len(rawLines) {
		previewEnd = len(rawLines)
	}
	cursorRow := cursorY
	if previewEnd > panePreviewDefaultLines {
		previewStart := previewEnd - panePreviewDefaultLines
		rawLines = rawLines[previewStart:previewEnd]
		cursorRow -= previewStart
	} else {
		rawLines = rawLines[:previewEnd]
	}
	result.Lines = trimPreviewLines(rawLines, cursorRow)
	if cursorRow < 0 || cursorRow >= len(result.Lines) {
		return result, nil
	}
	result.CursorVisible = true
	result.CursorX = cursorX
	result.CursorY = cursorRow
	return result, nil
}

func paneCursorPosition(client tmuxClient, target string) (int, int, int, bool) {
	if client == nil {
		return 0, 0, 0, false
	}
	value, err := client.DisplayMessage(target, "#{cursor_x},#{cursor_y},#{pane_height}")
	if err != nil {
		return 0, 0, 0, false
	}
	parts := strings.Split(strings.TrimSpace(value), ",")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	cursorX, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, 0, false
	}
	cursorY, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, 0, false
	}
	paneHeight, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return 0, 0, 0, false
	}
	if cursorX < 0 || cursorY < 0 || paneHeight <= 0 {
		return 0, 0, 0, false
	}
	return cursorX, cursorY, paneHeight, true
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

func splitPreviewLinesPreserveTrailing(text string) []string {
	if text == "" {
		return nil
	}
	normalised := strings.ReplaceAll(text, "\r\n", "\n")
	normalised = strings.ReplaceAll(normalised, "\r", "\n")
	raw := strings.Split(normalised, "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		lines = append(lines, strings.TrimRight(line, " \t"))
	}
	return lines
}

func trimPreviewLines(lines []string, cursorRow int) []string {
	if len(lines) == 0 {
		return nil
	}
	keepUntil := 0
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			keepUntil = i + 1
			break
		}
	}
	if cursorRow >= 0 && cursorRow+1 > keepUntil {
		keepUntil = cursorRow + 1
	}
	if keepUntil <= 0 {
		return nil
	}
	return lines[:keepUntil]
}

func lastMeaningfulPreviewLine(lines []string) int {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(ansi.Strip(lines[i])) != "" {
			return i
		}
	}
	return -1
}
