package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

// fakeMouseClock replaces the double-click clock with one the test advances.
func fakeMouseClock(t *testing.T) func(time.Duration) {
	t.Helper()
	now := time.Unix(1_700_000_000, 0)
	prev := mouseNow
	mouseNow = func() time.Time { return now }
	t.Cleanup(func() { mouseNow = prev })
	return func(d time.Duration) { now = now.Add(d) }
}

// click renders a frame (the mouse can only hit what is on screen) and sends
// a left click at x, y without running the returned command.
func click(h *Harness, x, y int) tea.Cmd {
	h.View()
	return h.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func wheel(h *Harness, x, y int, button tea.MouseButton) {
	h.View()
	h.Update(tea.MouseWheelMsg{X: x, Y: y, Button: button})
}

// rowOf returns the screen row whose text contains needle.
func rowOf(t *testing.T, h *Harness, needle string) int {
	t.Helper()
	for y, line := range strings.Split(ansi.Strip(h.View()), "\n") {
		if strings.Contains(line, needle) {
			return y
		}
	}
	t.Fatalf("no row contains %q:\n%s", needle, ansi.Strip(h.View()))
	return -1
}

func TestMouseClickMovesCursorToRow(t *testing.T) {
	fakeMouseClock(t)
	h := NewHarness(NewModel(ModelConfig{Width: 100, Height: 30}))
	lvl := h.Model().currentLevel()
	if lvl.Items[lvl.Cursor].ID == "window" {
		t.Fatal("test needs the cursor to start off the window row")
	}
	y := rowOf(t, h, "window")
	if cmd := click(h, 10, y); cmd != nil {
		t.Fatalf("a single click must only move the cursor, got a command")
	}
	if got := lvl.Items[lvl.Cursor].ID; got != "window" {
		t.Fatalf("cursor is on %q after clicking the window row", got)
	}
	if h.Model().loading {
		t.Fatal("a single click must not activate the row")
	}
}

func TestMouseDoubleClickActsAsEnter(t *testing.T) {
	advance := fakeMouseClock(t)
	h := NewHarness(NewModel(ModelConfig{Width: 100, Height: 30}))
	y := rowOf(t, h, "window")
	click(h, 10, y)
	advance(doubleClickWindow / 2)
	if cmd := click(h, 10, y); cmd == nil {
		t.Fatal("a double click should start loading the submenu")
	}
	if m := h.Model(); !m.loading || m.pendingID != "window" {
		t.Fatalf("double click did not enter the window menu (loading=%t pending=%q)", m.loading, m.pendingID)
	}
}

func TestMouseSlowSecondClickIsNotADoubleClick(t *testing.T) {
	advance := fakeMouseClock(t)
	h := NewHarness(NewModel(ModelConfig{Width: 100, Height: 30}))
	y := rowOf(t, h, "window")
	click(h, 10, y)
	advance(doubleClickWindow + time.Millisecond)
	click(h, 10, y)
	if h.Model().loading {
		t.Fatal("two clicks further apart than the double-click window entered the menu")
	}
}

func TestMouseClicksOnDifferentRowsAreNotADoubleClick(t *testing.T) {
	fakeMouseClock(t)
	h := NewHarness(NewModel(ModelConfig{Width: 100, Height: 30}))
	click(h, 10, rowOf(t, h, "window"))
	click(h, 10, rowOf(t, h, "session"))
	if h.Model().loading {
		t.Fatal("clicks on two different rows entered a menu")
	}
	lvl := h.Model().currentLevel()
	if got := lvl.Items[lvl.Cursor].ID; got != "session" {
		t.Fatalf("cursor is on %q, want the last clicked row", got)
	}
}

func TestMouseClickOutsideTheListIsIgnored(t *testing.T) {
	fakeMouseClock(t)
	h := NewHarness(NewModel(ModelConfig{Width: 100, Height: 30}))
	lvl := h.Model().currentLevel()
	lvl.Cursor = 0
	click(h, 10, len(lvl.Items)+3) // blank rows below the items
	click(h, 10, 29)               // the input row
	if lvl.Cursor != 0 {
		t.Fatalf("a click outside the list moved the cursor to %d", lvl.Cursor)
	}
}

func TestMouseWheelMovesListCursorWithoutWrapping(t *testing.T) {
	h := NewHarness(NewModel(ModelConfig{Width: 100, Height: 30}))
	lvl := h.Model().currentLevel()
	lvl.Cursor = 0
	wheel(h, 10, 2, tea.MouseWheelUp)
	if lvl.Cursor != 0 {
		t.Fatalf("wheel up at the top wrapped the cursor to %d", lvl.Cursor)
	}
	wheel(h, 10, 2, tea.MouseWheelDown)
	wheel(h, 10, 2, tea.MouseWheelDown)
	if lvl.Cursor != 2 {
		t.Fatalf("two wheel-down steps put the cursor on %d, want 2", lvl.Cursor)
	}
	lvl.Cursor = len(lvl.Items) - 1
	wheel(h, 10, 2, tea.MouseWheelDown)
	if lvl.Cursor != len(lvl.Items)-1 {
		t.Fatalf("wheel down at the bottom wrapped the cursor to %d", lvl.Cursor)
	}
}

// multiSelectHarness opens pane:kill, a multi-select level, over three panes.
func multiSelectHarness(t *testing.T) (*Harness, *level) {
	t.Helper()
	m := NewModel(ModelConfig{Width: 100, Height: 30, NoPreview: true})
	node, ok := m.registry.Find("pane:kill")
	if !ok {
		t.Fatal("registry missing pane:kill")
	}
	lvl := newLevel("pane:kill", "kill", []menu.Item{
		{ID: "main:0.0", Label: "main:0.0 one"},
		{ID: "main:0.1", Label: "main:0.1 two"},
		{ID: "main:1.0", Label: "main:1.0 three"},
	}, node)
	m.applyNodeSettings(lvl)
	if !lvl.MultiSelect {
		t.Fatal("pane:kill should be a multi-select level")
	}
	m.stack = append(m.stack, lvl)
	return NewHarness(m), lvl
}

func TestMouseClickOnCheckboxTogglesSelection(t *testing.T) {
	fakeMouseClock(t)
	h, lvl := multiSelectHarness(t)
	y := rowOf(t, h, "main:0.1 two")
	box := strings.Index(strings.Split(ansi.Strip(h.View()), "\n")[y], "□")
	x := ansi.StringWidth(strings.Split(ansi.Strip(h.View()), "\n")[y][:box])

	click(h, x, y)
	if !lvl.IsSelected("main:0.1") || lvl.Cursor != 1 {
		t.Fatalf("clicking the checkbox should mark the row and move the cursor (selected=%t cursor=%d)", lvl.IsSelected("main:0.1"), lvl.Cursor)
	}
	// A second click straight away unmarks it: it is a toggle, not a double click.
	if cmd := click(h, x, y); cmd != nil {
		t.Fatal("clicking a checkbox twice must not act as enter")
	}
	if lvl.IsSelected("main:0.1") {
		t.Fatal("a second click on the checkbox should unmark the row")
	}
	// A click on the label only moves the cursor.
	click(h, x+8, rowOf(t, h, "main:1.0 three"))
	if lvl.IsSelected("main:1.0") || lvl.Cursor != 2 {
		t.Fatalf("clicking the label should only move the cursor (selected=%t cursor=%d)", lvl.IsSelected("main:1.0"), lvl.Cursor)
	}
}

func TestMouseClickOnFindTickBoxTogglesHistory(t *testing.T) {
	h := newFindHarness(t, menu.PaneFindID, nil, nil)
	st := findStateOf(h.Model().currentLevel())
	y := rowOf(t, h, "all history")
	click(h, 0, y)
	if !st.history {
		t.Fatal("clicking the tick box should enable history search")
	}
	if view := ansi.Strip(h.View()); !strings.Contains(view, "■ all history") {
		t.Fatalf("the tick box should render ticked:\n%s", view)
	}
	click(h, findTickBoxWidth()-1, y)
	if st.history {
		t.Fatal("clicking the tick box label again should disable history search")
	}
	click(h, findTickBoxWidth()+5, y)
	if st.history {
		t.Fatal("a click beside the tick box must not toggle it")
	}
}

func TestMouseClickOnTreeArrowTogglesNode(t *testing.T) {
	fakeMouseClock(t)
	sessions := []menu.SessionEntry{{Name: "alpha", Windows: 2}, {Name: "beta", Windows: 1}}
	windows := []menu.WindowEntry{
		{ID: "0", Label: "0:bash", Session: "alpha"},
		{ID: "1", Label: "1:vim", Session: "alpha"},
		{ID: "0", Label: "0:zsh", Session: "beta"},
	}
	h := NewHarness(testTreeModel(sessions, windows, nil, false))
	// "beta (" is the tree row; the preview title also names the session.
	y := rowOf(t, h, "beta (")
	line := strings.Split(ansi.Strip(h.View()), "\n")[y]
	arrowX := ansi.StringWidth(line[:strings.Index(line, "▶")])

	click(h, arrowX, y)
	if view := ansi.Strip(h.View()); !strings.Contains(view, "0:zsh") {
		t.Fatalf("clicking a collapsed node's arrow should expand it:\n%s", view)
	}
	click(h, arrowX, rowOf(t, h, "beta ("))
	if view := ansi.Strip(h.View()); strings.Contains(view, "0:zsh") {
		t.Fatalf("clicking an expanded node's arrow should collapse it:\n%s", view)
	}
	// A click on the name selects the row without toggling it.
	y = rowOf(t, h, "alpha (")
	line = strings.Split(ansi.Strip(h.View()), "\n")[y]
	click(h, ansi.StringWidth(line[:strings.Index(line, "alpha")])+1, y)
	if view := ansi.Strip(h.View()); strings.Contains(view, "0:bash") {
		t.Fatalf("clicking a node's name must not expand it:\n%s", view)
	}
}

func TestMouseClickOnCompletionRowSelectsThenAccepts(t *testing.T) {
	advance := fakeMouseClock(t)
	h := setupCommandHarness(t)
	sendKeys(h, "kill-session")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "-t")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	if !h.Model().completionVisible() {
		t.Fatal("expected the session dropdown to be open")
	}
	cs := h.Model().completion
	want := cs.filtered[len(cs.filtered)-1].Value
	if cs.selected() == want {
		t.Fatalf("test needs a candidate other than the selected one, got only %q", want)
	}
	y := rowOf(t, h, want)
	line := strings.Split(ansi.Strip(h.View()), "\n")[y]
	x := ansi.StringWidth(line[:strings.Index(line, want)])

	click(h, x, y)
	if got := cs.selected(); got != want {
		t.Fatalf("clicking a candidate selected %q, want %q", got, want)
	}
	if strings.Contains(h.Model().currentLevel().Filter, want) {
		t.Fatal("a single click must not accept the candidate")
	}
	advance(doubleClickWindow / 2)
	h.processCmd(click(h, x, y))
	if got := h.Model().currentLevel().Filter; !strings.HasSuffix(strings.TrimSpace(got), want) {
		t.Fatalf("double clicking a candidate should accept it, prompt is %q", got)
	}
}

