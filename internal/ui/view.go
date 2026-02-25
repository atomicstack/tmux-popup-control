package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

const (
	previewMaxDisplayLines = 20  // used by inline (vertical) preview only
	previewPanelMinWidth   = 40  // minimum cols for the preview panel; below this no split
	previewPanelFraction   = 0.6 // fraction of total width given to the preview panel
)

// previewBorder styles used when drawing the preview box.
var (
	previewBorderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	previewScrollStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

type styledLine struct {
	text          string
	style         *lipgloss.Style
	prefixStyle   *lipgloss.Style
	highlightFrom int
	raw           bool // text contains ANSI escapes; skip style wrapping, use ANSI-aware truncation
}

// hasSidePreview reports whether the current level should be rendered with the
// preview panel on the right rather than inline below the items.
func (m *Model) hasSidePreview() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	if previewKindForLevel(current.ID) == previewKindNone {
		return false
	}
	return m.previewPanelWidth() > 0
}

// previewPanelWidth returns the width in columns for the right-hand preview
// panel.  Returns 0 when the terminal is too narrow to split.
func (m *Model) previewPanelWidth() int {
	if m.width <= 0 {
		return 0
	}
	w := int(float64(m.width) * previewPanelFraction)
	if w < previewPanelMinWidth {
		return 0
	}
	return w
}

// menuColumnWidth returns the width available for the left-hand menu column.
func (m *Model) menuColumnWidth() int {
	return m.width - m.previewPanelWidth()
}

// View implements tea.Model.
func (m *Model) View() string {
	header := m.menuHeader()
	switch m.mode {
	case ModePaneForm:
		if m.paneForm != nil {
			return m.viewPaneFormWithHeader(header)
		}
	case ModeWindowForm:
		if m.windowForm != nil {
			return m.viewWindowFormWithHeader(header)
		}
	case ModeSessionForm:
		if m.sessionForm != nil {
			return m.viewSessionFormWithHeader(header)
		}
	}
	if m.hasSidePreview() {
		return m.viewSideBySide(header)
	}
	return m.viewVertical(header)
}

// viewVertical is the standard single-column layout with an optional inline
// preview block below the menu items (used when the terminal is too narrow for
// side-by-side, or on non-preview menu levels).
func (m *Model) viewVertical(header string) string {
	lines := make([]styledLine, 0, 16)
	if header != "" {
		lines = append(lines, styledLine{text: header, style: styles.Header})
	}
	if current := m.currentLevel(); current != nil {
		m.syncViewport(current)
		start := 0
		displayItems := current.Items
		if maxItems := m.maxVisibleItems(); maxItems > 0 && len(displayItems) > maxItems {
			start = current.ViewportOffset
			if start < 0 {
				start = 0
			}
			if start+maxItems > len(displayItems) {
				start = len(displayItems) - maxItems
				if start < 0 {
					start = 0
				}
				current.ViewportOffset = start
			}
			displayItems = displayItems[start : start+maxItems]
		}
		if len(current.Items) == 0 {
			msg := "(no entries)"
			if current.Filter != "" {
				msg = fmt.Sprintf("No matches for %q", current.Filter)
			}
			lines = append(lines, styledLine{text: msg, style: styles.Info})
		} else {
			for i, item := range displayItems {
				idx := start + i
				lines = append(lines, m.buildItemLine(item.ID, item.Label, idx, current, m.width))
			}
		}
	}
	if preview := m.activePreview(); shouldRenderPreview(preview) {
		lines = append(lines, styledLine{})
		title := previewTitleText(preview)
		titleStyle := styles.Info
		if styles.PreviewTitle != nil {
			titleStyle = styles.PreviewTitle
		}
		lines = append(lines, styledLine{text: title, style: titleStyle})
		if preview.err != "" {
			errStyle := styles.Error
			if styles.PreviewError != nil {
				errStyle = styles.PreviewError
			}
			lines = append(lines, styledLine{text: preview.err, style: errStyle})
		} else {
			bodyStyle := styles.Info
			if styles.PreviewBody != nil {
				bodyStyle = styles.PreviewBody
			}
			for _, line := range previewDisplayLines(preview) {
				if preview.rawANSI {
					lines = append(lines, styledLine{text: line, raw: true})
				} else {
					lines = append(lines, styledLine{text: line, style: bodyStyle})
				}
			}
		}
	}
	if info := m.currentInfo(); info != "" {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: info, style: styles.Info})
	}
	if m.showFooter {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: "↑/↓ move  enter select  tab mark  backspace clear  esc back  ctrl+c quit", style: styles.Footer})
	}
	// Reserve 3 rows for the bottom bar (blank + error/status + prompt).
	lines = limitHeight(lines, m.height-2, m.width)
	lines = applyWidth(lines, m.width)

	// Bottom bar: error/status line + filter prompt.
	var statusLine styledLine
	if m.errMsg != "" {
		statusLine = styledLine{text: fmt.Sprintf("Error: %s", m.errMsg), style: styles.Error}
	}
	promptText, _ := m.filterPrompt()
	bottomLines := []styledLine{
		statusLine,
		{text: promptText},
	}
	bottomLines = applyWidth(bottomLines, m.width)
	lines = append(lines, bottomLines...)
	return renderLines(lines)
}

