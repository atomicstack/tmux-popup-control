package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// doubleClickWindow is how close two clicks on the same row must be to count
// as a double click (which acts as enter).
const doubleClickWindow = 400 * time.Millisecond

// mouseNow is the clock used for double-click detection; tests replace it.
var mouseNow = time.Now

// hitRect is a rectangle of screen cells; x1 and y1 are exclusive.
type hitRect struct {
	x0, y0, x1, y1 int
}

func (r hitRect) contains(x, y int) bool {
	return x >= r.x0 && x < r.x1 && y >= r.y0 && y < r.y1
}

// hitMap records where the clickable parts of the last rendered frame are.
// View rebuilds it on every render, so mouse handlers always test against what
// is on screen rather than re-deriving the layout.
type hitMap struct {
	// list is the block of item rows; listFirst is the item index of its top
	// row. treeLabelX maps an expandable tree item's index to the column its
	// name starts at; the cells before it are its connectors and arrow.
	list       hitRect
	listFirst  int
	treeLabelX map[int]int

	// preview is the side preview panel.
	preview hitRect

	// barTop is the first row of the bottom bar; subtitleRow is the row,
	// relative to it, of a control row such as the find tick box (-1: none).
	barTop      int
	subtitleRow int

	// dropdown is the item rows of an open dropdown (command completion or an
	// extract selector); dropdownFirst is the item index of its top row.
	dropdown      hitRect
	dropdownState *completionState
	dropdownFirst int

	// formInputRow is the row of the active form's text input (-1: none) and
	// formCheckbox the pane-capture form's tick box.
	formInputRow int
	formCheckbox hitRect

	// yn is where the buttons of the active y/n prompt are.
	yn ynButtons
}

func newHitMap() hitMap {
	return hitMap{subtitleRow: -1, formInputRow: -1}
}

// clickRecord remembers the last list click, for double-click detection.
type clickRecord struct {
	level *level
	item  string
	at    time.Time
}

// recordListHit notes where renderMenuLines' rows for current landed.
func (m *Model) recordListHit(current *level, top, width, rows int) {
	if current == nil || len(current.Items) == 0 || rows <= 0 {
		return
	}
	rows = min(rows, len(current.Items)-current.ViewportOffset)
	m.hit.list = hitRect{x0: 0, y0: top, x1: max(width, 1), y1: top + rows}
	m.hit.listFirst = current.ViewportOffset
}

// ynPromptActive reports whether a y/n prompt is on screen. While one is, the
// mouse only answers the prompt: clicks elsewhere are ignored.
func (m *Model) ynPromptActive() bool {
	switch m.mode {
	case ModeMenu:
		return m.confirmState != nil
	case ModePluginInstall:
		s := m.pluginInstallState
		if s == nil {
			return false
		}
		return pluginUninstallPromptText(s) != "" || (s.finished && len(s.installed) > 0)
	}
	return false
}

// answerYN answers the active y/n prompt exactly as the y or n key would.
func (m *Model) answerYN(choice ynChoice) tea.Cmd {
	key := tea.KeyPressMsg{Code: 'y', Text: "y"}
	if choice == ynNo {
		key = tea.KeyPressMsg{Code: 'n', Text: "n"}
	}
	m.ynHover = ynNone
	if m.mode == ModePluginInstall {
		_, cmd := m.handlePluginInstallKey(key)
		return cmd
	}
	return m.handleDeleteConfirmKey(key)
}

// handleMouseMsg routes mouse input: clicks select and activate, the wheel
// scrolls whatever is under the pointer, and motion drives button hover.
func (m *Model) handleMouseMsg(msg tea.Msg) tea.Cmd {
	switch ev := msg.(type) {
	case tea.MouseClickMsg:
		if ev.Button != tea.MouseLeft {
			return nil
		}
		return m.handleMouseClick(ev.X, ev.Y)
	case tea.MouseMotionMsg:
		if m.ynPromptActive() {
			m.ynHover = m.hit.yn.at(ev.X, ev.Y)
		}
	case tea.MouseWheelMsg:
		return m.handleMouseWheel(ev)
	}
	return nil
}

