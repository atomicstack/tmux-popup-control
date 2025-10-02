package menu

func loadProcessMenu(Context) ([]Item, error) {
	items := []string{
		"display",
		"tree",
		"terminate",
		"kill",
		"interrupt",
		"continue",
		"stop",
		"quit",
		"hangup",
	}
	return menuItemsFromIDs(items), nil
}
