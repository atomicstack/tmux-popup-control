package state

import "github.com/atomicstack/tmux-popup-control/internal/menu"

// CleanupSelections drops selections that are no longer present in the item list.
func (l *Level) CleanupSelections() {
	if len(l.Selected) == 0 {
		return
	}
	valid := make(map[string]struct{}, len(l.Full))
	for _, item := range l.Full {
		valid[item.ID] = struct{}{}
	}
	for id := range l.Selected {
		if _, ok := valid[id]; !ok {
			delete(l.Selected, id)
		}
	}
}

// IsSelected reports whether the given id is selected.
func (l *Level) IsSelected(id string) bool {
	if l.Selected == nil {
		return false
	}
	_, ok := l.Selected[id]
	return ok
}

// ToggleSelection toggles selection membership for the supplied id.
func (l *Level) ToggleSelection(id string) {
	if l.Selected == nil {
		l.Selected = make(map[string]struct{})
	}
	if _, ok := l.Selected[id]; ok {
		delete(l.Selected, id)
	} else {
		l.Selected[id] = struct{}{}
	}
}

// ToggleCurrentSelection toggles the selection state at the current cursor.
func (l *Level) ToggleCurrentSelection() {
	if !l.MultiSelect || len(l.Items) == 0 || l.Cursor < 0 || l.Cursor >= len(l.Items) {
		return
	}
	l.ToggleSelection(l.Items[l.Cursor].ID)
}

// ClearSelection clears all selected items.
func (l *Level) ClearSelection() {
	if len(l.Selected) == 0 {
		return
	}
	for id := range l.Selected {
		delete(l.Selected, id)
	}
}

// SelectedItems returns the currently selected items in display order.
func (l *Level) SelectedItems() []menu.Item {
	if len(l.Selected) == 0 {
		return nil
	}
	selected := make([]menu.Item, 0, len(l.Selected))
	for _, item := range l.Items {
		if l.IsSelected(item.ID) {
			selected = append(selected, item)
		}
	}
	return selected
}
