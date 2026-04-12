package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	completionMaxVisible      = 10
	completionMaxContentWidth = 50 // hard cap on popup content width, regardless of terminal size
)

// OptionScope categorises a tmux option by its primary scope. Blank means
// "unknown/not an option" and suppresses scope-specific styling.
type OptionScope string

const (
	ScopeServer  OptionScope = "server"
	ScopeSession OptionScope = "session"
	ScopeWindow  OptionScope = "window"
	ScopePane    OptionScope = "pane"
	ScopeUser    OptionScope = "user"
)

type completionItem struct {
	Value       string
	Label       string
	Description string
	Scope       OptionScope
}

type CompletionOptions struct {
	Items        []string
	Labels       map[string]string
	Descriptions map[string]string
	Scopes       map[string]OptionScope
	ArgType      string
	TypeLabel    string
	Prefix       string
	AnchorCol    int
}

// completionState tracks the argument completion dropdown state.
type completionState struct {
	visible   bool
	items     []completionItem
	filtered  []completionItem
	cursor    int
	prefix    string
	anchorCol int
	argType   string
	typeLabel string
}

func newCompletionState(opts CompletionOptions) *completionState {
	detailedItems := make([]completionItem, 0, len(opts.Items))
	for _, item := range opts.Items {
		label := item
		if opts.Labels != nil && opts.Labels[item] != "" {
			label = opts.Labels[item]
		}
		description := ""
		if opts.Descriptions != nil {
			description = opts.Descriptions[item]
		}
		var scope OptionScope
		if opts.Scopes != nil {
			scope = opts.Scopes[item]
		}
		detailedItems = append(detailedItems, completionItem{
			Value:       item,
			Label:       label,
			Description: description,
			Scope:       scope,
		})
	}
	return newCompletionStateWithItems(detailedItems, opts.ArgType, opts.TypeLabel, opts.AnchorCol)
}

func newCompletionStateWithItems(items []completionItem, argType, typeLabel string, anchorCol int) *completionState {
	cs := &completionState{
		visible:   len(items) > 0,
		items:     append([]completionItem(nil), items...),
		anchorCol: anchorCol,
		argType:   argType,
		typeLabel: typeLabel,
	}
	for idx := range cs.items {
		if cs.items[idx].Label == "" {
			cs.items[idx].Label = cs.items[idx].Value
		}
	}
	cs.filtered = append([]completionItem(nil), cs.items...)
	return cs
}

func (cs *completionState) applyFilter(prefix string) {
	if cs == nil {
		return
	}
	cs.prefix = prefix
	cs.filtered = cs.filtered[:0]

	if prefix == "" {
		cs.filtered = append(cs.filtered, cs.items...)
		cs.cursor = 0
		cs.visible = len(cs.filtered) > 0
		return
	}

	lower := strings.ToLower(prefix)
	for _, item := range cs.items {
		if strings.HasPrefix(strings.ToLower(item.Value), lower) {
			cs.filtered = append(cs.filtered, item)
		}
	}
	if cs.cursor >= len(cs.filtered) {
		cs.cursor = 0
	}
	cs.visible = len(cs.filtered) > 0
}

func (cs *completionState) moveDown() {
	if cs == nil || len(cs.filtered) == 0 {
		return
	}
	if cs.cursor < len(cs.filtered)-1 {
		cs.cursor++
		return
	}
	cs.cursor = 0
}

func (cs *completionState) moveUp() {
	if cs == nil || len(cs.filtered) == 0 {
		return
	}
	if cs.cursor > 0 {
		cs.cursor--
		return
	}
	cs.cursor = len(cs.filtered) - 1
}

// pageStep returns the number of rows a page-up/page-down should move. It is
// sized to the visible viewport of the dropdown — the same completionMaxVisible
// constant used when rendering — clamped to at least 1.
func (cs *completionState) pageStep() int {
	return max(completionMaxVisible, 1)
}

// movePageDown advances the cursor by one viewport, clamping at the last item.
func (cs *completionState) movePageDown() {
	if cs == nil || len(cs.filtered) == 0 {
		return
	}
	cs.cursor += cs.pageStep()
	if cs.cursor >= len(cs.filtered) {
		cs.cursor = len(cs.filtered) - 1
	}
}

