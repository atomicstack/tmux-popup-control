package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
)

// extractModeTimeout is how long the mode selector popup stays open without
// activity before it auto-dismisses.
const extractModeTimeout = time.Second

// extractModeTimeoutMsg fires when the mode popup inactivity timer elapses.
// seq guards against stale timers: only a timeout whose seq still matches
// m.extractModeSeq closes the popup (each activity reschedules with a new seq).
type extractModeTimeoutMsg struct{ seq int }

// extractModePopupVisible reports whether the mode selector popup is open.
func (m *Model) extractModePopupVisible() bool {
	return m.extractModePopup != nil && m.extractModePopup.visible
}

// armExtractModeTimer bumps the seq and schedules a fresh inactivity timeout.
func (m *Model) armExtractModeTimer() tea.Cmd {
	m.extractModeSeq++
	seq := m.extractModeSeq
	return tea.Tick(extractModeTimeout, func(time.Time) tea.Msg {
		return extractModeTimeoutMsg{seq: seq}
	})
}

// extractModeAnchorCol is the column at which the popup is anchored — under the
// current mode name in the "mode: " line (indicator column + len("mode: ")).
func (m *Model) extractModeAnchorCol() int {
	return len("mode: ")
}

// openExtractModePopup opens the mode selector at the current category without
// changing it, recording the pre-popup category for esc-revert.
func (m *Model) openExtractModePopup() tea.Cmd {
	m.extractModePrePopup = m.extractCategory
	cats := extract.Categories()
	items := make([]string, len(cats))
	cursor := 0
	for i, c := range cats {
		items[i] = c.String()
		if c == m.extractCategory {
			cursor = i
		}
	}
	cs := newCompletionState(CompletionOptions{
		Items:     items,
		AnchorCol: m.extractModeAnchorCol(),
	})
	cs.cursor = cursor
	m.extractModePopup = cs
	return m.armExtractModeTimer()
}

// closeExtractModePopup hides the popup, keeping the current category.
func (m *Model) closeExtractModePopup() {
	m.extractModePopup = nil
}

// applyExtractModeCursor syncs m.extractCategory to the popup's highlighted row
// and triggers a live re-extract.
func (m *Model) applyExtractModeCursor() tea.Cmd {
	if m.extractModePopup == nil {
		return nil
	}
	cats := extract.Categories()
	idx := m.extractModePopup.cursor
	if idx < 0 || idx >= len(cats) {
		return nil
	}
	if cats[idx] == m.extractCategory {
		return nil
	}
	m.extractCategory = cats[idx]
	return m.extractReloadCmd()
}

// handleExtractKey routes key presses for the extract level, including the mode
// selector popup. It returns (cmd, handled); handled=false lets the key fall
// through to the normal menu key handling.
func (m *Model) handleExtractKey(keyMsg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := keyMsg.String()

	if m.extractModePopupVisible() {
		switch key {
		case "ctrl+f", "down":
			m.extractModePopup.moveDown()
			return tea.Batch(m.applyExtractModeCursor(), m.armExtractModeTimer()), true
		case "up":
			m.extractModePopup.moveUp()
			return tea.Batch(m.applyExtractModeCursor(), m.armExtractModeTimer()), true
		case "enter":
			// Confirm the current mode and close.
			m.closeExtractModePopup()
			return nil, true
		case "esc":
			// Revert to the mode active before the popup opened.
			m.closeExtractModePopup()
			if m.extractCategory != m.extractModePrePopup {
				m.extractCategory = m.extractModePrePopup
				return m.extractReloadCmd(), true
			}
			return nil, true
		}
		// Other keys (typing to filter, etc.) pass through to normal handling.
		return nil, false
	}

	// Popup closed.
	switch key {
	case "ctrl+f":
		return m.openExtractModePopup(), true
	case "tab", "ctrl+y":
		return m.extractCopy(), true
	case "shift+tab":
		if current := m.currentLevel(); current != nil {
			current.ToggleCurrentSelection()
		}
		return nil, true
	case "enter":
		return m.extractInsert(), true
	}
	return nil, false
}

// handleExtractModeTimeoutMsg closes the mode popup when its inactivity timer
// fires and no newer activity has rescheduled it.
func (m *Model) handleExtractModeTimeoutMsg(msg tea.Msg) tea.Cmd {
	timeout, ok := msg.(extractModeTimeoutMsg)
	if !ok {
		return nil
	}
	if !m.extractModePopupVisible() {
		return nil
	}
	if timeout.seq != m.extractModeSeq {
		return nil // superseded by later activity
	}
	m.closeExtractModePopup()
	return nil
}
