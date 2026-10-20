package menu

import (
	"fmt"
	"strings"
)

// LiveSource identifies which backend data set a live level is rebuilt from.
type LiveSource uint8

const (
	// LiveSessions levels rebuild when the session list changes.
	LiveSessions LiveSource = iota + 1
	// LiveWindows levels rebuild when the window list changes.
	LiveWindows
	// LivePanes levels rebuild when the pane list changes.
	LivePanes
)

// LiveLevel declares how a menu level showing tmux state rebuilds its items
// from a fresh Context whenever its Source changes.
type LiveLevel struct {
	ID     string
	Source LiveSource
	Items  func(Context) []Item
}

// liveLevels is the table of levels with a plain "rebuild items from context"
// refresh. Order is preserved per source. Levels needing bespoke refresh logic
// (session:tree, window:pull-from-session, window move, restore picker) are
// handled by the UI and are deliberately absent.
var liveLevels = []LiveLevel{
	{ID: "session:switch", Source: LiveSessions, Items: SessionSwitchMenuItems},
	{ID: "session:rename", Source: LiveSessions, Items: func(c Context) []Item { return SessionRenameItems(c.Sessions) }},
	{ID: "session:detach", Source: LiveSessions, Items: func(c Context) []Item { return SessionEntriesToItems(c.Sessions) }},
	{ID: "session:kill", Source: LiveSessions, Items: func(c Context) []Item { return SessionEntriesToItems(c.Sessions) }},
	{ID: "window:push-to-session", Source: LiveSessions, Items: WindowPushToSessionItems},

	{ID: "window:switch", Source: LiveWindows, Items: WindowSwitchItems},
	{ID: "window:swap", Source: LiveWindows, Items: WindowSwapItems},
	{ID: "window:kill", Source: LiveWindows, Items: WindowSwapItems},

	{ID: "pane:switch", Source: LivePanes, Items: PaneSwitchItems},
	{ID: "pane:break", Source: LivePanes, Items: PaneBreakItems},
	{ID: "pane:join", Source: LivePanes, Items: PaneJoinItems},
	{ID: "pane:swap", Source: LivePanes, Items: PaneSwapItems},
	{ID: "pane:kill", Source: LivePanes, Items: PaneKillItems},
	{ID: "pane:rename", Source: LivePanes, Items: func(c Context) []Item { return PaneEntriesToItems(c.Panes) }},
}

// LiveLevels returns, in refresh order, the live levels rebuilt from src.
func LiveLevels(src LiveSource) []LiveLevel {
	var out []LiveLevel
	for _, l := range liveLevels {
		if l.Source == src {
			out = append(out, l)
		}
	}
	return out
}

// LiveLevelItems returns the item builder for a live level id.
func LiveLevelItems(id string) (func(Context) []Item, bool) {
	for _, l := range liveLevels {
		if l.ID == id {
			return l.Items, true
		}
	}
	return nil, false
}

// WindowPushToSessionItems lists every session except the current window's.
func WindowPushToSessionItems(ctx Context) []Item {
	items := make([]Item, 0, len(ctx.Sessions))
	for _, entry := range ctx.Sessions {
		if entry.Name == ctx.CurrentWindowSession {
			continue
		}
		items = append(items, Item{ID: entry.Name, Label: entry.Label})
	}
	return items
}

// WindowSwapItems lists all windows, prefixed by a "[current]" entry.
func WindowSwapItems(ctx Context) []Item {
	items := WindowEntriesToItems(ctx.Windows)
	if currentItem, ok := CurrentWindowMenuItem(ctx); ok {
		items = append([]Item{currentItem}, items...)
	}
	return items
}

// CurrentWindowMenuItem builds the "[current]" entry for the current window.
func CurrentWindowMenuItem(ctx Context) (Item, bool) {
	return currentMenuItem(ctx.CurrentWindowID, ctx.CurrentWindowLabel)
}

// CurrentPaneMenuItem builds the "[current]" entry for the current pane.
func CurrentPaneMenuItem(ctx Context) (Item, bool) {
	return currentMenuItem(ctx.CurrentPaneID, ctx.CurrentPaneLabel)
}

func currentMenuItem(rawID, rawLabel string) (Item, bool) {
	id := strings.TrimSpace(rawID)
	if id == "" {
		return Item{}, false
	}
	label := strings.TrimSpace(rawLabel)
	if label == "" {
		label = id
	}
	return Item{ID: id, Label: fmt.Sprintf("[current] %s", label)}, true
}

// PaneItems lists every pane.
func PaneItems(ctx Context) []Item {
	items := make([]Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		items = append(items, Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}

// PaneSwitchItems lists panes, omitting the current one unless
// PaneIncludeCurrent is set.
func PaneSwitchItems(ctx Context) []Item {
	items := make([]Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		if entry.Current && !ctx.PaneIncludeCurrent {
			continue
		}
		items = append(items, Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}

// PaneJoinItems lists every pane except the current one.
func PaneJoinItems(ctx Context) []Item {
	items := make([]Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		if entry.Current {
			continue
		}
		items = append(items, Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}

// PaneBreakItems lists all panes, prefixed by a "[current]" entry.
func PaneBreakItems(ctx Context) []Item { return paneItemsWithCurrent(ctx) }

// PaneSwapItems lists all panes, prefixed by a "[current]" entry.
func PaneSwapItems(ctx Context) []Item { return paneItemsWithCurrent(ctx) }

// PaneKillItems lists all panes, prefixed by a "[current]" entry.
func PaneKillItems(ctx Context) []Item { return paneItemsWithCurrent(ctx) }

func paneItemsWithCurrent(ctx Context) []Item {
	items := PaneItems(ctx)
	if current, ok := CurrentPaneMenuItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items
}
