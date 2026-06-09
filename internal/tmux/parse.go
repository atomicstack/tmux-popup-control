package tmux

import (
	"strconv"
	"strings"
)

// splitTabLine trims the line, splits it into at most n tab-separated fields,
// and trims each field. It returns nil for a blank line so callers can skip it
// with a simple len check.
func splitTabLine(line string, n int) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	parts := strings.SplitN(line, "\t", n)
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// atoiOr0 parses s as a base-10 int, returning 0 when it cannot be parsed.
func atoiOr0(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

// fetchFormattedLines runs a tmux list-* format command and returns the
// non-blank rows split into at most splitN trimmed tab-separated fields. Rows
// with fewer than minFields fields are dropped; remaining rows are guaranteed
// to have at least minFields entries (but may have up to splitN, so callers
// must bounds-check any optional trailing fields).
func fetchFormattedLines(listFn func(filter, format string) ([]string, error), filter, format string, splitN, minFields int) ([][]string, error) {
	rawLines, err := listFn(filter, format)
	if err != nil {
		return nil, err
	}
	result := make([][]string, 0, len(rawLines))
	for _, line := range rawLines {
		parts := splitTabLine(line, splitN)
		if len(parts) < minFields {
			continue
		}
		result = append(result, parts)
	}
	return result, nil
}
