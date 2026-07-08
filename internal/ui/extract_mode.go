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

// extractModeAnchorCol is the column at which the mode popup is anchored —
// under the current mode name in the "mode: " line (indicator column +
// len("mode: ")). Uses the same chrome constant extractSubtitle renders with
// (see extract_selector.go), so the two cannot drift apart.
func (m *Model) extractModeAnchorCol() int {
	return len(extractModePrefix)
}

// openExtractModePopup opens the mode selector at the current category without
// changing it, recording the pre-popup category for esc-revert.
func (m *Model) openExtractModePopup() tea.Cmd {
	m.extractModePrePopup = m.extractCategory
	items, cursor := buildExtractSelectorItems(extract.Categories(), m.extractCategory)
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
	return applyExtractSelectorCursor(m, m.extractModePopup, extract.Categories(), &m.extractCategory)
}

// revertExtractMode reverts extractCategory to the value active before the
// mode popup opened, re-extracting live if it changed. Used by esc.
func (m *Model) revertExtractMode() tea.Cmd {
	if m.extractCategory == m.extractModePrePopup {
		return nil
	}
	m.extractCategory = m.extractModePrePopup
	return m.extractReloadCmd()
}

// handleExtractModeTimeoutMsg closes whichever selector popup (mode or area)
// is open when its inactivity timer fires and no newer activity has
// rescheduled it. Only one selector popup is ever open at a time, and both
// share the same timer/seq (m.extractModeSeq), so a single seq comparison
// covers either popup.
func (m *Model) handleExtractModeTimeoutMsg(msg tea.Msg) tea.Cmd {
	timeout, ok := msg.(extractModeTimeoutMsg)
	if !ok {
		return nil
	}
	if !m.extractModePopupVisible() && !m.extractAreaPopupVisible() {
		return nil
	}
	if timeout.seq != m.extractModeSeq {
		return nil // superseded by later activity
	}
	if m.extractModePopupVisible() {
		m.closeExtractModePopup()
	} else {
		m.closeExtractAreaPopup()
	}
	return nil
}
