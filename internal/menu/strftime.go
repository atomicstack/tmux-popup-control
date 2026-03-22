package menu

import (
	"os"
	"strings"
	"time"
)

// strftime token → Go time layout mapping.
var strftimeTokens = map[byte]string{
	'F': "2006-01-02",
	'Y': "2006",
	'm': "01",
	'd': "02",
	'H': "15",
	'M': "04",
	'S': "05",
	'T': "15:04:05",
}

// expandStrftime replaces strftime tokens (%F, %H, etc.) with formatted time
// values. unrecognised %x tokens are passed through unchanged. %% produces a
// literal %.
func expandStrftime(s string) string {
	return expandStrftimeAt(s, time.Now())
}

// expandStrftimeAt is the testable core — takes an explicit timestamp.
func expandStrftimeAt(s string, t time.Time) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			b.WriteByte('%')
			continue
		}
		next := s[i+1]
		if next == '%' {
			b.WriteByte('%')
			i++
			continue
		}
		if layout, ok := strftimeTokens[next]; ok {
			b.WriteString(t.Format(layout))
			i++
			continue
		}
		// unknown token — pass through unchanged.
		b.WriteByte('%')
	}
	return b.String()
}

// expandTilde replaces a leading ~/ with the user's home directory.
func expandTilde(s string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return s
	}
	return expandTildeWith(s, home)
}

// expandTildeWith is the testable core — takes an explicit home path.
func expandTildeWith(s, home string) string {
	if strings.HasPrefix(s, "~/") {
		return home + s[1:]
	}
	return s
}
