package ui

import (
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
)

func (m *Model) updateFilterCursorModel(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.filterCursor, cmd = m.filterCursor.Update(msg)
	return cmd
}

func (m *Model) noteFilterCursorChange(l *level, before int) {
	if l == nil {
		return
	}
	if before != l.FilterCursorPos() {
		m.filterCursorDirty = true
	}
}

func (m *Model) handleTextInput(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.loading {
		return false, nil
	}
	current := m.currentLevel()
	if current == nil {
		return false, nil
	}
	switch msg.String() {
	case "ctrl+u":
		if current.Filter == "" {
			return false, nil
		}
		before := current.FilterCursorPos()
		current.SetFilter("", 0)
		m.noteFilterCursorChange(current, before)
		m.forceClearInfo()
		m.errMsg = ""
		events.Filter.Cleared(current.ID)
		m.syncTreeFilter(current)
		m.syncFilterViewport(current)
		return true, m.ensurePreviewForLevel(current)
	case "ctrl+w":
		before := current.FilterCursorPos()
		if !current.DeleteFilterWordBackward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		m.forceClearInfo()
		m.errMsg = ""
		events.Filter.WordBackspace(current.ID, current.Filter)
		m.syncTreeFilter(current)
		m.syncFilterViewport(current)
		return true, m.ensurePreviewForLevel(current)
	case "ctrl+a":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorStart() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	case "ctrl+e":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorEnd() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	case "alt+b":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorWordBackward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.CursorWord(current.ID, current.FilterCursor)
		return true, nil
	case "alt+f":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorWordForward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.CursorWord(current.ID, current.FilterCursor)
		return true, nil
	case "backspace":
		if m.removeFilterRune() {
			return true, m.ensurePreviewForLevel(current)
		}
		return false, nil
	case "space":
		if m.appendToFilter(" ") {
			return true, m.ensurePreviewForLevel(current)
		}
		return false, nil
	case "left":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorRuneBackward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	case "right":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorRuneForward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	default:
		if msg.Mod.Contains(tea.ModAlt) {
			return false, nil
		}
		if msg.Text == "" {
			return false, nil
		}
		for _, r := range []rune(msg.Text) {
			if unicode.IsControl(r) {
				return false, nil
			}
			if unicode.IsSpace(r) {
				return false, nil
			}
		}
		if m.appendToFilter(msg.Text) {
			return true, m.ensurePreviewForLevel(current)
		}
		return false, nil
	}
}

func (m *Model) appendToFilter(text string) bool {
	if text == "" {
		return false
	}
	current := m.currentLevel()
	if current == nil {
		return false
	}
	before := current.FilterCursorPos()
	if !current.InsertFilterText(text) {
		return false
	}
	m.noteFilterCursorChange(current, before)
	m.forceClearInfo()
	m.errMsg = ""
	events.Filter.Append(current.ID, current.Filter)
	m.syncTreeFilter(current)
	m.syncFilterViewport(current)
	return true
}

func (m *Model) removeFilterRune() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	before := current.FilterCursorPos()
	if !current.DeleteFilterRuneBackward() {
		return false
	}
	m.noteFilterCursorChange(current, before)
	m.forceClearInfo()
	m.errMsg = ""
	events.Filter.Backspace(current.ID, current.Filter)
	m.syncTreeFilter(current)
	m.syncFilterViewport(current)
	return true
}

func (m *Model) filterPrompt() (string, *lipgloss.Style) {
	current := m.currentLevel()
	if current == nil {
		return ">", styles.Filter
	}
	render := func(style *lipgloss.Style, value string) string {
		if style == nil || value == "" {
			return value
		}
		return style.Render(value)
	}
	if styles.Cursor != nil {
		m.filterCursor.Style = *styles.Cursor
	}
	if styles.Filter != nil {
		m.filterCursor.TextStyle = *styles.Filter
	} else {
		m.filterCursor.TextStyle = lipgloss.Style{}
	}
	prompt := "» "
	if styles.FilterPrompt != nil {
		prompt = styles.FilterPrompt.Render(prompt)
	}
	text := current.Filter
	if text == "" {
		placeholder := "(type to search)"
		runes := []rune(placeholder)
		var caretRune string
		var rest string
		if len(runes) > 0 {
			caretRune = string(runes[0])
			rest = string(runes[1:])
		}
		if styles.FilterPlaceholder != nil {
			m.filterCursor.TextStyle = *styles.FilterPlaceholder
		}
		caret := m.renderFilterCursor(caretRune)
		return prompt + caret + render(styles.FilterPlaceholder, rest), nil
	}
	runes := []rune(text)
	pos := current.FilterCursorPos()
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	before := render(styles.Filter, string(runes[:pos]))
	ghost := m.autoCompleteGhost()
	var caretRune string
	if pos < len(runes) {
		caretRune = string(runes[pos])
	} else if ghost != "" {
		// Use first ghost character as the cursor caret so there is no
		// visible gap between typed text and the autocomplete hint.
		ghostRunes := []rune(ghost)
		if styles.FilterPlaceholder != nil {
			m.filterCursor.TextStyle = *styles.FilterPlaceholder
		}
		caretRune = string(ghostRunes[0])
		caret := m.renderFilterCursor(caretRune)
		ghostTail := ""
		if len(ghostRunes) > 1 {
			ghostTail = render(styles.FilterPlaceholder, string(ghostRunes[1:]))
		}
		return prompt + before + caret + ghostTail, nil
	} else {
		caretRune = " "
	}
	caret := m.renderFilterCursor(caretRune)
	var after string
	if pos < len(runes) {
		after = render(styles.Filter, string(runes[pos+1:]))
	} else {
		after = ""
	}
	return prompt + before + caret + after, nil
}

// autoCompleteGhost returns the ghost text suffix for autocomplete, or "" if
// none applies. Ghost text is shown when the cursor is at the end of the
// filter text and the filter is a case-insensitive prefix of the highlighted
// item's ID on a FilterCommand level.
func (m *Model) autoCompleteGhost() string {
	current := m.currentLevel()
	if current == nil {
		return ""
	}
	if current.Node == nil || !current.Node.FilterCommand {
		return ""
	}
	if current.Filter == "" {
		return ""
	}
	if current.FilterCursorPos() != len([]rune(current.Filter)) {
		return ""
	}
	if current.Cursor < 0 || current.Cursor >= len(current.Items) {
		return ""
	}
	item := current.Items[current.Cursor]
	lower := strings.ToLower(current.Filter)
	idLower := strings.ToLower(item.ID)
	if !strings.HasPrefix(idLower, lower) {
		return ""
	}
	return item.ID[len(current.Filter):]
}

func (m *Model) renderFilterCursor(char string) string {
	if char == "" {
		char = " "
	}
	m.filterCursor.SetChar(char)

	base := m.filterCursor.TextStyle
	base = base.Inline(true)

	if m.filterCursor.IsBlinked {
		return base.Render(char)
	}

	if styles.Cursor != nil {
		cursorStyle := lipgloss.NewStyle().Inherit(*styles.Cursor).Inline(true)
		base = base.Inherit(cursorStyle).Blink(false)
		return base.Render(char)
	}

	return base.Reverse(true).Render(char)
}
