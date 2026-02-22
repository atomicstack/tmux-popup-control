package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const previewMaxDisplayLines = 20

type styledLine struct {
	text          string
	style         *lipgloss.Style
	highlightFrom int
}

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
	lines := make([]styledLine, 0, 16)
	if header != "" {
		lines = append(lines, styledLine{text: header, style: styles.Header})
	}
	if m.loading {
		label := m.pendingLabel
		if label == "" {
			label = m.pendingID
		}
		if label == "" {
			label = "items"
		}
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
				prefix := "  "
				lineStyle := styles.Item
				selectDisplay := ""
				if current.MultiSelect {
					mark := " "
					if current.IsSelected(item.ID) {
						mark = "✓"
					}
					selectDisplay = fmt.Sprintf("[%s] ", mark)
				}
				if idx == current.Cursor {
					prefix = "▌ "
					lineStyle = styles.SelectedItem
				}
				lineText := selectDisplay + item.Label
				fullText := fmt.Sprintf("%s%s", prefix, lineText)
				highlightFrom := 0
				if lineStyle == styles.SelectedItem {
					highlightFrom = len([]rune(prefix)) + len([]rune(selectDisplay))
				}
				lines = append(lines, styledLine{text: fullText, style: lineStyle, highlightFrom: highlightFrom})
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
				lines = append(lines, styledLine{text: line, style: bodyStyle})
			}
		}
	}
	if m.errMsg != "" {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: fmt.Sprintf("Error: %s", m.errMsg), style: styles.Error})
	}
	if info := m.currentInfo(); info != "" {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: info, style: styles.Info})
	}
	if m.showFooter {
		lines = append(lines, styledLine{})
		lines = append(lines, styledLine{text: "↑/↓ move  enter select  tab mark  backspace clear  esc back  ctrl+c quit", style: styles.Footer})
	}

	promptText, _ := m.filterPrompt()
	lines = append(lines, styledLine{text: promptText})

	lines = limitHeight(lines, m.height, m.width)
	lines = applyWidth(lines, m.width)
	return renderLines(lines)
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
	used := 1 // filter prompt
	if header := m.menuHeader(); header != "" {
		used++
	}
	if m.errMsg != "" {
		used += 2
	}
	if info := m.currentInfo(); info != "" {
		used += 2
	}
	if m.showFooter {
		used += 2
	}
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
	out := make([]styledLine, len(lines))
	for i, line := range lines {
		out[i] = styledLine{
			text:          truncateText(line.text, width),
			style:         line.style,
			highlightFrom: line.highlightFrom,
		}
	}
	return out
}

func renderLines(lines []styledLine) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		text := line.text
		if line.style != nil {
			runes := []rune(text)
			if line.highlightFrom <= 0 || line.highlightFrom >= len(runes) {
				text = line.style.Render(text)
			} else {
				head := string(runes[:line.highlightFrom])
				tail := string(runes[line.highlightFrom:])
				text = head + line.style.Render(tail)
			}
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
