package menu

import (
	"strings"
	"unicode"
)

// Item represents a selectable menu entry.
type Item struct {
	ID    string
	Label string
}

// Level describes a breadcrumb component for display purposes.
type Level struct {
	ID    string
	Title string
	Items []Item
}

// Context carries runtime data needed by loader functions.
type Context struct {
	SocketPath string
	Sessions   []string
}

// Loader populates submenu entries on demand.
type Loader func(Context) ([]Item, error)

// RootItems returns the top-level menu entries.
func RootItems() []Item {
	return []Item{
		{ID: "process", Label: "process"},
		{ID: "clipboard", Label: "clipboard"},
		{ID: "keybinding", Label: "keybinding"},
		{ID: "command", Label: "command"},
		{ID: "pane", Label: "pane"},
		{ID: "window", Label: "window"},
		{ID: "session", Label: "session"},
	}
}

// CategoryLoaders lists submenu loaders keyed by root item ID.
func CategoryLoaders() map[string]Loader {
	return map[string]Loader{
		"process":    loadProcessMenu,
		"clipboard":  loadClipboardMenu,
		"keybinding": loadKeybindingMenu,
		"command":    loadCommandMenu,
		"pane":       loadPaneMenu,
		"window":     loadWindowMenu,
		"session":    loadSessionMenu,
	}
}

// ActionLoaders enumerates loaders for nested submenu actions.
func ActionLoaders() map[string]Loader {
	return map[string]Loader{
		"session:switch": loadSessionSwitchMenu,
	}
}

func menuItemsFromIDs(ids []string) []Item {
	items := make([]Item, 0, len(ids))
	for _, id := range ids {
		items = append(items, Item{ID: id, Label: prettyLabel(id)})
	}
	return items
}

func prettyLabel(id string) string {
	if id == "" {
		return id
	}
	parts := strings.FieldsFunc(id, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		for j := 1; j < len(runes); j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}
