package state

// MoveCursorHome moves the cursor to the first item.
func (l *Level) MoveCursorHome() bool {
	if len(l.Items) == 0 {
		l.Cursor = 0
		return false
	}
	old := l.Cursor
	l.Cursor = 0
	return old != l.Cursor
}

// MoveCursorEnd moves the cursor to the last item.
func (l *Level) MoveCursorEnd() bool {
	n := len(l.Items)
	if n == 0 {
		l.Cursor = 0
		return false
	}
	old := l.Cursor
	l.Cursor = n - 1
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
	return l.Cursor != old
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
