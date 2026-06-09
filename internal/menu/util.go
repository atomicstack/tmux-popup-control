package menu

import (
	"bufio"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
)

// currentLabelPrefix is the marker prepended to the active window/pane entry in
// switch/rename menus. stripCurrentPrefix removes it so the bare name is used
// when seeding rename forms or building table rows.
const currentLabelPrefix = "[current] "

// stripCurrentPrefix removes the "[current] " marker (and any "[current]"
// followed by a space) from a label, returning the bare name.
func stripCurrentPrefix(label string) string {
	label = strings.TrimSpace(label)
	if trimmed, ok := strings.CutPrefix(label, currentLabelPrefix); ok {
		return strings.TrimSpace(trimmed)
	}
	if strings.HasPrefix(label, "[current]") {
		if _, after, ok := strings.Cut(label, " "); ok {
			return strings.TrimSpace(after)
		}
	}
	return label
}

// withCurrentFirst prepends currentItem to items when ok is true, returning a
// new slice. Used by loaders that surface the active window/pane at the top.
func withCurrentFirst(items []Item, currentItem Item, ok bool) []Item {
	if !ok {
		return items
	}
	return append([]Item{currentItem}, items...)
}

// tableItems formats rows into aligned label strings and zips them with the
// parallel ids slice into menu items. rows and ids must be the same length.
func tableItems(rows [][]string, ids []string, aligns []table.Alignment) []Item {
	if len(rows) == 0 {
		return nil
	}
	aligned := table.Format(rows, aligns)
	items := make([]Item, len(aligned))
	for i, label := range aligned {
		items[i] = Item{ID: ids[i], Label: label}
	}
	return items
}

// splitSelectionIDs splits a multi-select ID payload into its constituent IDs.
// The UI joins marked selections with "\n" (internal/ui/navigation.go), so this
// splits on "\n" only — preserving any spaces or commas that legitimately occur
// within a single tmux target. Segments are trimmed, blanks dropped, and
// duplicates removed while preserving first-seen order.
func splitSelectionIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []string
	seen := make(map[string]struct{})
	for part := range strings.SplitSeq(raw, "\n") {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// failCmd returns a tea.Cmd that yields an ActionResult carrying a formatted
// error. Used by action handlers to report invalid targets and similar.
func failCmd(format string, args ...any) tea.Cmd {
	err := fmt.Errorf(format, args...)
	return func() tea.Msg { return ActionResult{Err: err} }
}

// runAction returns a tea.Cmd that traces, runs op, and reports the outcome:
// ActionResult{Err} on failure, otherwise ActionResult{Info: okMsg}.
func runAction(trace func(), op func() error, okMsg string) tea.Cmd {
	return func() tea.Msg {
		if trace != nil {
			trace()
		}
		if err := op(); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: okMsg}
	}
}

// styleFormInput applies consistent prompt and cursor styling to a textinput,
// matching the filter prompt's coloured chevron and cursor.
func styleFormInput(ti *textinput.Model) {
	ti.Prompt = "» "
	s := ti.Styles()
	s.Focused.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	s.Blurred.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true)
	s.Cursor.Color = lipgloss.Color("33")
	s.Cursor.Shape = tea.CursorBlock
	s.Cursor.Blink = true
	ti.SetStyles(s)
	ti.SetVirtualCursor(false)
}

func splitLines(input string) []string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
