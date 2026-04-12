package state

import (
	"sync"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

type SessionStore interface {
	Entries() []menu.SessionEntry
	SetEntries([]menu.SessionEntry)
	Current() string
	SetCurrent(string)
	IncludeCurrent() bool
	SetIncludeCurrent(bool)
}

type sessionStore struct {
	mu             sync.RWMutex
	entries        []menu.SessionEntry
	current        string
	includeCurrent bool
}

func NewSessionStore() SessionStore {
	return &sessionStore{includeCurrent: true}
}

func (s *sessionStore) Entries() []menu.SessionEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSessionEntries(s.entries)
}

func (s *sessionStore) SetEntries(entries []menu.SessionEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = cloneSessionEntries(entries)
}

func (s *sessionStore) Current() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *sessionStore) SetCurrent(current string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = current
}

func (s *sessionStore) IncludeCurrent() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.includeCurrent
}

func (s *sessionStore) SetIncludeCurrent(include bool) {
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
			dup[i].Clients = append([]string(nil), entry.Clients...)
		}
	}
	return dup
}
