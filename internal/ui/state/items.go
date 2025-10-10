package state

import "github.com/atomicstack/tmux-popup-control/internal/menu"

// CloneItems produces a shallow copy of the provided menu items.
func CloneItems(items []menu.Item) []menu.Item {
	dup := make([]menu.Item, len(items))
	copy(dup, items)
	return dup
}
