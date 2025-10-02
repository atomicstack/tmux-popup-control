package dispatcher

import (
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/state"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

type Result struct {
	SessionsUpdated bool
	WindowsUpdated  bool
	PanesUpdated    bool
}

type Dispatcher struct {
	sessions state.SessionStore
	windows  state.WindowStore
	panes    state.PaneStore
}

func New(s state.SessionStore, w state.WindowStore, p state.PaneStore) *Dispatcher {
	return &Dispatcher{sessions: s, windows: w, panes: p}
}

func (d *Dispatcher) Handle(evt backend.Event) Result {
	var res Result
	if evt.Err != nil {
		return res
	}
	switch evt.Kind {
	case backend.KindSessions:
		if snapshot, ok := evt.Data.(tmux.SessionSnapshot); ok {
			entries := menu.SessionEntriesFromTmux(snapshot.Sessions)
			d.sessions.SetEntries(entries)
			d.sessions.SetCurrent(snapshot.Current)
			d.sessions.SetIncludeCurrent(snapshot.IncludeCurrent)
			res.SessionsUpdated = true
		}
	case backend.KindWindows:
		if snapshot, ok := evt.Data.(tmux.WindowSnapshot); ok {
			entries := menu.WindowEntriesFromTmux(snapshot.Windows)
			d.windows.SetEntries(entries)
			d.windows.SetCurrent(snapshot.CurrentID, snapshot.CurrentLabel, snapshot.CurrentSession)
			d.windows.SetIncludeCurrent(snapshot.IncludeCurrent)
			res.WindowsUpdated = true
		}
	case backend.KindPanes:
		if snapshot, ok := evt.Data.(tmux.PaneSnapshot); ok {
			entries := menu.PaneEntriesFromTmux(snapshot.Panes)
			d.panes.SetEntries(entries)
			d.panes.SetCurrent(snapshot.CurrentID, snapshot.CurrentLabel)
			d.panes.SetIncludeCurrent(snapshot.IncludeCurrent)
			res.PanesUpdated = true
		}
	}
	return res
}