// viewSideBySide renders the menu on the left and a preview panel on the right.
func (m *Model) viewSideBySide(header string) string {
	menuW := m.menuColumnWidth()
	prevW := m.previewPanelWidth()

	// Bottom bar: status/error line + filter prompt.
	// These span the full terminal width beneath both columns.
	const bottomBarRows = 2

	// --- Left column: menu items, info, footer ---
	contentLines := make([]styledLine, 0, 16)
	if header != "" {
		contentLines = append(contentLines, styledLine{text: header, style: styles.Header})
	}
	if current := m.currentLevel(); current != nil {
		m.syncViewport(current)
		start := 0
		displayItems := current.Items
		if maxItems := m.maxVisibleItems(); maxItems > 0 && len(displayItems) > maxItems {
			start = current.ViewportOffset
			if start < 0 {
				start = 0
			}
			if start+maxItems > len(displayItems) {
				start = len(displayItems) - maxItems
				if start < 0 {
					start = 0
				}
				current.ViewportOffset = start
			}
			displayItems = displayItems[start : start+maxItems]
		}
		if len(current.Items) == 0 {
			msg := "(no entries)"
			if current.Filter != "" {
				msg = fmt.Sprintf("No matches for %q", current.Filter)
			}
			contentLines = append(contentLines, styledLine{text: msg, style: styles.Info})
		} else {
			for i, item := range displayItems {
				idx := start + i
				contentLines = append(contentLines, m.buildItemLine(item.ID, item.Label, idx, current, menuW))
			}
		}
	}
	if info := m.currentInfo(); info != "" {
		contentLines = append(contentLines, styledLine{})
		contentLines = append(contentLines, styledLine{text: info, style: styles.Info})
	}
	if m.showFooter {
		contentLines = append(contentLines, styledLine{})
		contentLines = append(contentLines, styledLine{text: "↑/↓ move  enter select  tab mark  backspace clear  esc back  ctrl+c quit", style: styles.Footer})
	}

	// Pad content lines so the columns fill the space above the bottom bar.
	panelH := m.height - bottomBarRows
	if panelH < 1 {
		panelH = 1
	}
	if len(contentLines) > panelH {
		contentLines = contentLines[:panelH]
	}
	for len(contentLines) < panelH {
		contentLines = append(contentLines, styledLine{})
	}

	// Apply width to content lines only — they have no embedded ANSI codes
	// (styling lives in the styledLine.style field), so rune-based truncation
	// is correct.
	contentLines = applyWidth(contentLines, menuW)
	leftStr := renderLines(contentLines)

	// Pad/truncate every rendered row to exactly menuW visible columns so
	// JoinHorizontal keeps the preview panel flush to the right edge
	// regardless of content length or cursor-blink state. Uses lipgloss.Width
	// (ANSI-aware visual measurement) and reflow/truncate (ANSI-safe truncation).
	leftRows := strings.Split(leftStr, "\n")
	for i, row := range leftRows {
		w := lipgloss.Width(row)
		if w > menuW {
			leftRows[i] = truncate.StringWithTail(row, uint(menuW-1), "…")
		} else if w < menuW {
			leftRows[i] = row + strings.Repeat(" ", menuW-w)
		}
	}
	leftStr = strings.Join(leftRows, "\n")

	// --- Right column: preview panel ---
	rightStr := m.renderPreviewPanel(m.activePreview(), prevW, panelH)

	topSection := lipgloss.JoinHorizontal(lipgloss.Top, leftStr, rightStr)

	// --- Bottom bar: error/status + prompt (full width) ---
	var statusLine styledLine
	if m.errMsg != "" {
		statusLine = styledLine{text: fmt.Sprintf("Error: %s", m.errMsg), style: styles.Error}
	}
	promptText, _ := m.filterPrompt()
	bottomLines := []styledLine{
		statusLine,
		{text: promptText},
	}
	bottomLines = applyWidth(bottomLines, m.width)
	bottomStr := renderLines(bottomLines)

	return topSection + "\n" + bottomStr
}

