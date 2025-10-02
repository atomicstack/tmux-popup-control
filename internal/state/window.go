package state

import "github.com/atomicstack/tmux-popup-control/internal/menu"

type WindowStore interface {
	Entries() []menu.WindowEntry
	SetEntries([]menu.WindowEntry)
	CurrentID() string
	CurrentLabel() string
	CurrentSession() string
	SetCurrent(id, label, session string)
	IncludeCurrent() bool
	SetIncludeCurrent(bool)
}

type windowStore struct {
	entries        []menu.WindowEntry
	currentID      string
	currentLabel   string
	currentSession string
	includeCurrent bool
}

func NewWindowStore() WindowStore {
	return &windowStore{includeCurrent: true}
}

func (w *windowStore) Entries() []menu.WindowEntry {
	return cloneWindowEntries(w.entries)
}

func (w *windowStore) SetEntries(entries []menu.WindowEntry) {
	w.entries = cloneWindowEntries(entries)
}

func (w *windowStore) CurrentID() string {
	return w.currentID
}

func (w *windowStore) CurrentLabel() string {
	return w.currentLabel
}

func (w *windowStore) CurrentSession() string {
	return w.currentSession
}

func (w *windowStore) SetCurrent(id, label, session string) {
	w.currentID = id
	w.currentLabel = label
	w.currentSession = session
}

func (w *windowStore) IncludeCurrent() bool {
	return w.includeCurrent
}

func (w *windowStore) SetIncludeCurrent(include bool) {
	w.includeCurrent = include
}

func cloneWindowEntries(entries []menu.WindowEntry) []menu.WindowEntry {
	if len(entries) == 0 {
		return nil
	}
	dup := make([]menu.WindowEntry, len(entries))
	copy(dup, entries)
	return dup
}
