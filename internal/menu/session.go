package menu

import "github.com/atomicstack/tmux-popup-control/internal/tmux"

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
	sessions, err := tmux.FetchSessions(ctx.SocketPath)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(sessions))
	for _, sess := range sessions {
		items = append(items, Item{ID: sess, Label: sess})
	}
	return items, nil
}
