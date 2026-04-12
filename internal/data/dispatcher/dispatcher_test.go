package dispatcher

import (
	"errors"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/state"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestHandleSessionEventUpdatesStore(t *testing.T) {
	sessions := state.NewSessionStore()
	windows := state.NewWindowStore()
	panes := state.NewPaneStore()
	d := New(sessions, windows, panes)

	res := d.Handle(backend.Event{
		Kind: backend.KindSessions,
		Data: tmux.SessionSnapshot{
			Sessions:       []tmux.Session{{Name: "main", Label: "main", Clients: []string{"c1"}, Windows: 2}},
			Current:        "main",
			IncludeCurrent: false,
		},
	})

	if !res.SessionsUpdated || res.WindowsUpdated || res.PanesUpdated {
		t.Fatalf("unexpected result flags: %+v", res)
	}
	got := sessions.Entries()
	if len(got) != 1 || got[0].Name != "main" {
		t.Fatalf("unexpected session entries: %#v", got)
	}
	if sessions.Current() != "main" {
		t.Fatalf("expected current session main, got %q", sessions.Current())
	}
	if sessions.IncludeCurrent() {
		t.Fatal("expected include current to be false")
	}
}

func TestHandleWindowAndPaneEventsUpdateStores(t *testing.T) {
	sessions := state.NewSessionStore()
	windows := state.NewWindowStore()
	panes := state.NewPaneStore()
	d := New(sessions, windows, panes)

	windowRes := d.Handle(backend.Event{
		Kind: backend.KindWindows,
		Data: tmux.WindowSnapshot{
			Windows:        []tmux.Window{{ID: "main:1", Session: "main", Name: "editor", Label: "editor", Index: 1}},
			CurrentID:      "main:1",
			CurrentLabel:   "editor",
			CurrentSession: "main",
			IncludeCurrent: false,
		},
	})
	paneRes := d.Handle(backend.Event{
		Kind: backend.KindPanes,
		Data: tmux.PaneSnapshot{
			Panes:          []tmux.Pane{{ID: "%1", Label: "shell", PaneID: "%1"}},
			CurrentID:      "%1",
			CurrentLabel:   "shell",
			IncludeCurrent: false,
		},
	})

	if !windowRes.WindowsUpdated || windowRes.SessionsUpdated || windowRes.PanesUpdated {
		t.Fatalf("unexpected window result flags: %+v", windowRes)
	}
	if !paneRes.PanesUpdated || paneRes.SessionsUpdated || paneRes.WindowsUpdated {
		t.Fatalf("unexpected pane result flags: %+v", paneRes)
	}
	if got := windows.Entries(); len(got) != 1 || got[0].ID != "main:1" {
		t.Fatalf("unexpected window entries: %#v", got)
	}
	if got := panes.Entries(); len(got) != 1 || got[0].ID != "%1" {
		t.Fatalf("unexpected pane entries: %#v", got)
	}
	if windows.CurrentID() != "main:1" || windows.CurrentLabel() != "editor" || windows.CurrentSession() != "main" {
		t.Fatalf("unexpected current window state: id=%q label=%q session=%q", windows.CurrentID(), windows.CurrentLabel(), windows.CurrentSession())
	}
	if panes.CurrentID() != "%1" || panes.CurrentLabel() != "shell" {
		t.Fatalf("unexpected current pane state: id=%q label=%q", panes.CurrentID(), panes.CurrentLabel())
	}
}

func TestHandleIgnoresErrorsAndUnexpectedPayloads(t *testing.T) {
	sessions := state.NewSessionStore()
	windows := state.NewWindowStore()
	panes := state.NewPaneStore()
	d := New(sessions, windows, panes)

	errRes := d.Handle(backend.Event{Kind: backend.KindSessions, Err: errors.New("boom")})
	badRes := d.Handle(backend.Event{Kind: backend.KindWindows, Data: "wrong"})

	if errRes != (Result{}) {
		t.Fatalf("error result should be zero, got %+v", errRes)
	}
	if badRes != (Result{}) {
		t.Fatalf("bad payload result should be zero, got %+v", badRes)
	}
	if len(sessions.Entries()) != 0 || len(windows.Entries()) != 0 || len(panes.Entries()) != 0 {
		t.Fatal("stores should remain unchanged")
	}
}
