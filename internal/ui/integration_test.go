package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	tea "github.com/charmbracelet/bubbletea"
)

func TestWindowSwitchPaginationRespectsViewport(t *testing.T) {
	model := NewModel("/tmp/tmux.sock", 40, 8, false, false, nil, "", "")
	harness := NewHarness(model)

	harness.Send(tea.WindowSizeMsg{Width: 40, Height: 8})

	windows := make([]tmux.Window, 10)
	for i := range windows {
		idx := i + 1
		windows[i] = tmux.Window{
			ID:      fmt.Sprintf("win-%02d", idx),
			Label:   fmt.Sprintf("win-%02d", idx),
			Name:    fmt.Sprintf("win-%02d", idx),
			Session: "s:1",
			Index:   idx,
			Current: idx == 1,
		}
	}
	snapshot := tmux.WindowSnapshot{
		Windows:        windows,
		CurrentID:      windows[0].ID,
		CurrentLabel:   windows[0].Label,
		CurrentSession: "s:1",
		IncludeCurrent: true,
	}
	harness.Send(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: snapshot}})

	// Navigate into the window menu from the root menu.
	root := harness.Model().stack[0]
	root.Cursor = root.IndexOf("window")
	harness.Send(tea.KeyMsg{Type: tea.KeyEnter})

	ctx := harness.Model().menuContext()
	items := make([]menu.Item, 0, len(ctx.Windows))
	for _, entry := range ctx.Windows {
		if entry.Current && !ctx.WindowIncludeCurrent {
			continue
		}
		items = append(items, menu.Item{ID: entry.ID, Label: entry.Label})
	}
	node, _ := harness.Model().registry.Find("window:switch")
	level := newLevel("window:switch", "switch", items, node)
	level.Cursor = 0
	harness.Model().stack = append(harness.Model().stack, level)
	harness.Model().applyNodeSettings(level)
	harness.Model().syncViewport(level)

	view := harness.View()
	if strings.Contains(view, "win-07") {
		t.Fatalf("expected win-07 to be outside initial viewport, view =\n%s", view)
	}

	for i := 0; i < 7; i++ {
		harness.Send(tea.KeyMsg{Type: tea.KeyDown})
	}
	view = harness.View()
	if !strings.Contains(view, "win-08") {
		t.Fatalf("expected win-08 to be visible after scrolling, view =\n%s", view)
	}
}