// movePageUp retreats the cursor by one viewport, clamping at the first item.
func (cs *completionState) movePageUp() {
	if cs == nil || len(cs.filtered) == 0 {
		return
	}
	cs.cursor -= cs.pageStep()
	if cs.cursor < 0 {
		cs.cursor = 0
	}
}

func (cs *completionState) selected() string {
	if cs == nil || len(cs.filtered) == 0 || cs.cursor < 0 || cs.cursor >= len(cs.filtered) {
		return ""
	}
	return cs.filtered[cs.cursor].Value
}

func (cs *completionState) hasExactMatch(prefix string) bool {
	if cs == nil || prefix == "" {
		return false
	}
	for _, item := range cs.filtered {
		if strings.EqualFold(item.Value, prefix) {
			return true
		}
	}
	return false
}

func (cs *completionState) ghostHint(typedPrefix string) string {
	if cs == nil || len(cs.filtered) == 0 {
		return ""
	}

	selected := cs.selected()
	if selected == "" {
		return ""
	}
	if typedPrefix == "" {
		return selected
	}

	selectedRunes := []rune(selected)
	prefixRunes := []rune(typedPrefix)
	if len(prefixRunes) > len(selectedRunes) {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(selected), strings.ToLower(typedPrefix)) {
		return string(selectedRunes[len(prefixRunes):])
	}
	return ""
}

