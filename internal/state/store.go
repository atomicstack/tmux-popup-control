package state

import "sync"

// entryStore is the shared RWMutex-guarded slice container used by the
// session, window and pane stores. The clone func lets each store deep-copy
// its element type on the way in and out so callers can never mutate stored
// state through a returned slice.
type entryStore[T any] struct {
	mu      sync.RWMutex
	entries []T
	clone   func([]T) []T
}

// Entries returns a clone of the stored slice under the read lock.
func (s *entryStore[T]) Entries() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clone(s.entries)
}

// SetEntries stores a clone of the supplied slice under the write lock.
func (s *entryStore[T]) SetEntries(entries []T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.clone(entries)
}
