package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/charmbracelet/x/ansi"
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
	previewCursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
)

type styledLine struct {
	text          string
	style         *lipgloss.Style
	prefixStyle   *lipgloss.Style
	highlightFrom int
	raw           bool   // text contains ANSI escapes; skip style wrapping, use ANSI-aware truncation
	suffix        string // pre-rendered fragment appended verbatim after the styled body (e.g. scrollbar cell)
}

// hasSidePreview reports whether the current level should be rendered with the
// preview panel on the right rather than inline below the items.
func (m *Model) hasSidePreview() bool {
	current := m.currentLevel()
	if current == nil {
		return false
	}
	kind := previewKindForLevel(current.ID)
	if kind == previewKindNone || kind == previewKindLayout {
		return false
	}
	return m.previewPanelWidth() > 0
}

// previewPanelWidth returns the width in columns for the right-hand preview
// panel.  Returns 0 when the terminal is too narrow to split.
func (m *Model) previewPanelWidth() int {
	if m.noPreview || m.width <= 0 {
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

func (m *Model) wrapView(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// View implements tea.Model.
func (m *Model) View() (view tea.View) {
	span := logging.StartSpan("ui", "view", logging.SpanOptions{
		Attrs: map[string]any{
			"mode":        m.mode.String(),
			"stack_depth": len(m.stack),
			"width":       m.width,
			"height":      m.height,
		},
	})
	defer func() {
		span.AddAttr("content_len", len(view.Content))
		span.End(nil)
	}()

	header := m.menuHeader()
	var content string
	switch m.mode {
	case ModePaneForm:
		if m.paneForm != nil {
			content = m.viewPaneFormWithHeader(header)
			return m.wrapView(content)
		}
	case ModePaneCaptureForm:
		if m.paneCaptureForm != nil {
			content = m.viewPaneCaptureForm(header)
			return m.wrapView(content)
		}
	case ModeCommandOutput:
		content = m.viewCommandOutput(header)
		return m.wrapView(content)
	case ModeWindowForm:
		if m.windowForm != nil {
			content = m.viewWindowFormWithHeader(header)
			return m.wrapView(content)
		}
	case ModeSessionForm:
		if m.sessionForm != nil {
			content = m.viewSessionFormWithHeader(header)
			return m.wrapView(content)
		}
	case ModePluginConfirm:
		content = m.pluginConfirmView()
		return m.wrapView(content)
	case ModePluginInstall:
		content = m.pluginInstallView()
		return m.wrapView(content)
	case ModeResurrect:
		content = m.resurrectView()
		return m.wrapView(content)
	case ModeSessionSaveForm:
		if m.saveForm != nil {
			content = m.viewSaveForm()
		}
		return m.wrapView(content)
	}
	if m.hasSidePreview() {
		content = m.viewSideBySide(header)
		return m.wrapView(content)
	}
	content = m.viewVertical(header)
	return m.wrapView(content)
}

// viewVertical is the standard single-column layout with an optional inline
// preview block below the menu items (used when the terminal is too narrow for
// side-by-side, or on non-preview menu levels).
func (m *Model) viewVertical(header string) string {
	lines := make([]styledLine, 0, 16)
	if header != "" {
		lines = append(lines, styledLine{text: header, style: styles.Header})
	}
	if current := m.currentLevel(); current != nil && current.Subtitle != "" {
		dimStyle := lipgloss.NewStyle().Faint(true)
		lines = append(lines, styledLine{text: current.Subtitle, style: &dimStyle})
	}
	if current := m.currentLevel(); current != nil {
		m.syncViewport(current)
		start := 0
		displayItems := current.Items
		if maxItems := m.maxVisibleItems(); maxItems > 0 && len(displayItems) > maxItems {
			start = current.ViewportOffset
			start = max(start, 0)
			if start+maxItems > len(displayItems) {
				start = len(displayItems) - maxItems
				start = max(start, 0)
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
		} else if isTreeLevel(current.ID) {
			ts, _ := current.Data.(*menu.TreeState)
			lines = append(lines, m.renderTreeView(treeRenderOptions{
				LevelID:        current.ID,
				Items:          current.Items,
				State:          ts,
				CursorIdx:      current.Cursor,
				Width:          m.width,
				ViewportOffset: current.ViewportOffset,
				MaxVisible:     m.maxVisibleItems(),
			})...)
		} else {
			scrollCells := renderScrollbar(len(current.Items), len(displayItems), start)
			itemWidth := m.width
			if scrollCells != nil && itemWidth > 0 {
				itemWidth = m.width - 1
			}
			for i, item := range displayItems {
				idx := start + i
				line := m.buildItemLine(item, idx, current, itemWidth)
				if scrollCells != nil {
					line.suffix = scrollbarStyle.Render(string(scrollCells[i]))
				}
				lines = append(lines, line)
			}
		}
	}
	if preview := m.activePreview(); !m.noPreview && shouldRenderPreview(preview) {
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
			displayLines, displayStart := previewDisplayLines(preview)
			cursorOn := m.previewCursorVisible()
			for idx, line := range displayLines {
				if rendered, raw := renderPreviewLine(preview, line, displayStart+idx, bodyStyle, cursorOn); raw {
					lines = append(lines, styledLine{text: rendered, raw: true})
				} else {
					lines = append(lines, styledLine{text: rendered, style: bodyStyle})
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
	lines = limitHeight(lines, m.height-m.bottomBarRows(), m.width)
	lines = applyWidth(lines, m.width)

	// Bottom bar: error/status line + filter prompt.
	lines = append(lines, m.renderBottomBarLines()...)
	return m.overlayCompletion(renderLines(lines))
}

// viewSideBySide renders the menu on the left and a preview panel on the right.
func (m *Model) viewSideBySide(header string) string {
	menuW := m.menuColumnWidth()
	prevW := m.previewPanelWidth()

	// Bottom bar spans the full terminal width beneath both columns.
	bottomBarRows := m.bottomBarRows()

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
			start = max(start, 0)
			if start+maxItems > len(displayItems) {
				start = len(displayItems) - maxItems
				start = max(start, 0)
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
		} else if isTreeLevel(current.ID) {
			ts, _ := current.Data.(*menu.TreeState)
			contentLines = append(contentLines, m.renderTreeView(treeRenderOptions{
				LevelID:        current.ID,
				Items:          current.Items,
				State:          ts,
				CursorIdx:      current.Cursor,
				Width:          menuW,
				ViewportOffset: current.ViewportOffset,
				MaxVisible:     m.maxVisibleItems(),
			})...)
		} else {
			scrollCells := renderScrollbar(len(current.Items), len(displayItems), start)
			itemWidth := menuW
			if scrollCells != nil && itemWidth > 0 {
				itemWidth = menuW - 1
			}
			for i, item := range displayItems {
				idx := start + i
				line := m.buildItemLine(item, idx, current, itemWidth)
				if scrollCells != nil {
					line.suffix = scrollbarStyle.Render(string(scrollCells[i]))
				}
				contentLines = append(contentLines, line)
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
	panelH = max(panelH, 1)
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
			leftRows[i] = ansi.Truncate(row, menuW-1, "…")
		} else if w < menuW {
			leftRows[i] = row + strings.Repeat(" ", menuW-w)
		}
	}
	leftStr = strings.Join(leftRows, "\n")

	// --- Right column: preview panel ---
	rightStr := m.renderPreviewPanel(m.activePreview(), prevW, panelH)

	topSection := lipgloss.JoinHorizontal(lipgloss.Top, leftStr, rightStr)

	bottomStr := renderLines(m.renderBottomBarLines())

	return m.overlayCompletion(topSection + "\n" + bottomStr)
}

func (m *Model) viewCommandOutput(header string) string {
	lines := make([]styledLine, 0, len(m.commandOutputLines)+3)
	title := "output"
	if header != "" {
		title = header + menuHeaderSeparator + title
	}
	lines = append(lines, styledLine{text: title, style: styles.Header})
	if m.commandOutputTitle != "" {
		subtitleStyle := styles.FilterPlaceholder
		if subtitleStyle == nil {
			fallback := lipgloss.NewStyle().Faint(true)
			subtitleStyle = &fallback
		}
		lines = append(lines, styledLine{text: m.commandOutputTitle, style: subtitleStyle})
	}

	pageSize := m.commandOutputPageSize()
	start := m.commandOutputOffset
	start = max(start, 0)
	if maxOffset := m.maxCommandOutputOffset(); start > maxOffset {
		start = maxOffset
		m.commandOutputOffset = start
	}
	end := start + pageSize
	end = min(end, len(m.commandOutputLines))
	if start >= end {
		lines = append(lines, styledLine{text: "(no output)", style: styles.Info})
	} else {
		bodyStyle := styles.Info
		if styles.PreviewBody != nil {
			bodyStyle = styles.PreviewBody
		}
		for _, line := range m.commandOutputLines[start:end] {
			if decorated, ok := decorateShowOptionsLine(line, bodyStyle); ok {
				lines = append(lines, styledLine{text: decorated, raw: true})
				continue
			}
			lines = append(lines, styledLine{text: line, style: bodyStyle})
		}
	}

	rangeStart := 0
	if len(m.commandOutputLines) > 0 && start < end {
		rangeStart = start + 1
	}
	footer := fmt.Sprintf(
		"↑/↓ scroll  pgup/pgdown page  home/end jump  esc back  %d-%d/%d",
		rangeStart,
		end,
		len(m.commandOutputLines),
	)
	lines = append(lines, styledLine{text: footer, style: styles.Footer})
	lines = applyWidth(lines, m.width)
	lines = limitHeight(lines, m.height, m.width)
	return renderLines(lines)
}

func (m *Model) bottomBarRows() int {
	rows := 2
	if m.currentCommandSummary() != "" {
		rows++
	}
	return rows
}

func (m *Model) renderBottomBarLines() []styledLine {
	var statusLine styledLine
	if m.errMsg != "" {
		statusLine = styledLine{text: fmt.Sprintf("Error: %s", m.errMsg), style: styles.Error}
	}

	promptText, _ := m.filterPrompt()
	lines := []styledLine{
		statusLine,
		{text: promptText},
	}
	if summary := m.currentCommandSummary(); summary != "" {
		summaryStyle := styles.FilterPlaceholder
		if summaryStyle == nil {
			fallback := lipgloss.NewStyle().Faint(true)
			summaryStyle = &fallback
		}
		lines = append(lines, styledLine{text: summary, style: summaryStyle})
	}
	return applyWidth(lines, m.width)
}

// buildItemLine constructs a single styledLine for a menu item.
// width is the target column width; when > 0 the text is padded so that
// the selected item's background spans the full container.
func (m *Model) buildItemLine(item menu.Item, idx int, current *level, width int) styledLine {
	if item.Header {
		fullText := "  " + item.Label
		if width > 0 {
			if pad := width - lipgloss.Width(fullText); pad > 0 {
				fullText += strings.Repeat(" ", pad)
			}
		}
		return styledLine{
			text:  fullText,
			style: styles.HeaderItem,
		}
	}
	indicator := "▌"
	lineStyle := styles.Item
	indicatorStyle := styles.ItemIndicator
	if idx == current.Cursor {
		indicatorStyle = styles.SelectedItemIndicator
		lineStyle = styles.SelectedItem
	}
	opts := itemLineOptions{
		Indicator:      indicator,
		LineStyle:      lineStyle,
		IndicatorStyle: indicatorStyle,
		Current:        current,
		Width:          width,
	}
	if current.MultiSelect {
		return m.buildMultiSelectLine(item, opts)
	}
	if item.StyledLabel != "" {
		return buildStyledNormalLine(item, opts)
	}
	displayLabel := item.Label
	fullText := indicator + " " + displayLabel
	if width > 0 {
		visibleText := indicator + " " + item.Label
		if pad := width - lipgloss.Width(visibleText); pad > 0 {
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

type itemLineOptions struct {
	Indicator      string
	LineStyle      *lipgloss.Style
	IndicatorStyle *lipgloss.Style
	Current        *level
	Width          int
}

func buildStyledNormalLine(item menu.Item, opts itemLineOptions) styledLine {
	bodyContent := item.StyledLabel
	if opts.Width > 0 {
		visWidth := lipgloss.Width(opts.Indicator + " " + item.Label)
		if pad := opts.Width - visWidth; pad > 0 {
			bodyContent += strings.Repeat(" ", pad)
		}
	}

	styledIndicator := opts.Indicator
	if opts.IndicatorStyle != nil {
		styledIndicator = opts.IndicatorStyle.Render(opts.Indicator)
	}

	styledBody := " " + bodyContent
	if opts.LineStyle != nil {
		styledBody = opts.LineStyle.Render(styledBody)
	}

	return styledLine{
		text: styledIndicator + styledBody,
		raw:  true,
	}
}

// buildMultiSelectLine renders a multi-select item with a styled checkbox.
// Each segment (indicator, checkbox, body) is rendered independently to avoid
// ANSI nesting issues where an inner reset would break the outer background.
// The result is marked raw so applyWidth uses ANSI-aware truncation.
func (m *Model) buildMultiSelectLine(item menu.Item, opts itemLineOptions) styledLine {
	var cbChar string
	var cbBaseStyle *lipgloss.Style
	if opts.Current != nil && opts.Current.IsSelected(item.ID) {
		cbChar = "■"
		cbBaseStyle = styles.CheckboxChecked
	} else {
		cbChar = "□"
		cbBaseStyle = styles.Checkbox
	}
	// Composite checkbox style: checkbox fg/bold + line background.
	cbStyle := *cbBaseStyle
	if opts.LineStyle != nil {
		cbStyle = cbStyle.Inherit(*opts.LineStyle)
	}

	// Use plain Label for width measurement, StyledLabel for display.
	displayLabel := item.Label
	if item.StyledLabel != "" {
		displayLabel = item.StyledLabel
	}
	bodyContent := displayLabel
	if opts.Width > 0 {
		visWidth := lipgloss.Width(opts.Indicator + " " + cbChar + " " + item.Label)
		if pad := opts.Width - visWidth; pad > 0 {
			bodyContent += strings.Repeat(" ", pad)
		}
	}

	styledIndicator := opts.Indicator
	if opts.IndicatorStyle != nil {
		styledIndicator = opts.IndicatorStyle.Render(opts.Indicator)
	}
	var styledBody string
	if opts.LineStyle != nil {
		styledBody = opts.LineStyle.Render(" ") + cbStyle.Render(cbChar) + opts.LineStyle.Render(" "+bodyContent)
	} else {
		styledBody = " " + cbStyle.Render(cbChar) + " " + bodyContent
	}
	return styledLine{
		text: styledIndicator + styledBody,
		raw:  true,
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
			maxOffset = max(maxOffset, 0)
			if preview.scrollOffset > maxOffset {
				preview.scrollOffset = maxOffset
			}
			if preview.scrollOffset < 0 {
				preview.scrollOffset = 0
			}
			end := preview.scrollOffset + innerH
			end = min(end, len(preview.lines))
			contentLines = preview.lines[preview.scrollOffset:end]
			lastVisible := preview.scrollOffset + len(contentLines)
			scrollInfo = fmt.Sprintf(" %d/%d ", lastVisible, len(preview.lines))
		}
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
		rawLine := rawANSI
		if i < len(contentLines) {
			content = contentLines[i]
		}
		if preview != nil && preview.err == "" && i < len(contentLines) {
			if rendered, renderedRaw := renderPreviewLine(preview, content, preview.scrollOffset+i, bodyStyle, m.previewCursorVisible()); rendered != "" {
				content = rendered
				rawLine = renderedRaw
			}
		}
		// Truncate and pad using ANSI-aware measurement when content
		// may contain escape sequences from capture-pane -e.
		w := lipgloss.Width(content)
		if w > innerW {
			content = ansi.Truncate(content, innerW-1, "…")
			w = lipgloss.Width(content)
		}
		if w < innerW {
			content = content + strings.Repeat(" ", innerW-w)
		}
		var styledContent string
		if rawLine {
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
	ev, ok := msg.(tea.MouseWheelMsg)
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
	innerH = max(innerH, 1)
	switch ev.Button {
	case tea.MouseWheelUp:
		preview.scrollOffset -= 3
		if preview.scrollOffset < 0 {
			preview.scrollOffset = 0
		}
	case tea.MouseWheelDown:
		maxOffset := len(preview.lines) - innerH
		maxOffset = max(maxOffset, 0)
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
	return false
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

func previewDisplayLines(data *previewData) ([]string, int) {
	lines := data.lines
	if len(lines) == 0 {
		return []string{}, 0
	}
	start := 0
	if previewMaxDisplayLines > 0 && len(lines) > previewMaxDisplayLines {
		start = len(lines) - previewMaxDisplayLines
		return lines[start:], start
	}
	return lines, start
}

func (m *Model) previewCursorVisible() bool {
	return !m.filterCursor.IsBlinked
}

func renderPreviewLine(preview *previewData, line string, absoluteRow int, bodyStyle *lipgloss.Style, cursorOn bool) (string, bool) {
	if preview == nil || !preview.cursorVisible || !cursorOn || preview.cursorY != absoluteRow {
		return line, preview != nil && preview.rawANSI
	}
	lineWidth := lipgloss.Width(line)
	if preview.cursorX < 0 {
		return line, preview.rawANSI
	}
	if preview.cursorX > lineWidth {
		line = line + strings.Repeat(" ", preview.cursorX-lineWidth)
		lineWidth = preview.cursorX
	}

	left := ansi.Cut(line, 0, preview.cursorX)
	right := ""
	if preview.cursorX+1 < lineWidth {
		right = ansi.Cut(line, preview.cursorX+1, lineWidth)
	}
	block := previewCursorStyle.Render("█")

	if preview.rawANSI {
		return left + block + right, true
	}
	if bodyStyle != nil {
		return bodyStyle.Render(left) + block + bodyStyle.Render(right), true
	}
	return left + block + right, true
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
	m.dismissCompletion()
	if current := m.currentLevel(); current != nil {
		m.syncViewport(current)
	}
	return nil
}

func (m *Model) overlayCompletion(rendered string) string {
	if !m.completionVisible() {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	barStart := len(lines) - m.bottomBarRows()
	barStart = max(barStart, 0)
	spaceAbove := barStart
	spaceBelow := 0
	if m.height > len(lines) {
		spaceBelow = m.height - len(lines)
	}

	maxW := m.width - m.completion.anchorCol
	maxW = max(maxW, 20)

	maxH := m.height - 4
	maxH = max(maxH, 3)
	naturalDropdown := m.completion.view(maxW, maxH)
	if naturalDropdown == "" {
		return rendered
	}

	dropLines := strings.Split(naturalDropdown, "\n")
	placeBelow := spaceBelow > 0 && len(dropLines) > spaceAbove

	if placeBelow {
		dropdown := m.completion.view(maxW, spaceBelow)
		if dropdown == "" {
			return rendered
		}
		dropLines = strings.Split(dropdown, "\n")
		insertStart := len(lines)
		for len(lines) < insertStart+len(dropLines) {
			lines = append(lines, "")
		}
		for idx, line := range dropLines {
			lineIdx := insertStart + idx
			if lineIdx >= len(lines) {
				break
			}
			lines[lineIdx] = overlayAt(lines[lineIdx], line, m.completion.anchorCol, m.width)
		}
		return strings.Join(lines, "\n")
	}

	if spaceAbove > 0 {
		dropdown := m.completion.view(maxW, spaceAbove)
		if dropdown == "" {
			return rendered
		}
		dropLines = strings.Split(dropdown, "\n")
	}

	insertEnd := barStart
	insertStart := insertEnd - len(dropLines)
	insertStart = max(insertStart, 0)
	for idx, line := range dropLines {
		lineIdx := insertStart + idx
		if lineIdx < 0 || lineIdx >= len(lines) {
			continue
		}
		lines[lineIdx] = overlayAt(lines[lineIdx], line, m.completion.anchorCol, m.width)
	}

	return strings.Join(lines, "\n")
}

func overlayAt(baseLine, overlayStr string, col, maxWidth int) string {
	if col < 0 {
		col = 0
	}
	prefix := ansi.Truncate(baseLine, col, "")
	if pad := col - lipgloss.Width(prefix); pad > 0 {
		prefix += strings.Repeat(" ", pad)
	}
	line := prefix + overlayStr
	if maxWidth > 0 && lipgloss.Width(line) > maxWidth {
		line = ansi.Truncate(line, maxWidth, "")
	}
	return line
}

func (m *Model) maxVisibleItems() int {
	if m.height <= 0 {
		return -1
	}
	used := m.bottomBarRows()
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
				displayLines, _ := previewDisplayLines(preview)
				used += len(displayLines)
			}
		} else if current := m.currentLevel(); current != nil {
			kind := previewKindForLevel(current.ID)
			if kind != previewKindNone && kind != previewKindLayout {
				// Reserve space for the preview that is about to load.
				used += 3 // blank + title + "Loading preview…"
			}
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
		// Reserve visible columns for any pre-rendered suffix so truncation of
		// the main text accounts for the scrollbar cell or similar.
		cap := width
		if line.suffix != "" {
			cap = width - lipgloss.Width(line.suffix)
			cap = max(cap, 0)
		}
		if line.raw {
			w := lipgloss.Width(text)
			if w > cap {
				text = ansi.Truncate(text, cap-1, "…")
			}
		} else {
			text = truncateText(text, cap)
		}
		result[i] = styledLine{
			text:          text,
			style:         line.style,
			prefixStyle:   line.prefixStyle,
			highlightFrom: line.highlightFrom,
			raw:           line.raw,
			suffix:        line.suffix,
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
			out[i] = text + line.suffix
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
		out[i] = text + line.suffix
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
