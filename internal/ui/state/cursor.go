package state

// MoveCursorHome moves the cursor to the first selectable item.
func (l *Level) MoveCursorHome() bool {
	if len(l.Items) == 0 {
		l.Cursor = 0
		return false
	}
	old := l.Cursor
	l.Cursor = 0
	l.SkipHeaders(1)
	return old != l.Cursor
}

// MoveCursorEnd moves the cursor to the last selectable item.
func (l *Level) MoveCursorEnd() bool {
	n := len(l.Items)
	if n == 0 {
		l.Cursor = 0
		return false
	}
	old := l.Cursor
	l.Cursor = n - 1
	l.SkipHeaders(-1)
	return old != l.Cursor
}

// MoveCursorPageUp moves the cursor up by the given page size.
func (l *Level) MoveCursorPageUp(maxVisible int) bool {
	return l.moveCursorBy(-l.pageSize(maxVisible))
}

// MoveCursorPageDown moves the cursor down by the given page size.
func (l *Level) MoveCursorPageDown(maxVisible int) bool {
	return l.moveCursorBy(l.pageSize(maxVisible))
}

func (l *Level) moveCursorBy(delta int) bool {
	if len(l.Items) == 0 {
		l.Cursor = 0
		return false
	}
	old := l.Cursor
	if l.Cursor < 0 {
		l.Cursor = 0
	}
	l.Cursor += delta
	if l.Cursor < 0 {
		l.Cursor = 0
	}
	if l.Cursor >= len(l.Items) {
		l.Cursor = len(l.Items) - 1
	}
	dir := 1
	if delta < 0 {
		dir = -1
	}
	l.SkipHeaders(dir)
	return l.Cursor != old
}

// SkipHeaders advances the cursor past any header items in the given direction.
// If no selectable item exists in that direction, it searches the opposite way.
func (l *Level) SkipHeaders(dir int) {
	n := len(l.Items)
	if n == 0 {
		return
	}
	for l.Cursor >= 0 && l.Cursor < n && l.Items[l.Cursor].Header {
		l.Cursor += dir
	}
	if l.Cursor < 0 || l.Cursor >= n || l.Items[l.Cursor].Header {
		// hit boundary while still on a header; search opposite direction
		if l.Cursor < 0 {
			l.Cursor = 0
		}
		if l.Cursor >= n {
			l.Cursor = n - 1
		}
		opp := -dir
		for l.Cursor >= 0 && l.Cursor < n && l.Items[l.Cursor].Header {
			l.Cursor += opp
		}
		if l.Cursor < 0 {
			l.Cursor = 0
		}
		if l.Cursor >= n {
			l.Cursor = n - 1
		}
	}
}

func (l *Level) pageSize(maxVisible int) int {
	total := len(l.Items)
	if total == 0 {
		return 0
	}
	size := maxVisible
	if size <= 0 || size > total {
		size = total
	}
	if size < 1 {
		size = 1
	}
	return size
}

// EnsureCursorVisible adjusts the viewport offset so the cursor stays visible.
func (l *Level) EnsureCursorVisible(maxVisible int) {
	if len(l.Items) == 0 {
		l.Cursor = 0
		l.ViewportOffset = 0
		return
	}
	if l.Cursor < 0 {
		l.Cursor = 0
	}
	if l.Cursor >= len(l.Items) {
		l.Cursor = len(l.Items) - 1
	}
	if maxVisible <= 0 {
		l.ViewportOffset = 0
		return
	}
	maxOffset := len(l.Items) - maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if l.ViewportOffset > maxOffset {
		l.ViewportOffset = maxOffset
	}
	if l.ViewportOffset < 0 {
		l.ViewportOffset = 0
	}
	if l.Cursor < l.ViewportOffset {
		l.ViewportOffset = l.Cursor
	}
	upper := l.ViewportOffset + maxVisible - 1
	if l.Cursor > upper {
		l.ViewportOffset = l.Cursor - maxVisible + 1
		if l.ViewportOffset < 0 {
			l.ViewportOffset = 0
		}
		if l.ViewportOffset > maxOffset {
			l.ViewportOffset = maxOffset
		}
	}
}

// EnsureCursorVisibleWithAnchor adjusts the viewport offset so the cursor stays
// visible and, when possible, appears at the supplied row within the viewport.
func (l *Level) EnsureCursorVisibleWithAnchor(maxVisible int, anchorRow int) {
	if len(l.Items) == 0 {
		l.Cursor = 0
		l.ViewportOffset = 0
		return
	}
	if l.Cursor < 0 {
		l.Cursor = 0
	}
	if l.Cursor >= len(l.Items) {
		l.Cursor = len(l.Items) - 1
	}
	if maxVisible <= 0 {
		l.ViewportOffset = 0
		return
	}
	maxOffset := len(l.Items) - maxVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if anchorRow < 0 {
		anchorRow = 0
	}
	if anchorRow >= maxVisible {
		anchorRow = maxVisible - 1
	}
	offset := l.Cursor - anchorRow
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	l.ViewportOffset = offset
}
