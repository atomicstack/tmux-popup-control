package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const completionMaxVisible = 10

type completionItem struct {
	Value       string
	Label       string
	Description string
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

func newCompletionState(items []string, argType, typeLabel string, anchorCol int) *completionState {
	return newCompletionStateWithLabels(items, nil, argType, typeLabel, anchorCol)
}

func newCompletionStateWithLabels(items []string, labels map[string]string, argType, typeLabel string, anchorCol int) *completionState {
	return newCompletionStateWithMetadata(items, labels, nil, argType, typeLabel, anchorCol)
}

func newCompletionStateWithMetadata(items []string, labels, descriptions map[string]string, argType, typeLabel string, anchorCol int) *completionState {
	detailedItems := make([]completionItem, 0, len(items))
	for _, item := range items {
		label := item
		if labels != nil && labels[item] != "" {
			label = labels[item]
		}
		description := ""
		if descriptions != nil {
			description = descriptions[item]
		}
		detailedItems = append(detailedItems, completionItem{
			Value:       item,
			Label:       label,
			Description: description,
		})
	}
	return newCompletionStateWithItems(detailedItems, argType, typeLabel, anchorCol)
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
	if cs == nil {
		return
	}
	if cs.cursor < len(cs.filtered)-1 {
		cs.cursor++
	}
}

func (cs *completionState) moveUp() {
	if cs == nil {
		return
	}
	if cs.cursor > 0 {
		cs.cursor--
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

func (cs *completionState) selectedLabel() string {
	if cs == nil || len(cs.filtered) == 0 || cs.cursor < 0 || cs.cursor >= len(cs.filtered) {
		return ""
	}
	return cs.filtered[cs.cursor].Label
}

func (cs *completionState) labelFor(value string) string {
	if cs == nil {
		return value
	}
	for _, item := range cs.items {
		if item.Value == value {
			if item.Label != "" {
				return item.Label
			}
			return value
		}
	}
	return value
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
	end := start + maxRows
	if end > len(cs.filtered) {
		end = len(cs.filtered)
	}
	visible := cs.filtered[start:end]

	leftWidth := 1
	rightWidth := 0
	hasDescriptions := false
	for _, item := range visible {
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
	if maxWidth > 0 {
		if capWidth := maxWidth - 4; capWidth > 0 && contentWidth > capWidth {
			if !hasDescriptions {
				contentWidth = capWidth
				leftWidth = capWidth
			} else if leftWidth >= capWidth {
				leftWidth = capWidth
				rightWidth = 0
				contentWidth = capWidth
			} else {
				rightWidth = capWidth - leftWidth - 2
				if rightWidth < 0 {
					rightWidth = 0
				}
				contentWidth = leftWidth
				if rightWidth > 0 {
					contentWidth += 2 + rightWidth
				}
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

	lines := make([]string, 0, len(visible))
	for idx, item := range visible {
		label := ansi.Truncate(item.Label, leftWidth, "…")
		if pad := leftWidth - lipgloss.Width(label); pad > 0 {
			label += strings.Repeat(" ", pad)
		}
		content := label
		if hasDescriptions && rightWidth > 0 {
			description := ""
			if item.Description != "" {
				description = ansi.Truncate(item.Description, rightWidth, "…")
			}
			content += "  " + description
		}
		if pad := contentWidth - lipgloss.Width(content); pad > 0 {
			content += strings.Repeat(" ", pad)
		}
		line := " " + content + " "
		if start+idx == cs.cursor {
			lines = append(lines, selectedStyle.Render(line))
		} else {
			lines = append(lines, itemStyle.Render(line))
		}
	}

	if start > 0 && len(lines) > 0 {
		lines[0] = itemStyle.Render(" " + padIndicator("^", contentWidth) + " ")
	}
	if end < len(cs.filtered) && len(lines) > 0 {
		lines[len(lines)-1] = itemStyle.Render(" " + padIndicator("v", contentWidth) + " ")
	}

	borderStyle := styles.CompletionBorder
	borderColor := lipgloss.Color("240")
	if borderStyle != nil {
		borderColor = borderStyle.GetForeground()
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(strings.Join(lines, "\n"))
}

func padIndicator(marker string, width int) string {
	if width <= 1 {
		return marker
	}
	return strings.Repeat(" ", width-1) + marker
}
