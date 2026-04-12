package command

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

type testMsg struct {
	Value string
}

func TestExecuteSkipsNilHandler(t *testing.T) {
	bus := New()

	cmd := bus.Execute(menu.Context{}, Request{ID: "pane:kill", Label: "kill"})
	if got := cmd(); got != nil {
		t.Fatalf("expected nil msg, got %#v", got)
	}
}

func TestExecuteSkipsNilCommand(t *testing.T) {
	bus := New()

	cmd := bus.Execute(menu.Context{}, Request{
		ID:    "pane:kill",
		Label: "kill",
		Handler: func(menu.Context, menu.Item) tea.Cmd {
			return nil
		},
	})
	if got := cmd(); got != nil {
		t.Fatalf("expected nil msg, got %#v", got)
	}
}

func TestExecuteReturnsHandlerMessage(t *testing.T) {
	bus := New()
	want := testMsg{Value: "ok"}

	cmd := bus.Execute(menu.Context{}, Request{
		ID:    "pane:kill",
		Label: "kill",
		Item:  menu.Item{ID: "%1", Label: "shell"},
		Handler: func(menu.Context, menu.Item) tea.Cmd {
			return func() tea.Msg { return want }
		},
	})
	if got := cmd(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
