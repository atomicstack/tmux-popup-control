package menu

func loadWindowMenu(Context) ([]Item, error) {
	items := []string{
		"switch",
		"link",
		"move",
		"swap",
		"rename",
		"kill",
	}
	return menuItemsFromIDs(items), nil
}
