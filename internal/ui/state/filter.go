package state

import (
	"strings"
	"unicode"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

// SetFilter updates the filter query and cursor position.
func (l *Level) SetFilter(query string, cursor int) {
	trimmed := strings.TrimSpace(query)
	prevTrimmed := strings.TrimSpace(l.Filter)
	restore := -1
	l.Filter = query
	runes := []rune(l.Filter)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	l.FilterCursor = cursor
	if trimmed != "" {
		if prevTrimmed == "" {
			l.LastCursor = l.Cursor
		}
		l.Cursor = 0
	} else if prevTrimmed != "" {
		restore = l.LastCursor
	}
	l.applyFilter()
	if trimmed != "" && len(l.Items) > 0 {
		if idx := BestMatchIndex(l.Items, trimmed); idx >= 0 {
			l.Cursor = idx
		}
	}
	if trimmed == "" && prevTrimmed != "" {
		if restore >= 0 && restore < len(l.Items) {
			l.Cursor = restore
		} else if len(l.Items) > 0 {
			l.Cursor = len(l.Items) - 1
		}
		l.LastCursor = -1
	}
}

func (l *Level) applyFilter() {
	l.Items = FilterItems(l.Full, l.Filter)
	if len(l.Items) == 0 {
		l.Cursor = 0
		l.ViewportOffset = 0
		return
	}
	if l.Cursor < 0 {
		l.Cursor = len(l.Items) - 1
		return
	}
	if l.Cursor >= len(l.Items) {
		l.Cursor = len(l.Items) - 1
	}
	if l.ViewportOffset > len(l.Items)-1 {
		l.ViewportOffset = 0
	}
}

// FilterCursorPos returns the rune offset of the filter cursor.
func (l *Level) FilterCursorPos() int {
	runes := []rune(l.Filter)
	if l.FilterCursor < 0 {
		return 0
	}
	if l.FilterCursor > len(runes) {
		return len(runes)
	}
	return l.FilterCursor
}

// InsertFilterText inserts text into the filter at the cursor position.
func (l *Level) InsertFilterText(text string) bool {
	if text == "" {
		return false
	}
	insert := []rune(text)
	if len(insert) == 0 {
		return false
	}
	runes := []rune(l.Filter)
	pos := l.FilterCursorPos()
	updated := make([]rune, 0, len(runes)+len(insert))
	updated = append(updated, runes[:pos]...)
	updated = append(updated, insert...)
	updated = append(updated, runes[pos:]...)
	l.SetFilter(string(updated), pos+len(insert))
	return true
}

// DeleteFilterRuneBackward deletes a rune before the filter cursor.
func (l *Level) DeleteFilterRuneBackward() bool {
	runes := []rune(l.Filter)
	pos := l.FilterCursorPos()
	if pos == 0 || len(runes) == 0 {
		return false
	}
	updated := append(runes[:pos-1], runes[pos:]...)
	l.SetFilter(string(updated), pos-1)
	return true
}

// DeleteFilterWordBackward deletes the word preceding the cursor.
func (l *Level) DeleteFilterWordBackward() bool {
	runes := []rune(l.Filter)
	pos := l.FilterCursorPos()
	if pos == 0 || len(runes) == 0 {
		return false
	}
	i := pos
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	updated := append(runes[:i], runes[pos:]...)
	l.SetFilter(string(updated), i)
	return true
}

// MoveFilterCursorStart moves the filter cursor to the start.
func (l *Level) MoveFilterCursorStart() bool {
	if l.FilterCursorPos() == 0 {
		return false
	}
	l.FilterCursor = 0
	return true
}

// MoveFilterCursorEnd moves the filter cursor to the end.
func (l *Level) MoveFilterCursorEnd() bool {
	end := len([]rune(l.Filter))
	if l.FilterCursorPos() == end {
		return false
	}
	l.FilterCursor = end
	return true
}

// MoveFilterCursorWordBackward moves the filter cursor one word backward.
func (l *Level) MoveFilterCursorWordBackward() bool {
	runes := []rune(l.Filter)
	pos := l.FilterCursorPos()
	if pos == 0 || len(runes) == 0 {
		return false
	}
	i := pos
	for i > 0 && unicode.IsSpace(runes[i-1]) {
		i--
	}
	for i > 0 && !unicode.IsSpace(runes[i-1]) {
		i--
	}
	if i == pos {
		return false
	}
	l.FilterCursor = i
	return true
}

// MoveFilterCursorWordForward moves the filter cursor one word forward.
func (l *Level) MoveFilterCursorWordForward() bool {
	runes := []rune(l.Filter)
	pos := l.FilterCursorPos()
	if pos >= len(runes) {
		return false
	}
	i := pos
	for i < len(runes) && !unicode.IsSpace(runes[i]) {
		i++
	}
	for i < len(runes) && unicode.IsSpace(runes[i]) {
		i++
	}
	if i == pos {
		return false
	}
	l.FilterCursor = i
	return true
}

// MoveFilterCursorRuneBackward moves the filter cursor one rune backward.
func (l *Level) MoveFilterCursorRuneBackward() bool {
	if l.FilterCursorPos() == 0 {
		return false
	}
	l.FilterCursor = l.FilterCursorPos() - 1
	return true
}

// MoveFilterCursorRuneForward moves the filter cursor one rune forward.
func (l *Level) MoveFilterCursorRuneForward() bool {
	runes := []rune(l.Filter)
	pos := l.FilterCursorPos()
	if pos >= len(runes) {
		return false
	}
	l.FilterCursor = pos + 1
	return true
}

// FilterItems returns items matching the supplied filter string.
func FilterItems(items []menu.Item, query string) []menu.Item {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return CloneItems(items)
	}
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	ranks := fuzzy.RankFindNormalizedFold(trimmed, labels)
	if len(ranks) > 0 {
		matches := make(map[int]struct{}, len(ranks))
		for _, rank := range ranks {
			matches[rank.OriginalIndex] = struct{}{}
		}
		filtered := make([]menu.Item, 0, len(matches))
		for idx, item := range items {
			if _, ok := matches[idx]; ok {
				filtered = append(filtered, item)
			}
		}
		if len(filtered) > 0 {
			return CloneItems(filtered)
		}
	}
	lower := strings.ToLower(trimmed)
	filtered := make([]menu.Item, 0, len(items))
	for _, item := range items {
		labelLower := strings.ToLower(item.Label)
		idLower := strings.ToLower(item.ID)
		if strings.Contains(labelLower, lower) || strings.Contains(idLower, lower) {
			filtered = append(filtered, item)
		}
	}
	return CloneItems(filtered)
}