func (m *Model) handleMouseClick(x, y int) tea.Cmd {
	if m.ynPromptActive() {
		if choice := m.hit.yn.at(x, y); choice != ynNone {
			return m.answerYN(choice)
		}
		return nil
	}
	switch m.mode {
	case ModeMenu:
		return m.handleMenuClick(x, y)
	case ModePaneForm, ModeWindowForm, ModeSessionForm, ModeSessionSaveForm, ModePaneCaptureForm:
		return m.handleFormClick(x, y)
	}
	return nil
}

func (m *Model) handleMenuClick(x, y int) tea.Cmd {
	if m.loading {
		return nil
	}
	current := m.currentLevel()
	if current == nil {
		return nil
	}
	if cs := m.hit.dropdownState; cs != nil && m.hit.dropdown.contains(x, y) {
		return m.clickDropdownRow(cs, m.hit.dropdownFirst+y-m.hit.dropdown.y0)
	}
	if st := findStateOf(current); st != nil && m.hit.subtitleRow >= 0 &&
		y == m.hit.barTop+m.hit.subtitleRow && x < findTickBoxWidth() {
		st.history = !st.history
		return nil
	}
	if m.hit.list.contains(x, y) {
		return m.clickListRow(current, m.hit.listFirst+y-m.hit.list.y0, x)
	}
	return nil
}

// clickListRow handles a click on item idx of current at column x: it moves
// the cursor there, toggles a checkbox or tree arrow when the click landed on
// one, and treats a second click on the same row as enter.
func (m *Model) clickListRow(current *level, idx, x int) tea.Cmd {
	if idx < 0 || idx >= len(current.Items) || current.Items[idx].Header {
		return nil
	}
	if current.ID == windowMoveLevelID {
		// The cursor is the window being moved, not a selection.
		return nil
	}
	item := current.Items[idx]
	var cmd tea.Cmd
	if current.Cursor != idx {
		current.Cursor = idx
		events.UI.MenuCursor(current.ID, current.Cursor)
		m.syncViewport(current)
		cmd = m.ensurePreviewForCurrentLevel()
	}

	if labelX, ok := m.hit.treeLabelX[idx]; ok && x < labelX {
		m.lastClick = clickRecord{}
		if ts, ok := current.Data.(*menu.TreeState); ok && ts != nil && menu.TreeIsExpandable(item.ID) {
			if ts.IsExpanded(item.ID) {
				return tea.Batch(cmd, m.treeCollapse(current, ts))
			}
			return tea.Batch(cmd, m.treeExpand(current, ts))
		}
		return cmd
	}
	if current.MultiSelect && current.ID != extractLevelID && x >= 1 && x <= 3 {
		m.lastClick = clickRecord{}
		m.toggleCursorSelection(current)
		return cmd
	}

	now := mouseNow()
	double := m.lastClick.level == current && m.lastClick.item == item.ID &&
		now.Sub(m.lastClick.at) <= doubleClickWindow
	if !double {
		m.lastClick = clickRecord{level: current, item: item.ID, at: now}
		return cmd
	}
	m.lastClick = clickRecord{}
	if current.Node != nil && current.Node.FilterCommand {
		// Enter on the command level runs the typed text, so a double click
		// does what tab does: it puts the command under the cursor in the prompt.
		m.replaceCommandTokenUnderCursor()
		return cmd
	}
	return tea.Batch(cmd, m.handleEnterKey())
}

// clickDropdownRow selects row idx of an open dropdown. An extract selector
// applies its value at once, as moving its cursor does; a second click on a
// completion candidate accepts it.
func (m *Model) clickDropdownRow(cs *completionState, idx int) tea.Cmd {
	if idx < 0 || idx >= len(cs.filtered) {
		return nil
	}
	already := cs.cursor == idx
	cs.cursor = idx
	switch cs {
	case m.extractModePopup:
		return tea.Batch(m.applyExtractModeCursor(), m.armExtractModeTimer())
	case m.extractAreaPopup:
		return tea.Batch(m.applyExtractAreaCursor(), m.armExtractModeTimer())
	case m.completion:
		now := mouseNow()
		double := already && m.lastClick.level == nil && m.lastClick.item == cs.selected() &&
			now.Sub(m.lastClick.at) <= doubleClickWindow
		if double {
			m.lastClick = clickRecord{}
			return m.acceptCompletion()
		}
		m.lastClick = clickRecord{item: cs.selected(), at: now}
	}
	return nil
}

