package ui

import (
	"sort"

	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/state"
)

// modelDataSource adapts Model state stores to cmdparse.DataSource.
type modelDataSource struct {
	sessions state.SessionStore
	windows  state.WindowStore
	panes    state.PaneStore
	schemas  map[string]*cmdparse.CommandSchema
}

func (ds *modelDataSource) Sessions() []string {
	entries := ds.sessions.Entries()
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func (ds *modelDataSource) Windows() []string {
	entries := ds.windows.Entries()
	labels := make([]string, 0, len(entries))
	for _, entry := range entries {
		labels = append(labels, entry.Session+":"+entry.Name)
	}
	return labels
}

func (ds *modelDataSource) Panes() []string {
	entries := ds.panes.Entries()
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.PaneID)
	}
	return ids
}

func (ds *modelDataSource) Clients() []string {
	return nil
}

func (ds *modelDataSource) Commands() []string {
	names := make([]string, 0, len(ds.schemas))
	seen := make(map[string]bool, len(ds.schemas))
	for _, schema := range ds.schemas {
		if schema == nil || schema.Name == "" || seen[schema.Name] {
			continue
		}
		seen[schema.Name] = true
		names = append(names, schema.Name)
	}
	sort.Strings(names)
	return names
}

func (ds *modelDataSource) Buffers() []string {
	return nil
}
