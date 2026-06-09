package state

import (
	"slices"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

type WindowStore struct {
	entryStore[menu.WindowEntry]
	currentID      string
	currentLabel   string
	currentSession string
	includeCurrent bool
}

func NewWindowStore() *WindowStore {
	return &WindowStore{
		entryStore:     entryStore[menu.WindowEntry]{clone: cloneWindowEntries},
		includeCurrent: true,
	}
}

func (w *WindowStore) CurrentID() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentID
}

func (w *WindowStore) CurrentLabel() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentLabel
}

func (w *WindowStore) CurrentSession() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.currentSession
}

func (w *WindowStore) SetCurrent(id, label, session string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.currentID = id
	w.currentLabel = label
	w.currentSession = session
}

func (w *WindowStore) IncludeCurrent() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.includeCurrent
}

func (w *WindowStore) SetIncludeCurrent(include bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.includeCurrent = include
}

func cloneWindowEntries(entries []menu.WindowEntry) []menu.WindowEntry {
	if len(entries) == 0 {
		return nil
	}
	return slices.Clone(entries)
}
