package state

import (
	"slices"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// Level encapsulates menu level state such as cursor position, filter, and viewport.
type Level struct {
	ID             string
	Title          string
	Subtitle       string
	Items          []menu.Item
	Full           []menu.Item
	Filter         string
	FilterCursor   int
	Cursor         int
	MultiSelect    bool
	Selected       map[string]struct{}
	Data           any
	LastCursor     int
	Node           *menu.Node
	ViewportOffset int
}

// NewLevel constructs a Level using the provided items and menu node.
func NewLevel(id, title string, items []menu.Item, node *menu.Node) *Level {
	l := &Level{
		ID:         id,
		Title:      title,
		Cursor:     -1,
		LastCursor: -1,
		Selected:   make(map[string]struct{}),
		Node:       node,
	}
	l.UpdateItems(items)
	return l
}

// IndexOf returns the index for a given item identifier.
func (l *Level) IndexOf(id string) int {
	if id == "" {
		return -1
	}
	if i := slices.IndexFunc(l.Items, func(item menu.Item) bool { return item.ID == id }); i >= 0 {
		return i
	}
	if idx := strings.LastIndex(id, ":"); idx >= 0 {
		suffix := id[idx+1:]
		if i := slices.IndexFunc(l.Items, func(item menu.Item) bool { return item.ID == suffix }); i >= 0 {
			return i
		}
	}
	return -1
}

// UpdateItems refreshes the level items while preserving selections if possible.
func (l *Level) UpdateItems(items []menu.Item) {
	prevOffset := l.ViewportOffset
	l.Full = CloneItems(items)
	l.CleanupSelections()
	l.applyFilter()
	if len(l.Items) == 0 {
		l.ViewportOffset = 0
		return
	}
	prevOffset = max(prevOffset, 0)
	if prevOffset > len(l.Items)-1 {
		l.ViewportOffset = 0
		return
	}
	l.ViewportOffset = prevOffset
}
