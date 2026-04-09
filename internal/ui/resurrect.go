package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

type resurrectState struct {
	operation   string // "save" or "restore"
	progress    <-chan resurrect.ProgressEvent
	log         []logEntry
	step        int
	total       int
	displayStep float64 // smoothed value for rendering the bar
	animating   bool    // true while a lerp tick is running
	done        bool
	err         error
}

type logEntry struct {
	message string
	kind    string // "session", "window", "pane", "info", "error"
	id      string
}

type resurrectProgressMsg struct {
	event resurrect.ProgressEvent
}

type resurrectTickMsg struct{}

type resurrectAnimTickMsg struct{}

// resurrectAnimInterval is the tick rate for progress bar lerp animation.
const resurrectAnimInterval = 16 * time.Millisecond // ~60 fps

// resurrectLerpSpeed controls how quickly displayStep catches up to step
// per tick. higher = faster catchup. at 0.1 and 60fps, a 10-unit gap
// closes to <0.5 in ~250ms.
const resurrectLerpSpeed = 0.1

// SetResurrectInit configures the model to emit a ResurrectStart on Init,
// entering the progress UI immediately. Used by the CLI subcommands.
func (m *Model) SetResurrectInit(start menu.ResurrectStart) {
	m.initCmd = func() tea.Msg { return start }
}

func (m *Model) handleResurrectStartMsg(msg tea.Msg) tea.Cmd {
	start := msg.(menu.ResurrectStart)
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
		s.displayStep = float64(s.step)
		if evt.Err != nil {
			s.err = evt.Err
			return nil
		}
		// success: auto-dismiss after 1s
		return tea.Tick(time.Second, func(time.Time) tea.Msg {
			return resurrectTickMsg{}
		})
	}
	cmds := []tea.Cmd{readResurrectProgress(s.progress)}
	if !s.animating {
		s.animating = true
		cmds = append(cmds, tea.Tick(resurrectAnimInterval, func(time.Time) tea.Msg {
			return resurrectAnimTickMsg{}
		}))
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleResurrectAnimTickMsg(msg tea.Msg) tea.Cmd {
	s := m.resurrectState
	if s == nil {
		return nil
	}
	target := float64(s.step)
	gap := target - s.displayStep
	if gap < 0.5 || s.done {
		s.displayStep = target
		s.animating = false
		return nil
	}
	s.displayStep += gap * resurrectLerpSpeed
	return tea.Tick(resurrectAnimInterval, func(time.Time) tea.Msg {
		return resurrectAnimTickMsg{}
	})
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
