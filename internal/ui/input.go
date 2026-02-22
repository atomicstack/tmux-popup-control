package ui

import (
	"unicode"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func (m *Model) handleTextInput(msg tea.KeyMsg) (bool, tea.Cmd) {
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
		m.syncViewport(current)
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
		m.syncViewport(current)
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
	}
	switch msg.Type {
	case tea.KeyBackspace, tea.KeyCtrlH:
		if m.removeFilterRune() {
			return true, m.ensurePreviewForLevel(current)
		}
		return false, nil
	case tea.KeyRunes:
		if msg.Alt {
			return false, nil
		}
		if len(msg.Runes) == 0 {
			return false, nil
		}
		for _, r := range msg.Runes {
			if unicode.IsControl(r) {
				return false, nil
			}
			if unicode.IsSpace(r) {
				// allow the dedicated space handler to manage spaces
				return false, nil
			}
		}
		if m.appendToFilter(string(msg.Runes)) {
			return true, m.ensurePreviewForLevel(current)
		}
		return false, nil
	case tea.KeySpace:
		if m.appendToFilter(" ") {
			return true, m.ensurePreviewForLevel(current)
		}
		return false, nil
	case tea.KeyLeft:
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorRuneBackward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	case tea.KeyRight:
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorRuneForward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	}
	return false, nil
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
	m.syncViewport(current)
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
	m.syncViewport(current)
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
		m.filterCursor.Style = styles.Cursor.Copy()
	}
	if styles.Filter != nil {
		m.filterCursor.TextStyle = styles.Filter.Copy()
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
			m.filterCursor.TextStyle = styles.FilterPlaceholder.Copy()
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
	var caretRune string
	if pos < len(runes) {
		caretRune = string(runes[pos])
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

func (m *Model) renderFilterCursor(char string) string {
	if char == "" {
		char = " "
	}
	m.filterCursor.SetChar(char)

	base := m.filterCursor.TextStyle.Copy()
	base = base.Inline(true)

	if m.filterCursor.Blink {
		return base.Render(char)
	}

	if styles.Cursor != nil {
		cursorStyle := styles.Cursor.Copy().Inline(true)
		base = base.Inherit(cursorStyle).Blink(false)
		return base.Render(char)
	}

	return base.Reverse(true).Render(char)
}