// BestMatchIndex returns the best index for the query among the provided items.
func BestMatchIndex(items []menu.Item, query string) int {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		if len(items) == 0 {
			return -1
		}
		return 0
	}
	lower := strings.ToLower(trimmed)
	for i, item := range items {
		if strings.EqualFold(item.Label, trimmed) || strings.EqualFold(item.ID, trimmed) {
			return i
		}
	}
	for i, item := range items {
		if strings.HasPrefix(strings.ToLower(item.Label), lower) {
			return i
		}
	}
	for i, item := range items {
		if strings.HasPrefix(strings.ToLower(item.ID), lower) {
			return i
		}
	}
	for i, item := range items {
		if strings.Contains(strings.ToLower(item.ID), lower) {
			return i
		}
	}
	for i, item := range items {
		if strings.Contains(strings.ToLower(item.Label), lower) {
			return i
		}
	}
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	ranks := fuzzy.RankFindNormalizedFold(trimmed, labels)
	if len(ranks) == 0 {
		if len(items) == 0 {
			return -1
		}
		return 0
	}
	best := ranks[0]
	for _, rank := range ranks[1:] {
		if rank.Distance < best.Distance {
			best = rank
			continue
		}
		if rank.Distance == best.Distance && rank.OriginalIndex < best.OriginalIndex {
			best = rank
		}
	}
	if best.OriginalIndex < 0 || best.OriginalIndex >= len(items) {
		if len(items) == 0 {
			return -1
		}
		return 0
	}
	return best.OriginalIndex
}
