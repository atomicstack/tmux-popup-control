package trace

import (
	"fmt"
	"strings"
)

const (
	controlCommandLimit       = 512
	controlOutputPreviewLines = 5
	controlOutputLineLimit    = 160
	controlOutputTotalLimit   = 512
)

// FormatControlCommand normalises a control-mode command for logging.
func FormatControlCommand(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "<empty>"
	}
	sanitised := sanitiseControlValue(raw)
	return appendTruncation(sanitised, controlCommandLimit)
}

// FormatControlLine normalises a raw control-mode line received from tmux.
func FormatControlLine(raw string) string {
	raw = strings.TrimRight(raw, "\r\n")
	if raw == "" {
		return "<empty>"
	}
	sanitised := sanitiseControlValue(raw)
	return appendTruncation(sanitised, controlCommandLimit)
}

// SummariseControlLines produces a compact preview of output lines returned by tmux.
func SummariseControlLines(lines []string) string {
	if len(lines) == 0 {
		return "lines=0"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "lines=%d: ", len(lines))

	previewCount := len(lines)
	if previewCount > controlOutputPreviewLines {
		previewCount = controlOutputPreviewLines
	}

	totalChars := 0
	truncated := false

	for idx := 0; idx < previewCount; idx++ {
		if idx > 0 {
			b.WriteString(" | ")
			totalChars += 3
		}

		line := sanitiseControlValue(strings.TrimRight(lines[idx], "\r\n"))
		if line == "" {
			line = "<empty>"
		}

		line, lineTruncated := clipText(line, controlOutputLineLimit)
		if lineTruncated {
			truncated = true
		}

		if controlOutputTotalLimit > 0 && totalChars+len(line) > controlOutputTotalLimit {
			remaining := controlOutputTotalLimit - totalChars
			if remaining <= 0 {
				truncated = true
				break
			}
			line, _ = clipText(line, remaining)
			truncated = true
		}

		b.WriteString(line)
		totalChars += len(line)

		if controlOutputTotalLimit > 0 && totalChars >= controlOutputTotalLimit {
			truncated = true
			break
		}
	}

	if len(lines) > previewCount {
		fmt.Fprintf(&b, " (+%d more lines)", len(lines)-previewCount)
		truncated = true
	}

	if truncated {
		b.WriteString(" (truncated)")
	}

	return b.String()
}

func sanitiseControlValue(raw string) string {
	raw = strings.ReplaceAll(raw, "\r", "\\r")
	raw = strings.ReplaceAll(raw, "\n", "\\n")
	return raw
}

func appendTruncation(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	out, truncated := clipText(value, limit)
	if truncated {
		return out + " (truncated)"
	}
	return out
}

func clipText(value string, limit int) (string, bool) {
	if limit <= 0 || len(value) <= limit {
		return value, false
	}
	if limit <= 3 {
		return value[:limit], true
	}
	return value[:limit-3] + "...", true
}
