package ui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// TestBackendEventRefreshesTableDrivenLiveLevels pins that every level the
// menu package declares live is rebuilt from the matching backend event, using
// the menu-owned builder, and keeps its node's multi-select setting.
func TestBackendEventRefreshesTableDrivenLiveLevels(t *testing.T) {
	events := map[menu.LiveSource]backend.Event{
		menu.LiveSessions: {Kind: backend.KindSessions, Data: tmux.SessionSnapshot{
			Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}, {Name: "work", Label: "work: 2 windows"}},
			Current:  "main",
		}},
		menu.LiveWindows: {Kind: backend.KindWindows, Data: tmux.WindowSnapshot{
			Windows: []tmux.Window{
				{ID: "main:0", Session: "main", Name: "vim", Current: true},
				{ID: "work:1", Session: "work", Name: "logs"},
			},
			CurrentID:      "main:0",
			CurrentSession: "main",
		}},
		menu.LivePanes: {Kind: backend.KindPanes, Data: tmux.PaneSnapshot{
			Panes: []tmux.Pane{
				{ID: "main:0.0", PaneID: "%1", Session: "main", Current: true},
				{ID: "work:1.0", PaneID: "%2", Session: "work"},
			},
			CurrentID:    "main:0.0",
			CurrentLabel: "main:0.0: vim",
		}},
	}
	for _, src := range []menu.LiveSource{menu.LiveSessions, menu.LiveWindows, menu.LivePanes} {
		for _, live := range menu.LiveLevels(src) {
			t.Run(live.ID, func(t *testing.T) {
				h := NewHarness(NewModel(ModelConfig{Width: 80, Height: 24}))
				h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
				m := h.Model()
				lvl := newLevel(live.ID, live.ID, nil, nil)
				m.stack = append(m.stack, lvl)

				// Feed all sources first so the context is complete, then
				// re-send the level's own source to trigger its refresh.
				for _, s := range []menu.LiveSource{menu.LiveSessions, menu.LiveWindows, menu.LivePanes} {
					h.Send(backendEventMsg{event: events[s]})
				}
				h.Send(backendEventMsg{event: events[src]})

				want := live.Items(m.menuContext())
				if len(want) == 0 {
					t.Fatalf("test data produced no items for %s", live.ID)
				}
				gotIDs := make([]string, len(lvl.Items))
				for i, it := range lvl.Items {
					gotIDs[i] = it.ID
				}
				wantIDs := make([]string, len(want))
				for i, it := range want {
					wantIDs[i] = it.ID
				}
				if !slices.Equal(gotIDs, wantIDs) {
					t.Fatalf("items = %v, want %v", gotIDs, wantIDs)
				}
				if node, ok := m.registry.Find(live.ID); ok && lvl.MultiSelect != node.MultiSelect {
					t.Fatalf("MultiSelect = %v, want %v", lvl.MultiSelect, node.MultiSelect)
				}
			})
		}
	}
}
