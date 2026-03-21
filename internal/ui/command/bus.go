package command

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
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
		span := logging.StartSpan("ui", "command.execute", logging.SpanOptions{
			Target: req.ID,
			Attrs: map[string]interface{}{
				"label": req.Label,
			},
		})
		if req.Handler == nil {
			events.Command.Skip(req.ID, req.Label)
			span.EndWithOutcome("skip", nil)
			return nil
		}
		cmd := req.Handler(ctx, req.Item)
		if cmd == nil {
			events.Command.NoOp(req.ID, req.Label)
			span.EndWithOutcome("noop", nil)
			return nil
		}
		msg := cmd()
		span.AddAttr("msg_type", fmt.Sprintf("%T", msg))
		events.Command.Result(req.ID, req.Label, fmt.Sprintf("%T", msg))
		span.End(nil)
		return msg
	}
}
