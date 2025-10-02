package state

import "github.com/atomicstack/tmux-popup-control/internal/menu"

type SessionStore interface {
	Entries() []menu.SessionEntry
	SetEntries([]menu.SessionEntry)
	Current() string
	SetCurrent(string)
	IncludeCurrent() bool
	SetIncludeCurrent(bool)
}

type sessionStore struct {
	entries        []menu.SessionEntry
	current        string
	includeCurrent bool
}

func NewSessionStore() SessionStore {
	return &sessionStore{includeCurrent: true}
}

func (s *sessionStore) Entries() []menu.SessionEntry {
	return cloneSessionEntries(s.entries)
}

func (s *sessionStore) SetEntries(entries []menu.SessionEntry) {
	s.entries = cloneSessionEntries(entries)
}

func (s *sessionStore) Current() string {
	return s.current
}

func (s *sessionStore) SetCurrent(current string) {
	s.current = current
}

func (s *sessionStore) IncludeCurrent() bool {
	return s.includeCurrent
}

func (s *sessionStore) SetIncludeCurrent(include bool) {
	s.includeCurrent = include
}

func cloneSessionEntries(entries []menu.SessionEntry) []menu.SessionEntry {
	if len(entries) == 0 {
		return nil
	}
	dup := make([]menu.SessionEntry, len(entries))
	copy(dup, entries)
	return dup
}
