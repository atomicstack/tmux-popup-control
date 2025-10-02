package menu

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
