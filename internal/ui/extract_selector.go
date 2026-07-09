package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
)

// Chrome segments of the combined bottom-bar line
// ("mode: <cat> <^f>   area: <area> <^g>   insert: <Enter>   copy: <Tab>").
// Each angle-bracketed key name is a separate segment from its surrounding
// "<"/">" brackets so the key can render a slightly lighter grey
// (SelectorHintKey) than the brackets/labels (FilterPlaceholder). The trailing
// insert/copy hints append after the area selector and so do not affect the
// mode/area popup anchor columns.
const (
	extractModePrefix  = "mode: "
	extractAreaPrefix  = "area: "
	extractHotkeyOpen  = " <" // leading gap + open bracket for the ^f/^g hotkeys
	extractAngleOpen   = "<"  // open bracket for the insert/copy hints
	extractAngleClose  = ">"
	extractSelectorGap = "   "
	extractModeKey     = "^f"
	extractAreaKey     = "^g"
	extractInsertLabel = "insert: "
	extractInsertKey   = "Enter"
	extractCopyLabel   = "copy: "
	extractCopyKey     = "Tab"
)

// extractSelectorValue constrains the value types a selector popup can cycle
// through: comparable so the current value can be located/compared, and
// Stringer so item labels are derived straight from the type's own String().
// extract.Category and extract.GrabArea both satisfy this.
type extractSelectorValue interface {
	comparable
	String() string
}

// buildExtractSelectorItems renders values as their String() labels and
// locates the cursor position of current within them. Shared by
// openExtractModePopup and openExtractAreaPopup.
func buildExtractSelectorItems[T extractSelectorValue](values []T, current T) ([]string, int) {
	items := make([]string, len(values))
	cursor := 0
	for i, v := range values {
		items[i] = v.String()
		if v == current {
			cursor = i
		}
	}
	return items, cursor
}

// applyExtractSelectorCursor syncs *current to the popup's highlighted row
// and, if the value changed, triggers a live re-extract. Shared by
// applyExtractModeCursor and applyExtractAreaCursor.
func applyExtractSelectorCursor[T extractSelectorValue](m *Model, popup *completionState, values []T, current *T) tea.Cmd {
	if popup == nil {
		return nil
	}
	idx := popup.cursor
	if idx < 0 || idx >= len(values) {
		return nil
	}
	if values[idx] == *current {
		return nil
	}
	*current = values[idx]
	return m.extractReloadCmd()
}

// extractAreaAnchorCol is the column at which the area popup is anchored —
// under the current area name on the combined "mode: ... area: ..." line.
func (m *Model) extractAreaAnchorCol() int {
	return len(extractModePrefix) + len(m.extractCategory.String()) +
		len(extractHotkeyOpen) + len(extractModeKey) + len(extractAngleClose) +
		len(extractSelectorGap) + len(extractAreaPrefix)
}

// extractAreaPopupVisible reports whether the area selector popup is open.
func (m *Model) extractAreaPopupVisible() bool {
	return m.extractAreaPopup != nil && m.extractAreaPopup.visible
}

// openExtractAreaPopup opens the area selector at the current grab area
// without changing it, recording the pre-popup area for esc-revert.
func (m *Model) openExtractAreaPopup() tea.Cmd {
	m.extractAreaPrePopup = m.extractGrabArea
	items, cursor := buildExtractSelectorItems(extract.GrabAreas(), m.extractGrabArea)
	cs := newCompletionState(CompletionOptions{
		Items:     items,
		AnchorCol: m.extractAreaAnchorCol(),
	})
	cs.cursor = cursor
	m.extractAreaPopup = cs
	return m.armExtractModeTimer()
}

// closeExtractAreaPopup hides the popup, keeping the current grab area.
func (m *Model) closeExtractAreaPopup() {
	m.extractAreaPopup = nil
}

// applyExtractAreaCursor syncs m.extractGrabArea to the popup's highlighted
// row and triggers a live re-extract.
func (m *Model) applyExtractAreaCursor() tea.Cmd {
	return applyExtractSelectorCursor(m, m.extractAreaPopup, extract.GrabAreas(), &m.extractGrabArea)
}

// revertExtractArea reverts extractGrabArea to the value active before the
// area popup opened, re-extracting live if it changed. Used by esc.
func (m *Model) revertExtractArea() tea.Cmd {
	if m.extractGrabArea == m.extractAreaPrePopup {
		return nil
	}
	m.extractGrabArea = m.extractAreaPrePopup
	return m.extractReloadCmd()
}

// handleExtractKey routes key presses for the extract level, including the
// mode and area selector popups. It returns (cmd, handled); handled=false
// lets the key fall through to the normal menu key handling. Only one
// selector popup is ever open at a time: while one is open, the other
// selector's hotkey is inert (handled, no-op) rather than opening or
// switching popups.
func (m *Model) handleExtractKey(keyMsg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := keyMsg.String()

	if m.extractModePopupVisible() {
		return m.handleExtractModePopupKey(key)
	}
	if m.extractAreaPopupVisible() {
		return m.handleExtractAreaPopupKey(key)
	}

	// No popup open.
	switch key {
	case "ctrl+f":
		return m.openExtractModePopup(), true
	case "ctrl+g":
		return m.openExtractAreaPopup(), true
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

// handleExtractSelectorPopupKey implements the key routing shared by both
// selector popups: the popup's own hotkey or "down" advances (wrapping),
// "up" goes back (wrapping), "enter" confirms and closes, "esc" reverts to
// the pre-popup value (via revert) and closes, and the other selector's
// hotkey is inert — only one popup is ever open at a time. Every activity
// re-arms the shared inactivity timer. Other keys (typing to filter, etc.)
// pass through to normal handling.
func (m *Model) handleExtractSelectorPopupKey(key, ownHotkey, otherHotkey string, popup *completionState, moveApply func() tea.Cmd, closePopup func(), revert func() tea.Cmd) (tea.Cmd, bool) {
	switch key {
	case ownHotkey, "down":
		popup.moveDown()
		return tea.Batch(moveApply(), m.armExtractModeTimer()), true
	case "up":
		popup.moveUp()
		return tea.Batch(moveApply(), m.armExtractModeTimer()), true
	case "enter":
		closePopup()
		return nil, true
	case "esc":
		closePopup()
		return revert(), true
	case otherHotkey:
		// Only one selector popup can be open at a time; the other
		// selector's hotkey is a handled no-op rather than switching popups.
		return nil, true
	}
	return nil, false
}

func (m *Model) handleExtractModePopupKey(key string) (tea.Cmd, bool) {
	return m.handleExtractSelectorPopupKey(key, "ctrl+f", "ctrl+g", m.extractModePopup,
		m.applyExtractModeCursor, m.closeExtractModePopup, m.revertExtractMode)
}

func (m *Model) handleExtractAreaPopupKey(key string) (tea.Cmd, bool) {
	return m.handleExtractSelectorPopupKey(key, "ctrl+g", "ctrl+f", m.extractAreaPopup,
		m.applyExtractAreaCursor, m.closeExtractAreaPopup, m.revertExtractArea)
}
