package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

const (
	windowMoveLevelID = "window:move"
	windowMoveHint    = "↑/↓ move window · enter to confirm · esc to cancel"
)

// initWindowMove builds the move state from the current sessions/windows and
// pins the current window to the cursor. Without a current window (backend
// data not yet arrived on direct invocation) the level is left empty until
// refreshWindowMove sees data.
func (m *Model) initWindowMove(lvl *level) {
	if lvl == nil {
		return
	}
	state, ok := menu.NewWindowMoveState(m.menuContext())
	if !ok {
		lvl.Data = nil
		return
	}
	lvl.Data = state
	m.syncWindowMove(lvl)
}

// refreshWindowMove rebuilds an open window:move level from fresh backend
// data. The pinned window keeps its anchor between its neighbours (see
// WindowMoveState.Refresh); if it disappears the level empties and reports
// why. Refreshing is paused while the move itself is in flight.
func (m *Model) refreshWindowMove() {
	lvl := m.findLevelByID(windowMoveLevelID)
	if lvl == nil || (m.loading && m.pendingID == windowMoveLevelID) {
		return
	}
	state, ok := lvl.Data.(*menu.WindowMoveState)
	if !ok {
		m.initWindowMove(lvl)
		return
	}
	if !state.Refresh(m.menuContext()) {
		lvl.Data = nil
		lvl.Full = nil
		lvl.Items = nil
		lvl.Cursor = 0
		m.errMsg = fmt.Sprintf("window %s no longer exists", state.Source().Name)
		return
	}
	m.syncWindowMove(lvl)
}

func (m *Model) syncWindowMove(lvl *level) {
	state, ok := lvl.Data.(*menu.WindowMoveState)
	if !ok || state == nil {
		return
	}
	items := state.Items()
	lvl.Full = items
	lvl.Items = items
	lvl.Cursor = state.Cursor()
	m.syncViewport(lvl)
}

// handleWindowMoveKey routes keys on the window:move level: movement keys
// shift the pinned window, enter commits, and everything else except
// esc/ctrl+c and the tree style key is swallowed (the level has no filter).
func (m *Model) handleWindowMoveKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := msg.String()
	switch key {
	case "esc", "ctrl+c", treeStyleCycleKey:
		return nil, false
	}
	current := m.currentLevel()
	state, _ := current.Data.(*menu.WindowMoveState)
	if state == nil || m.loading {
		return nil, true
	}
	page := max(m.maxVisibleItems()-1, 1)
	moved := false
	switch key {
	case "up":
		moved = state.Step(-1)
	case "down":
		moved = state.Step(1)
	case "pgup":
		moved = state.Step(-page)
	case "pgdown":
		moved = state.Step(page)
	case "home":
		moved = state.StepHome()
	case "end":
		moved = state.StepEnd()
	case "enter":
		return m.commitWindowMove(state), true
	}
	if moved {
		m.syncWindowMove(current)
	}
	return nil, true
}

// commitWindowMove performs the move, or leaves exactly like escape when the
// window was put back where it started.
func (m *Model) commitWindowMove(state *menu.WindowMoveState) tea.Cmd {
	if !state.Moved() {
		return m.handleEscapeKey()
	}
	plan := state.Plan()
	m.loading = true
	m.pendingID = windowMoveLevelID
	m.pendingLabel = fmt.Sprintf("%s → %s:%d", state.Source().Name, plan.TargetSession, plan.Index)
	m.errMsg = ""
	m.forceClearInfo()
	return menu.WindowMoveCommand(m.socketPath, plan)
}
