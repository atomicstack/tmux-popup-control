package command

import (
	"fmt"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	tea "github.com/charmbracelet/bubbletea"
)

// Request encapsulates an action invocation.
type Request struct {
	ID      string
	Label   string
	Handler menu.Action
	Item    menu.Item
}

// Bus coordinates the execution of menu actions.
type Bus struct{}

// New initialises a command bus instance.
func New() *Bus {
	return &Bus{}
}

// Execute wraps a menu action into a Bubble Tea command while emitting trace logs.
func (b *Bus) Execute(ctx menu.Context, req Request) tea.Cmd {
	events.Command.Queue(req.ID, req.Label)
	return func() tea.Msg {
		if req.Handler == nil {
			events.Command.Skip(req.ID, req.Label)
			return nil
		}
		cmd := req.Handler(ctx, req.Item)
		if cmd == nil {
			events.Command.NoOp(req.ID, req.Label)
			return nil
		}
		msg := cmd()
		events.Command.Result(req.ID, req.Label, fmt.Sprintf("%T", msg))
		return msg
	}
}
