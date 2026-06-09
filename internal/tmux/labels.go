package tmux

import (
	"fmt"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func fetchSessionLabels(client tmuxClient, envFormat string) map[string]string {
	labelExpr := strings.TrimSpace(envFormat)
	if labelExpr != "" {
		labelExpr = fmt.Sprintf("#S: %s", labelExpr)
	} else {
		labelExpr = defaultSessionFormat
	}
	format := fmt.Sprintf("#{session_name}\t%s", labelExpr)
	lines, err := client.ListSessionsFormat(format)
	if err != nil {
		return map[string]string{}
	}
	labels := make(map[string]string, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		namePart, labelPart, _ := strings.Cut(line, "\t")
		name := strings.TrimSpace(namePart)
		if name == "" {
			continue
		}
		label := name
		if trimmed := strings.TrimSpace(labelPart); trimmed != "" {
			label = trimmed
		}
		labels[name] = label
	}
	return labels
}

func defaultLabelForSession(s *gotmux.Session) string {
	label := fmt.Sprintf("%s: %d window", s.Name, s.Windows)
	if s.Windows != 1 {
		label += "s"
	}
	if s.Attached > 0 {
		label += " (attached)"
	}
	return label
}
