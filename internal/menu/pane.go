package menu

func loadPaneMenu(Context) ([]Item, error) {
	items := []string{
		"switch",
		"break",
		"join",
		"swap",
		"layout",
		"kill",
		"resize",
	}
	return menuItemsFromIDs(items), nil
}