// buildItemLine constructs a single styledLine for a menu item.
// width is the target column width; when > 0 the text is padded so that
// the selected item's background spans the full container.
func (m *Model) buildItemLine(id, label string, idx int, current *level, width int) styledLine {
	indicator := "▌"
	lineStyle := styles.Item
	indicatorStyle := styles.ItemIndicator
	selectDisplay := ""
	if current.MultiSelect {
		mark := " "
		if current.IsSelected(id) {
			mark = "✓"
		}
		selectDisplay = fmt.Sprintf("[%s] ", mark)
	}
	if idx == current.Cursor {
		indicatorStyle = styles.SelectedItemIndicator
		lineStyle = styles.SelectedItem
	}
	fullText := indicator + " " + selectDisplay + label
	if width > 0 {
		if pad := width - len([]rune(fullText)); pad > 0 {
			fullText += strings.Repeat(" ", pad)
		}
	}
	return styledLine{
		text:          fullText,
		style:         lineStyle,
		prefixStyle:   indicatorStyle,
		highlightFrom: 1, // just the ▌ character
	}
}

// renderPreviewPanel builds the bordered preview box as a string with exactly
// height rows and totalWidth columns.
func (m *Model) renderPreviewPanel(preview *previewData, totalWidth, height int) string {
	const (
		tlc = "╭"
		trc = "╮"
		blc = "╰"
		brc = "╯"
		hz  = "─"
		vt  = "│"
	)

	innerW := totalWidth - 2
	innerH := height - 2
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}

	// Gather content and metadata.
	titleLabel := "Preview"
	scrollInfo := ""
	var contentLines []string
	var errLine string

	if preview != nil {
		lbl := strings.TrimSpace(preview.label)
		if lbl == "" {
			lbl = strings.TrimSpace(preview.target)
		}
		if lbl != "" {
			titleLabel = "Preview: " + lbl
		}

		if preview.err != "" {
			errLine = preview.err
		} else if len(preview.lines) > 0 {
			// Clamp scroll offset.
			maxOffset := len(preview.lines) - innerH
			if maxOffset < 0 {
				maxOffset = 0
			}
			if preview.scrollOffset > maxOffset {
				preview.scrollOffset = maxOffset
			}
			if preview.scrollOffset < 0 {
				preview.scrollOffset = 0
			}
			end := preview.scrollOffset + innerH
			if end > len(preview.lines) {
				end = len(preview.lines)
			}
			contentLines = preview.lines[preview.scrollOffset:end]
			lastVisible := preview.scrollOffset + len(contentLines)
			scrollInfo = fmt.Sprintf(" %d/%d ", lastVisible, len(preview.lines))
		} else if preview.loading {
			contentLines = []string{"Loading…"}
		}
	} else {
		contentLines = []string{"Loading…"}
	}

	// Build top border: ╭─ title ──────────── scrollInfo ─╮
	// Available inner width: innerW (between the two corner chars).
	// We put 1 hz on each side of the title/scrollInfo.
	titleSeg := " " + titleLabel + " "
	scrollSeg := scrollInfo
	// Total fixed chars: tlc + hz + titleSeg + <dashes> + scrollSeg + hz + trc
	// = 2 + len(titleSeg) + dashes + len(scrollSeg) + 2 = totalWidth
	// → dashes = totalWidth - 4 - len(titleSeg) - len(scrollSeg)
	dashes := totalWidth - 4 - len([]rune(titleSeg)) - len([]rune(scrollSeg))
	if dashes < 0 {
		// Too narrow for scroll info; drop it.
		scrollSeg = ""
		dashes = totalWidth - 4 - len([]rune(titleSeg))
	}
	if dashes < 0 {
		// Still too narrow; truncate title.
		titleSeg = " … "
		dashes = totalWidth - 4 - len([]rune(titleSeg))
	}
	if dashes < 0 {
		dashes = 0
	}
	topLine := previewBorderStyle.Render(tlc+hz) +
		styles.PreviewTitle.Render(titleSeg) +
		previewBorderStyle.Render(strings.Repeat(hz, dashes)) +
		previewScrollStyle.Render(scrollSeg) +
		previewBorderStyle.Render(hz+trc)

	// Build bottom border: ╰────────────────────────────────╯
	bottomLine := previewBorderStyle.Render(blc + strings.Repeat(hz, innerW) + brc)

	// Determine body style and whether content has embedded ANSI.
	bodyStyle := styles.PreviewBody
	rawANSI := preview != nil && preview.rawANSI
	if errLine != "" {
		bodyStyle = styles.PreviewError
		contentLines = []string{errLine}
		rawANSI = false
	}

	// Build content rows — pad/truncate to innerH rows, each innerW wide.
	rows := make([]string, 0, height)
	rows = append(rows, topLine)
	for i := 0; i < innerH; i++ {
		var content string
		if i < len(contentLines) {
			content = contentLines[i]
		}
		// Truncate and pad using ANSI-aware measurement when content
		// may contain escape sequences from capture-pane -e.
		w := lipgloss.Width(content)
		if w > innerW {
			content = truncate.StringWithTail(content, uint(innerW-1), "…")
			w = lipgloss.Width(content)
		}
		if w < innerW {
			content = content + strings.Repeat(" ", innerW-w)
		}
		var styledContent string
		if rawANSI {
			styledContent = content
		} else if bodyStyle != nil {
			styledContent = bodyStyle.Render(content)
		} else {
			styledContent = content
		}
		rows = append(rows, previewBorderStyle.Render(vt)+styledContent+previewBorderStyle.Render(vt))
	}
	rows = append(rows, bottomLine)
	return strings.Join(rows, "\n")
}

