package resurrect

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// RelativeTime returns a concise human-readable relative timestamp like
// "just now", "5m ago", "2h ago", "yesterday", or "3 days ago".
func RelativeTime(t, now time.Time) string {
	d := max(now.Sub(t), time.Duration(0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		switch {
		case days == 1:
			return "yesterday"
		case days < 30:
			return fmt.Sprintf("%dd ago", days)
		case days < 365:
			months := days / 30
			if months == 1 {
				return "1 month ago"
			}
			return fmt.Sprintf("%d months ago", months)
		default:
			years := days / 365
			if years == 1 {
				return "1 year ago"
			}
			return fmt.Sprintf("%d years ago", years)
		}
	}
}

// DisplayName returns a display-friendly name for a save entry. Named saves
// return the name as-is. Unnamed saves (UUID filenames) return a truncated
// 8-character UUID prefix.
func (e SaveEntry) DisplayName() string {
	if e.Name != "" {
		return e.Name
	}
	// extract UUID from filename: UUID_TIMESTAMP.json
	base := filepath.Base(e.Path)
	base = strings.TrimSuffix(base, ".json")
	// UUID is 36 chars (8-4-4-4-12); take the first 8 as short ID
	if len(base) >= 8 {
		return base[:8]
	}
	return base
}
