package state

import (
	"slices"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

type PaneStore struct {
	entryStore[menu.PaneEntry]
	currentID      string
	currentLabel   string
	includeCurrent bool
}

func NewPaneStore() *PaneStore {
	return &PaneStore{
		entryStore:     entryStore[menu.PaneEntry]{clone: clonePaneEntries},
		includeCurrent: true,
	}
}

func (p *PaneStore) CurrentID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentID
}

func (p *PaneStore) CurrentLabel() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentLabel
}

func (p *PaneStore) IncludeCurrent() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.includeCurrent
}

func (p *PaneStore) SetCurrent(id, label string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentID = id
	p.currentLabel = label
}

func (p *PaneStore) SetIncludeCurrent(include bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.includeCurrent = include
}

func clonePaneEntries(entries []menu.PaneEntry) []menu.PaneEntry {
	if len(entries) == 0 {
		return nil
	}
	return slices.Clone(entries)
}
