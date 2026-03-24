package state

import (
	"sync"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

type PaneStore interface {
	Entries() []menu.PaneEntry
	SetEntries([]menu.PaneEntry)
	CurrentID() string
	CurrentLabel() string
	IncludeCurrent() bool
	SetCurrent(id, label string)
	SetIncludeCurrent(bool)
}

type paneStore struct {
	mu             sync.RWMutex
	entries        []menu.PaneEntry
	currentID      string
	currentLabel   string
	includeCurrent bool
}

func NewPaneStore() PaneStore {
	return &paneStore{includeCurrent: true}
}

func (p *paneStore) Entries() []menu.PaneEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return clonePaneEntries(p.entries)
}

func (p *paneStore) SetEntries(entries []menu.PaneEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.entries = clonePaneEntries(entries)
}

func (p *paneStore) CurrentID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentID
}

func (p *paneStore) CurrentLabel() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentLabel
}

func (p *paneStore) IncludeCurrent() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.includeCurrent
}

func (p *paneStore) SetCurrent(id, label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentID = id
	p.currentLabel = label
}

func (p *paneStore) SetIncludeCurrent(include bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.includeCurrent = include
}

func clonePaneEntries(entries []menu.PaneEntry) []menu.PaneEntry {
	if len(entries) == 0 {
		return nil
	}
	dup := make([]menu.PaneEntry, len(entries))
	copy(dup, entries)
	return dup
}
