package state

import (
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// Level encapsulates menu level state such as cursor position, filter, and viewport.
type Level struct {
	ID             string
	Title          string
	Items          []menu.Item
	Full           []menu.Item
	Filter         string
	FilterCursor   int
	Cursor         int
	MultiSelect    bool
	Selected       map[string]struct{}
	Data           interface{}
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
	for i, item := range l.Items {
		if item.ID == id {
			return i
		}
	}
	if idx := strings.LastIndex(id, ":"); idx >= 0 {
		suffix := id[idx+1:]
		for i, item := range l.Items {
			if item.ID == suffix {
				return i
			}
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
	if prevOffset < 0 {
		prevOffset = 0
	}
	if prevOffset > len(l.Items)-1 {
		l.ViewportOffset = 0
		return
	}
	l.ViewportOffset = prevOffset
}