// formCursorSetter is a form whose text cursor can be placed by column.
type formCursorSetter interface {
	SetCursorColumn(col int)
}

func (m *Model) activeFormInput() formCursorSetter {
	switch {
	case m.mode == ModePaneForm && m.paneForm != nil:
		return m.paneForm
	case m.mode == ModeWindowForm && m.windowForm != nil:
		return m.windowForm
	case m.mode == ModeSessionForm && m.sessionForm != nil:
		return m.sessionForm
	case m.mode == ModeSessionSaveForm && m.saveForm != nil:
		return m.saveForm
	case m.mode == ModePaneCaptureForm && m.paneCaptureForm != nil:
		return m.paneCaptureForm
	}
	return nil
}

// handleFormClick places the text cursor of the active form, or toggles the
// pane-capture form's tick box.
func (m *Model) handleFormClick(x, y int) tea.Cmd {
	if m.mode == ModePaneCaptureForm && m.paneCaptureForm != nil && m.hit.formCheckbox.contains(x, y) {
		m.paneCaptureForm.ToggleEscSeqs()
		return nil
	}
	if form := m.activeFormInput(); form != nil && y == m.hit.formInputRow {
		form.SetCursorColumn(x)
	}
	return nil
}

func (m *Model) handleMouseWheel(ev tea.MouseWheelMsg) tea.Cmd {
	delta := 0
	switch ev.Button {
	case tea.MouseWheelUp:
		delta = -1
	case tea.MouseWheelDown:
		delta = 1
	default:
		return nil
	}
	switch m.mode {
	case ModeCommandOutput:
		m.commandOutputOffset = min(max(m.commandOutputOffset+3*delta, 0), m.maxCommandOutputOffset())
		return nil
	case ModeMenu:
	default:
		return nil
	}
	if m.confirmState != nil {
		return nil
	}
	if cs := m.hit.dropdownState; cs != nil && cs == m.completion && m.hit.dropdown.contains(ev.X, ev.Y) {
		cs.cursor = min(max(cs.cursor+delta, 0), len(cs.filtered)-1)
		return nil
	}
	if m.hasSidePreview() && m.hit.preview.contains(ev.X, ev.Y) {
		m.scrollPreview(3 * delta)
		return nil
	}
	current := m.currentLevel()
	if current == nil || current.ID == windowMoveLevelID || !m.hit.list.contains(ev.X, ev.Y) {
		return nil
	}
	// A page of one row is a single step that stops at either end instead of
	// wrapping round, which is what a wheel should do.
	step := func(l *level) bool { return l.MoveCursorPageDown(1) }
	if delta < 0 {
		step = func(l *level) bool { return l.MoveCursorPageUp(1) }
	}
	if m.moveCursor(step, false) {
		return m.ensurePreviewForCurrentLevel()
	}
	return nil
}

// scrollPreview scrolls the side preview panel by delta lines.
func (m *Model) scrollPreview(delta int) {
	preview := m.activePreview()
	if preview == nil || preview.loading {
		return
	}
	innerH := max(m.height-2, 1)
	maxOffset := max(len(preview.lines)-innerH, 0)
	preview.scrollOffset = min(max(preview.scrollOffset+delta, 0), maxOffset)
}

// toggleCursorSelection marks or unmarks the item under the cursor of a
// multi-select level, keeping the plugins "all" entry in step: marking it
// marks everything, unmarking it clears everything, and unmarking any other
// item while it is marked unmarks it too.
func (m *Model) toggleCursorSelection(current *level) {
	current.ToggleCurrentSelection()
	if current.Cursor < 0 || current.Cursor >= len(current.Items) {
		return
	}
	item := current.Items[current.Cursor]
	if item.ID == menu.AllPluginsSentinel {
		if current.IsSelected(item.ID) {
			current.SelectAll()
		} else {
			current.ClearAll()
		}
	} else if !current.IsSelected(item.ID) && current.IsSelected(menu.AllPluginsSentinel) {
		current.ToggleSelection(menu.AllPluginsSentinel)
	}
}

// findTickBoxWidth is how many columns of the find control row the "all
// history" tick box and its label occupy.
func findTickBoxWidth() int {
	return lipgloss.Width("□ all history <" + findHistoryKey + ">")
}
