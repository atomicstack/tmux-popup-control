package menu

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func loadSessionMenu(Context) ([]Item, error) {
	items := []string{
		"switch",
		"new",
		"rename",
		"detach",
		"kill",
	}
	return menuItemsFromIDs(items), nil
}

func loadSessionSwitchMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Sessions))
	for _, sess := range ctx.Sessions {
		items = append(items, Item{ID: sess, Label: sess})
	}
	return items, nil
}

func SessionSwitchAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		if err := tmux.SwitchClient(ctx.SocketPath, item.ID); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Switched to %s", item.Label)}
	}
}