// handleMouseMsg handles mouse wheel events to scroll the preview panel.
func (m *Model) handleMouseMsg(msg tea.Msg) tea.Cmd {
	ev, ok := msg.(tea.MouseMsg)
	if !ok {
		return nil
	}
	if !m.hasSidePreview() {
		return nil
	}
	preview := m.activePreview()
	if preview == nil || preview.loading {
		return nil
	}
	innerH := m.height - 2
	if innerH < 1 {
		innerH = 1
	}
	switch ev.Button {
	case tea.MouseButtonWheelUp:
		preview.scrollOffset -= 3
		if preview.scrollOffset < 0 {
			preview.scrollOffset = 0
		}
	case tea.MouseButtonWheelDown:
		maxOffset := len(preview.lines) - innerH
		if maxOffset < 0 {
			maxOffset = 0
		}
		preview.scrollOffset += 3
		if preview.scrollOffset > maxOffset {
			preview.scrollOffset = maxOffset
		}
	}
	return nil
}

func (m *Model) menuHeader() string {
	segments := m.headerSegments()
	if len(segments) == 0 {
		return ""
	}
	return strings.Join(segments, menuHeaderSeparator)
}

func (m *Model) headerSegments() []string {
	depth := len(m.stack)
	if depth == 0 {
		return nil
	}
	root := strings.TrimSpace(m.rootTitle)
	if root == "" {
		root = defaultRootTitle
	}
	if depth == 1 {
		return []string{root}
	}
	segments := make([]string, 0, depth)
	if m.rootMenuID != "" {
		segments = append(segments, root)
	}
	for i := 1; i < depth; i++ {
		segment := headerSegmentForLevel(m.stack[i])
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
	}
	if len(segments) == 0 {
		return []string{root}
	}
	return segments
}