func TestMouseOnPaneCaptureForm(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 24})
	h := NewHarness(m)
	h.Send(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1", SocketPath: "sock"},
		Template: "test.log",
	})
	form := h.Model().paneCaptureForm
	if form == nil || form.EscSeqs() {
		t.Fatal("expected the pane capture form with its tick box off")
	}
	click(h, 0, rowOf(t, h, "capture escape sequences"))
	if !form.EscSeqs() {
		t.Fatal("clicking the tick box should turn escape sequences on")
	}

	// Clicking in the text moves the cursor: typing then inserts there.
	y := rowOf(t, h, "test.log")
	click(h, 4, y) // "» te|st.log": two prompt columns, then two characters
	h.Send(tea.KeyPressMsg{Code: 'X', Text: "X"})
	if got := form.Value(); got != "teXst.log" {
		t.Fatalf("typing after a click in the input gave %q, want %q", got, "teXst.log")
	}
}

func TestYNPromptButtonsHoverAndClick(t *testing.T) {
	m, _ := makeDeleteSavedLevel(t)
	h := NewHarness(m)
	m.handleEnterKey()
	if m.confirmState == nil {
		t.Fatal("expected a delete confirmation")
	}
	view := m.View()
	if view.MouseMode != tea.MouseModeAllMotion {
		t.Fatal("a y/n prompt must ask for mouse motion so its buttons can hover")
	}
	buttons, ok := locateYNButtons(view.Content)
	if !ok {
		t.Fatalf("no y/n buttons found in:\n%s", ansi.Strip(view.Content))
	}
	if h := buttons.yes.y1 - buttons.yes.y0; h != ynPromptRows {
		t.Fatalf("yes button is %d rows tall, want %d", h, ynPromptRows)
	}
	if buttons.yes.y1 != 30 {
		t.Fatalf("the prompt should be pinned to the bottom of a 30-row popup, buttons end at row %d", buttons.yes.y1)
	}

	// Hovering highlights only the button under the pointer.
	plain := m.View().Content
	h.Update(tea.MouseMotionMsg{X: buttons.no.x0 + 1, Y: buttons.no.y0})
	if m.ynHover != ynNo {
		t.Fatalf("hover = %v, want the no button", m.ynHover)
	}
	if m.View().Content == plain {
		t.Fatal("hovering a button should change how it is drawn")
	}
	h.Update(tea.MouseMotionMsg{X: 0, Y: 0})
	if m.ynHover != ynNone || m.View().Content != plain {
		t.Fatal("moving off the buttons should clear the highlight")
	}

	// Clicks anywhere else are ignored, including on the list behind.
	lvl := m.currentLevel()
	cursor := lvl.Cursor
	click(h, 5, 2)
	click(h, buttons.yes.x0-3, buttons.yes.y0+1) // on the question text
	if m.confirmState == nil || lvl.Cursor != cursor {
		t.Fatal("a click outside the buttons must leave the prompt and the list alone")
	}

	click(h, buttons.no.x0, buttons.no.y1-1)
	if m.confirmState != nil {
		t.Fatal("clicking no should cancel the confirmation")
	}
	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("mouse mode after the prompt closed = %v, want cell motion", got)
	}

	m.handleEnterKey()
	buttons, _ = locateYNButtons(m.View().Content)
	if cmd := click(h, buttons.yes.x1-1, buttons.yes.y0); cmd == nil || !m.loading {
		t.Fatal("clicking yes should dispatch the delete")
	}
}

func TestYNPromptKeysStillAnswer(t *testing.T) {
	m, _ := makeDeleteSavedLevel(t)
	h := NewHarness(m)
	m.handleEnterKey()
	h.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if m.confirmState != nil {
		t.Fatal("the n key should still cancel the confirmation")
	}
}

func TestPluginReloadPromptButtonsClick(t *testing.T) {
	m := NewModel(ModelConfig{Width: 90, Height: 20})
	m.mode = ModePluginInstall
	m.pluginInstallState = &pluginInstallState{
		operation:       "install",
		finished:        true,
		summary:         "Installed 1 plugin(s)",
		installed:       []plugin.Plugin{{Name: "alpha"}},
		progressCurrent: 4,
		progressTotal:   4,
	}
	h := NewHarness(m)
	buttons, ok := locateYNButtons(m.View().Content)
	if !ok {
		t.Fatalf("no reload buttons found in:\n%s", ansi.Strip(m.View().Content))
	}
	click(h, buttons.no.x0+1, buttons.no.y0+1)
	if m.mode != ModeMenu || m.pluginInstallState != nil {
		t.Fatalf("clicking no should close the plugin screen (mode=%v)", m.mode)
	}
}
