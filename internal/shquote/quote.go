package shquote

import "strings"

// Quote wraps s in single quotes, escaping embedded single quotes for sh.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// JoinCommand quotes argv as a shell-safe command string.
func JoinCommand(argv ...string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		quoted = append(quoted, Quote(arg))
	}
	return strings.Join(quoted, " ")
}
