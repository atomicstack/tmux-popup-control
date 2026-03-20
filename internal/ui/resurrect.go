package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

type resurrectState struct {
	operation string // "save" or "restore"
	progress  <-chan resurrect.ProgressEvent
	log       []logEntry
	step      int
	total     int
	done      bool
	err       error
}

type logEntry struct {
	message string
	kind    string // "session", "window", "pane", "info", "error"
	id      string
}

// ResurrectStart triggers mode transition from a menu action.
type ResurrectStart struct {
	Operation string // "save" or "restore"
	Name      string // snapshot name (save-as only)
	SaveFile  string // path to restore from
	Config    resurrect.Config
}

type resurrectProgressMsg struct {
	event resurrect.ProgressEvent
}

type resurrectTickMsg struct{}

func (m *Model) handleResurrectStartMsg(msg tea.Msg) tea.Cmd {
	start := msg.(ResurrectStart)
	var ch <-chan resurrect.ProgressEvent
	if start.Operation == "restore" {
		ch = resurrect.Restore(start.Config, start.SaveFile)
	} else {
		ch = resurrect.Save(start.Config)
	}
	m.resurrectState = &resurrectState{
		operation: start.Operation,
		progress:  ch,
	}
	m.mode = ModeResurrect
	m.loading = false
	return readResurrectProgress(ch)
}

func readResurrectProgress(ch <-chan resurrect.ProgressEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return nil
		}
		return resurrectProgressMsg{event: event}
	}
}

func (m *Model) handleResurrectProgressMsg(msg tea.Msg) tea.Cmd {
	progMsg, ok := msg.(resurrectProgressMsg)
	if !ok {
		return nil
	}
	s := m.resurrectState
	if s == nil {
		return nil
	}
	evt := progMsg.event
	s.step = evt.Step
	s.total = evt.Total
	if evt.Message != "" {
		s.log = append(s.log, logEntry{
			message: evt.Message,
			kind:    evt.Kind,
			id:      evt.ID,
		})
	}
	if evt.Done {
		s.done = true
		if evt.Err != nil {
			s.err = evt.Err
			return nil
		}
		// success: auto-dismiss after 1s
		return tea.Tick(time.Second, func(time.Time) tea.Msg {
			return resurrectTickMsg{}
		})
	}
	return readResurrectProgress(s.progress)
}

func (m *Model) handleResurrectTickMsg(msg tea.Msg) tea.Cmd {
	return tea.Quit
}

func (m *Model) handleResurrectKey(msg tea.Msg) (bool, tea.Cmd) {
	s := m.resurrectState
	if s == nil {
		return false, nil
	}
	_, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}
	if !s.done {
		// consume all keys while running
		return true, nil
	}
	if s.err != nil {
		// on error, any key dismisses back to menu
		m.resurrectState = nil
		m.mode = ModeMenu
		return true, nil
	}
	// on success, any key quits
	return true, tea.Quit
}
