package ui

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdhelp"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
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
		m.clearCompletionSuppression()
		m.triggerCompletion()
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
		m.clearCompletionSuppression()
		m.triggerCompletion()
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
		m.dismissCompletionIfCursorMovedAway(current)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	case "ctrl+e":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorEnd() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		m.dismissCompletionIfCursorMovedAway(current)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	case "alt+b":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorWordBackward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		m.dismissCompletionIfCursorMovedAway(current)
		events.Filter.CursorWord(current.ID, current.FilterCursor)
		return true, nil
	case "alt+f":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorWordForward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		m.dismissCompletionIfCursorMovedAway(current)
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
		m.dismissCompletionIfCursorMovedAway(current)
		events.Filter.Cursor(current.ID, current.FilterCursor)
		return true, nil
	case "right":
		before := current.FilterCursorPos()
		if !current.MoveFilterCursorRuneForward() {
			return false, nil
		}
		m.noteFilterCursorChange(current, before)
		m.dismissCompletionIfCursorMovedAway(current)
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
	m.clearCompletionSuppression()
	m.triggerCompletion()
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
	m.clearCompletionSuppression()
	m.triggerCompletion()
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
	pos = max(pos, 0)
	pos = min(pos, len(runes))
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
// item's ID.
func (m *Model) autoCompleteGhost() string {
	current := m.currentLevel()
	if current == nil {
		return ""
	}
	if current.Filter == "" {
		return ""
	}
	if current.FilterCursorPos() != len([]rune(current.Filter)) {
		return ""
	}

	if m.completion != nil {
		if m.completion.visible && len(m.completion.filtered) > 0 {
			return m.completion.ghostHint(m.completion.prefix)
		}
		if !m.completion.visible && m.completion.typeLabel != "" && m.completion.prefix == "" {
			return m.completion.typeLabel
		}
	}

	if current.Node != nil && current.Node.FilterCommand && strings.Contains(current.Filter, " ") {
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

func (m *Model) currentCommandSummary() string {
	current := m.currentLevel()
	if current == nil || current.Node == nil || !current.Node.FilterCommand {
		return ""
	}

	name := ""
	if schema := m.lookupCommandSchema(current.Filter); schema != nil && schema.Name != "" {
		name = schema.Name
	} else if current.Filter != "" && !strings.Contains(current.Filter, " ") {
		if current.Cursor >= 0 && current.Cursor < len(current.Items) {
			name = current.Items[current.Cursor].ID
		}
	}

	if name == "" && current.Cursor >= 0 && current.Cursor < len(current.Items) {
		name = current.Items[current.Cursor].ID
	}
	if name == "" {
		return ""
	}

	help, ok := m.lookupCommandHelp(name)
	if !ok {
		return ""
	}
	return help.Summary
}

func (m *Model) lookupCommandHelp(name string) (cmdhelp.CommandHelp, bool) {
	if m == nil || m.commandHelp == nil || name == "" {
		return cmdhelp.CommandHelp{}, false
	}
	help, ok := m.commandHelp[name]
	return help, ok
}

// triggerCompletion analyses the current command input and updates the
// completion dropdown or placeholder state.
func (m *Model) triggerCompletion() {
	current := m.currentLevel()
	if current == nil || current.Node == nil || !current.Node.FilterCommand {
		m.dismissCompletion()
		return
	}
	if current.FilterCursorPos() != len([]rune(current.Filter)) {
		m.dismissCompletion()
		return
	}
	if m.commandSchemas == nil {
		m.dismissCompletion()
		return
	}
	if m.completionSuppressedFilter != "" && current.Filter == m.completionSuppressedFilter {
		m.dismissCompletion()
		return
	}

	ctx := cmdparse.Analyse(m.commandSchemas, current.Filter)
	schema := m.lookupCommandSchema(current.Filter)
	ctx.ArgType = m.adjustCompletionArgType(schema, ctx)
	typeLabel := ctx.TypeLabel
	if typeLabel == "" {
		typeLabel = ctx.ArgType
	}

	switch ctx.Kind {
	case cmdparse.ContextFlagName:
		if schema == nil {
			m.dismissCompletion()
			return
		}
		candidates := cmdparse.FlagCandidates(schema, ctx.FlagsUsed)
		if len(candidates) == 0 {
			m.dismissCompletion()
			return
		}
		values := make([]string, 0, len(candidates))
		labels := make(map[string]string, len(candidates))
		descriptions := make(map[string]string, len(candidates))
		help := m.commandHelpForSchema(schema)
		helpDescriptions := make(map[string]string, len(help.Args))
		for _, arg := range help.Args {
			helpDescriptions[arg.Name] = arg.Description
		}
		for _, candidate := range candidates {
			value := "-" + string(candidate.Flag)
			values = append(values, value)
			if candidate.ArgType != "" {
				labels[value] = fmt.Sprintf("-%c <%s>", candidate.Flag, candidate.ArgType)
			} else {
				labels[value] = value
			}
			descriptions[value] = helpDescriptions[value]
		}
		m.openCompletion(CompletionOptions{
			Items:        values,
			Labels:       labels,
			Descriptions: descriptions,
			ArgType:      "flag",
			TypeLabel:    "flag",
			Prefix:       ctx.Prefix,
		})
	case cmdparse.ContextFlagValue, cmdparse.ContextPositionalValue:
		if tmuxOpts, handled := m.tmuxOptCompletion(schema, ctx, current.Filter); handled {
			if len(tmuxOpts.Items) == 0 {
				m.completion = &completionState{
					typeLabel: tmuxOpts.TypeLabel,
					argType:   tmuxOpts.ArgType,
					prefix:    tmuxOpts.Prefix,
				}
				return
			}
			m.openCompletion(tmuxOpts)
			return
		}
		resolver := cmdparse.NewStoreResolver(&modelDataSource{
			sessions: m.sessions,
			windows:  m.windows,
			panes:    m.panes,
			schemas:  m.commandSchemas,
		})
		candidates := resolver.Resolve(ctx.ArgType)
		if len(candidates) == 0 {
			m.completion = &completionState{
				typeLabel: typeLabel,
				argType:   ctx.ArgType,
				prefix:    ctx.Prefix,
			}
			return
		}
		m.openCompletion(CompletionOptions{
			Items:     candidates,
			ArgType:   ctx.ArgType,
			TypeLabel: typeLabel,
			Prefix:    ctx.Prefix,
		})
	default:
		m.dismissCompletion()
	}
}

func (m *Model) openCompletion(opts CompletionOptions) {
	current := m.currentLevel()
	if current == nil {
		m.dismissCompletion()
		return
	}
	previousSelected := ""
	if m.completion != nil {
		previousSelected = m.completion.selected()
	}

	anchorCol := 2 + current.FilterCursorPos() - len([]rune(opts.Prefix))
	anchorCol = max(anchorCol, 0)

	opts.AnchorCol = anchorCol
	m.completion = newCompletionState(opts)
	if opts.Prefix != "" {
		m.completion.applyFilter(opts.Prefix)
	}
	if previousSelected != "" {
		for idx, item := range m.completion.filtered {
			if item.Value == previousSelected {
				m.completion.cursor = idx
				break
			}
		}
	}
	if opts.Prefix != "" && shouldDismissExactMatchCompletion(opts.ArgType) && m.completion.hasExactMatch(opts.Prefix) {
		m.dismissCompletion()
		return
	}
	if len(m.completion.filtered) == 0 {
		m.dismissCompletion()
	}
}

func (m *Model) dismissCompletion() {
	m.completion = nil
}

func (m *Model) dismissCompletionUntilInputChanges() {
	current := m.currentLevel()
	if current != nil {
		m.completionSuppressedFilter = current.Filter
	}
	m.dismissCompletion()
}

func (m *Model) clearCompletionSuppression() {
	m.completionSuppressedFilter = ""
}

func (m *Model) dismissCompletionIfCursorMovedAway(current *level) {
	if current == nil {
		return
	}
	if current.FilterCursorPos() != len([]rune(current.Filter)) {
		m.dismissCompletion()
	}
}

func (m *Model) lookupCommandSchema(input string) *cmdparse.CommandSchema {
	fields := strings.Fields(input)
	if len(fields) == 0 || m.commandSchemas == nil {
		return nil
	}
	return m.commandSchemas[fields[0]]
}

func (m *Model) commandHelpForSchema(schema *cmdparse.CommandSchema) cmdhelp.CommandHelp {
	if schema == nil || schema.Name == "" {
		return cmdhelp.CommandHelp{}
	}
	help, _ := m.lookupCommandHelp(schema.Name)
	return help
}

func (m *Model) adjustCompletionArgType(schema *cmdparse.CommandSchema, ctx cmdparse.CompletionContext) string {
	if schema == nil {
		return ctx.ArgType
	}
	if schema.Name == "move-window" &&
		ctx.Kind == cmdparse.ContextFlagValue &&
		ctx.ArgType == "dst-window" &&
		runeInSlice('r', ctx.FlagsUsed) {
		return "target-session"
	}
	return ctx.ArgType
}

func runeInSlice(target rune, values []rune) bool {
	return slices.Contains(values, target)
}

func shouldDismissExactMatchCompletion(argType string) bool {
	return argType != "flag"
}

func (m *Model) completionVisible() bool {
	return m.completion != nil && m.completion.visible && len(m.completion.filtered) > 0
}

func (m *Model) acceptCompletion() tea.Cmd {
	if m.completion == nil {
		return nil
	}
	selected := m.completion.selected()
	if selected == "" {
		m.dismissCompletion()
		return nil
	}

	current := m.currentLevel()
	if current == nil {
		m.dismissCompletion()
		return nil
	}

	filter := current.Filter
	prefix := m.completion.prefix
	if prefix != "" && strings.HasSuffix(filter, prefix) {
		filter = filter[:len(filter)-len(prefix)]
	}
	newFilter := filter + selected
	before := current.FilterCursorPos()
	current.SetFilter(newFilter, len([]rune(newFilter)))
	m.noteFilterCursorChange(current, before)
	m.syncFilterViewport(current)
	m.completionSuppressedFilter = current.Filter
	m.dismissCompletion()
	return nil
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
