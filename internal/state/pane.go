package state

import "github.com/atomicstack/tmux-popup-control/internal/menu"

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
	entries        []menu.PaneEntry
	currentID      string
	currentLabel   string
	includeCurrent bool
}

func NewPaneStore() PaneStore {
	return &paneStore{includeCurrent: true}
}

func (p *paneStore) Entries() []menu.PaneEntry {
	return clonePaneEntries(p.entries)
}

func (p *paneStore) SetEntries(entries []menu.PaneEntry) {
	p.entries = clonePaneEntries(entries)
}

func (p *paneStore) CurrentID() string {
	return p.currentID
}

func (p *paneStore) CurrentLabel() string {
	return p.currentLabel
}

func (p *paneStore) IncludeCurrent() bool {
	return p.includeCurrent
}

func (p *paneStore) SetCurrent(id, label string) {
	p.currentID = id
	p.currentLabel = label
}

func (p *paneStore) SetIncludeCurrent(include bool) {
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
