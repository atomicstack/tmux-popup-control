package state

import (
	"slices"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

type SessionStore struct {
	entryStore[menu.SessionEntry]
	current        string
	includeCurrent bool
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		entryStore:     entryStore[menu.SessionEntry]{clone: cloneSessionEntries},
		includeCurrent: true,
	}
}

func (s *SessionStore) Current() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *SessionStore) SetCurrent(current string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = current
}

func (s *SessionStore) IncludeCurrent() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.includeCurrent
}

func (s *SessionStore) SetIncludeCurrent(include bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.includeCurrent = include
}

func cloneSessionEntries(entries []menu.SessionEntry) []menu.SessionEntry {
	if len(entries) == 0 {
		return nil
	}
	dup := make([]menu.SessionEntry, len(entries))
	for i, entry := range entries {
		dup[i] = entry
		if len(entry.Clients) > 0 {
			dup[i].Clients = slices.Clone(entry.Clients)
		}
	}
	return dup
}