func (cs *completionState) view(maxWidth, maxHeight int) string {
	if cs == nil || len(cs.filtered) == 0 {
		return ""
	}

	maxRows := completionMaxVisible
	if maxHeight > 0 && maxHeight < maxRows {
		maxRows = maxHeight
	}
	if maxRows < 1 {
		maxRows = 1
	}

	start := 0
	if len(cs.filtered) > maxRows {
		switch {
		case cs.cursor >= maxRows:
			start = cs.cursor - maxRows + 1
		case cs.cursor < start:
			start = cs.cursor
		}
	}
	end := min(start+maxRows, len(cs.filtered))
	visible := cs.filtered[start:end]

	// Measure against the entire filtered candidate set, not just the rows
	// currently on screen. Measuring only `visible` makes the popup reflow
	// every time the user scrolls because the widest row drifts in and out
	// of the viewport.
	leftWidth := 1
	rightWidth := 0
	hasDescriptions := false
	for _, item := range cs.filtered {
		if w := lipgloss.Width(item.Label); w > leftWidth {
			leftWidth = w
		}
		if item.Description != "" {
			hasDescriptions = true
			if w := lipgloss.Width(item.Description); w > rightWidth {
				rightWidth = w
			}
		}
	}

	contentWidth := leftWidth
	if hasDescriptions {
		contentWidth += 2 + rightWidth
	}
	// Apply caps from tightest to loosest: hard fixed cap, then terminal-
	// relative cap, then leave contentWidth alone.
	capWidth := completionMaxContentWidth
	if maxWidth > 0 {
		if termCap := maxWidth - 4; termCap > 0 && termCap < capWidth {
			capWidth = termCap
		}
	}
	if capWidth > 0 && contentWidth > capWidth {
		if !hasDescriptions {
			contentWidth = capWidth
			leftWidth = capWidth
		} else if leftWidth >= capWidth {
			leftWidth = capWidth
			rightWidth = 0
			contentWidth = capWidth
		} else {
			rightWidth = capWidth - leftWidth - 2
			rightWidth = max(rightWidth, 0)
			contentWidth = leftWidth
			if rightWidth > 0 {
				contentWidth += 2 + rightWidth
			}
		}
	}

	itemStyle := styles.CompletionItem
	if itemStyle == nil {
		fallback := lipgloss.NewStyle()
		itemStyle = &fallback
	}
	selectedStyle := styles.CompletionSelected
	if selectedStyle == nil {
		fallback := lipgloss.NewStyle().Reverse(true)
		selectedStyle = &fallback
	}

	borderStyle := styles.CompletionBorder
	borderColor := lipgloss.Color("240")
	if borderStyle != nil {
		borderColor = borderStyle.GetForeground()
	}
	scrollbarCellStyle := lipgloss.NewStyle().Foreground(borderColor)

	lines := make([]string, 0, len(visible))
	for idx, item := range visible {
		label := ansi.Truncate(item.Label, leftWidth, "…")
		if pad := leftWidth - lipgloss.Width(label); pad > 0 {
			label += strings.Repeat(" ", pad)
		}
		isSelected := start+idx == cs.cursor
		segStyle := itemStyle
		if isSelected {
			segStyle = selectedStyle
		}

		// Compose the scope foreground over the segment background so the
		// selected row keeps its scope colour instead of being repainted in
		// the segment's own foreground.
		labelStyle := *segStyle
		if scopeStyle := scopeStyleFor(item.Scope); scopeStyle != nil {
			composed := *scopeStyle
			labelStyle = composed.Inherit(*segStyle)
		}

		description := ""
		if hasDescriptions && rightWidth > 0 && item.Description != "" {
			description = ansi.Truncate(item.Description, rightWidth, "…")
		}
		rawContent := label
		if hasDescriptions && rightWidth > 0 {
			rawContent += "  " + description
		}
		padCount := max(contentWidth-lipgloss.Width(rawContent), 0)

		// Render each segment independently so an inner ANSI reset from
		// the scope-coloured label cannot kill the segment background of
		// the surrounding cells.
		line := segStyle.Render(" ") + labelStyle.Render(label)
		if hasDescriptions && rightWidth > 0 {
			line += segStyle.Render("  " + description)
		}
		if padCount > 0 {
			line += segStyle.Render(strings.Repeat(" ", padCount))
		}
		line += segStyle.Render(" ")
		lines = append(lines, line)
	}
	listBlock := strings.Join(lines, "\n")

	// Render the scrollbar as its own column, then join it beside the item
	// block. This keeps the scrollbar strictly outside each row's styled body
	// so selection highlighting cannot bleed into it.
	if scrollCells := renderScrollbar(len(cs.filtered), len(visible), start); scrollCells != nil {
		cellStrs := make([]string, len(scrollCells))
		for i, c := range scrollCells {
			cellStrs[i] = scrollbarCellStyle.Render(string(c))
		}
		scrollColumn := strings.Join(cellStrs, "\n")
		listBlock = lipgloss.JoinHorizontal(lipgloss.Top, listBlock, scrollColumn)
	}

	popup := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(listBlock)

	// Render a one-line scope legend underneath the popup (outside the
	// border) when the dropdown is offering option-name candidates.
	if cs.argType == "option" {
		if legend := renderScopeLegend(); legend != "" {
			popup = lipgloss.JoinVertical(lipgloss.Left, popup, legend)
		}
	}

	return popup
}

// scopeStyleFor returns the theme style for an OptionScope or nil when
// the scope is empty/unknown. The returned style is safe to call Render
// on; a nil result means "no scope-specific styling applies".
func scopeStyleFor(scope OptionScope) *lipgloss.Style {
	switch scope {
	case ScopeServer:
		return styles.OptionScopeServer
	case ScopeSession:
		return styles.OptionScopeSession
	case ScopeWindow:
		return styles.OptionScopeWindow
	case ScopePane:
		return styles.OptionScopePane
	case ScopeUser:
		return styles.OptionScopeUser
	}
	return nil
}

// renderScopeLegend returns a single-line legend showing each scope
// category rendered in its theme colour. The order matches the natural
// tmux hierarchy so the eye can find the category without hunting.
func renderScopeLegend() string {
	parts := []struct {
		label string
		scope OptionScope
	}{
		{"server", ScopeServer},
		{"session", ScopeSession},
		{"window", ScopeWindow},
		{"pane", ScopePane},
		{"user", ScopeUser},
	}
	pieces := make([]string, 0, len(parts))
	for _, p := range parts {
		style := scopeStyleFor(p.scope)
		if style == nil {
			pieces = append(pieces, p.label)
			continue
		}
		pieces = append(pieces, style.Render(p.label))
	}
	return " " + strings.Join(pieces, "  ") + " "
}