func headerSegmentForLevel(l *level) string {
	if l == nil {
		return ""
	}
	candidate := strings.TrimSpace(l.ID)
	if candidate == "" {
		candidate = strings.TrimSpace(l.Title)
	}
	if candidate == "" {
		return ""
	}
	if idx := strings.LastIndex(candidate, ":"); idx >= 0 {
		candidate = candidate[idx+1:]
	}
	candidate = headerSegmentCleaner.Replace(candidate)
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	fields := strings.Fields(strings.ToLower(candidate))
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func shouldRenderPreview(data *previewData) bool {
	if data == nil {
		return false
	}
	if data.err != "" {
		return true
	}
	if len(data.lines) > 0 {
		return true
	}
	return data.loading
}

func previewTitleText(data *previewData) string {
	label := strings.TrimSpace(data.label)
	if label == "" {
		label = strings.TrimSpace(data.target)
	}
	if label == "" {
		label = "(unknown)"
	}
	status := ""
	if data.loading && data.err == "" {
		status = " (loading…)"
	}
	return fmt.Sprintf("Preview: %s%s", label, status)
}

func previewDisplayLines(data *previewData) []string {
	lines := data.lines
	if len(lines) == 0 {
		if data.loading {
			return []string{"Loading preview…"}
		}
		return []string{}
	}
	if previewMaxDisplayLines > 0 && len(lines) > previewMaxDisplayLines {
		return lines[len(lines)-previewMaxDisplayLines:]
	}
	return lines
}

func (m *Model) handleWindowSizeMsg(msg tea.Msg) tea.Cmd {
	resize, ok := msg.(tea.WindowSizeMsg)
	if !ok {
		return nil
	}
	if !m.fixedWidth {
		m.width = resize.Width
	}
	if !m.fixedHeight {
		m.height = resize.Height
	}
	if current := m.currentLevel(); current != nil {
		m.syncViewport(current)
	}
	return nil
}

func (m *Model) maxVisibleItems() int {
	if m.height <= 0 {
		return -1
	}
	used := 2 // bottom bar: error/status + filter prompt
	if header := m.menuHeader(); header != "" {
		used++
	}
	if info := m.currentInfo(); info != "" {
		used += 2
	}
	if m.showFooter {
		used += 2
	}
	// In side-by-side mode the full height is available for the left column;
	// no preview rows need to be reserved.
	if !m.hasSidePreview() {
		if preview := m.activePreview(); shouldRenderPreview(preview) {
			used += 2 // blank separator + title line
			if preview.err != "" {
				used++ // one line for the error text
			} else {
				used += len(previewDisplayLines(preview))
			}
		} else if current := m.currentLevel(); current != nil && previewKindForLevel(current.ID) != previewKindNone {
			// Reserve space for the preview that is about to load.
			used += 3 // blank + title + "Loading preview…"
		}
	}
	remain := m.height - used
	if remain < 1 {
		return 1
	}
	return remain
}

func (m *Model) setInfo(message string) {
	m.infoMsg = message
	m.infoExpire = time.Now().Add(5 * time.Second)
}

func (m *Model) clearInfo() {
	if m.infoMsg == "" {
		return
	}
	if !m.infoExpire.IsZero() && time.Now().Before(m.infoExpire) {
		return
	}
	m.infoMsg = ""
	m.infoExpire = time.Time{}
}

func (m *Model) forceClearInfo() {
	m.infoMsg = ""
	m.infoExpire = time.Time{}
}

func (m *Model) currentInfo() string {
	if m.infoMsg != "" && !m.infoExpire.IsZero() && time.Now().After(m.infoExpire) {
		m.infoMsg = ""
		m.infoExpire = time.Time{}
	}
	return m.infoMsg
}

func limitHeight(lines []styledLine, height, width int) []styledLine {
	if height <= 0 || len(lines) <= height {
		return lines
	}
	if height == 1 {
		return []styledLine{{text: truncateText("…", width)}}
	}
	trimmed := make([]styledLine, 0, height)
	trimmed = append(trimmed, lines[:height-1]...)
	trimmed = append(trimmed, styledLine{text: truncateText("…", width)})
	return trimmed
}

func applyWidth(lines []styledLine, width int) []styledLine {
	if width <= 0 {
		return lines
	}
	result := make([]styledLine, len(lines))
	for i, line := range lines {
		text := line.text
		if line.raw {
			w := lipgloss.Width(text)
			if w > width {
				text = truncate.StringWithTail(text, uint(width-1), "…")
			}
		} else {
			text = truncateText(text, width)
		}
		result[i] = styledLine{
			text:          text,
			style:         line.style,
			prefixStyle:   line.prefixStyle,
			highlightFrom: line.highlightFrom,
			raw:           line.raw,
		}
	}
	return result
}

func renderLines(lines []styledLine) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		text := line.text
		if line.raw {
			// Text already contains ANSI escapes; pass through as-is.
			out[i] = text
			continue
		}
		runes := []rune(text)
		if line.highlightFrom > 0 && line.highlightFrom < len(runes) {
			head := string(runes[:line.highlightFrom])
			tail := string(runes[line.highlightFrom:])
			if line.prefixStyle != nil {
				head = line.prefixStyle.Render(head)
			}
			if line.style != nil {
				tail = line.style.Render(tail)
			}
			text = head + tail
		} else if line.style != nil {
			text = line.style.Render(text)
		}
		out[i] = text
	}
	return strings.Join(out, "\n")
}

func truncateText(text string, width int) string {
	if width <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}
