package menu

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
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
	SocketPath           string
	Sessions             []SessionEntry
	Current              string
	IncludeCurrent       bool
	Windows              []WindowEntry
	CurrentWindowID      string
	CurrentWindowLabel   string
	CurrentWindowSession string
	WindowIncludeCurrent bool
	Panes                []PaneEntry
	CurrentPaneID        string
	CurrentPaneLabel     string
	PaneIncludeCurrent   bool
}

// WindowEntry represents a tmux window reference for menu loaders.
type WindowEntry struct {
	ID      string
	Label   string
	Name    string
	Session string
	Index   int
	Current bool
}

// PaneEntry represents a tmux pane reference for menu loaders.
type PaneEntry struct {
	ID        string
	Label     string
	PaneID    string
	Session   string
	Window    string
	WindowIdx int
	Index     int
	Current   bool
	Title     string
}

// SessionEntry represents a tmux session reference for menu loaders.
type SessionEntry struct {
	Name     string
	Label    string
	Attached bool
	Current  bool
	Clients  []string
	Windows  int
}

// Loader populates submenu entries on demand.
type Loader func(Context) ([]Item, error)

type Action func(Context, Item) tea.Cmd

// ActionResult communicates the outcome of executing a menu action.
type ActionResult struct {
	Info string
	Err  error
}

// SessionPrompt requests interactive input for session operations.
type SessionPrompt struct {
	Context Context
	Action  string
	Target  string
	Initial string
}

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

// ActionHandlers maps submenu identifiers to their execution logic.
func ActionHandlers() map[string]Action {
	return map[string]Action{
		"session:new":       SessionNewAction,
		"session:switch":    SessionSwitchAction,
		"session:rename":    SessionRenameAction,
		"session:detach":    SessionDetachAction,
		"session:kill":      SessionKillAction,
		"window:switch":     WindowSwitchAction,
		"window:link":       WindowLinkAction,
		"window:move":       WindowMoveAction,
		"window:swap":       WindowSwapAction,
		"window:rename":     WindowRenameAction,
		"window:kill":       WindowKillAction,
		"keybinding":        KeybindingAction,
		"command":           CommandAction,
		"pane:switch":       PaneSwitchAction,
		"pane:break":        PaneBreakAction,
		"pane:join":         PaneJoinAction,
		"pane:swap":         PaneSwapAction,
		"pane:kill":         PaneKillAction,
		"pane:rename":       PaneRenameAction,
		"pane:layout":       PaneLayoutAction,
		"pane:resize:left":  PaneResizeLeftAction,
		"pane:resize:right": PaneResizeRightAction,
		"pane:resize:up":    PaneResizeUpAction,
		"pane:resize:down":  PaneResizeDownAction,
	}
}

// ActionLoaders enumerates loaders for nested submenu actions.
func ActionLoaders() map[string]Loader {
	return map[string]Loader{
		"session:switch":    loadSessionSwitchMenu,
		"session:rename":    loadSessionRenameMenu,
		"session:detach":    loadSessionDetachMenu,
		"session:kill":      loadSessionKillMenu,
		"window:switch":     loadWindowSwitchMenu,
		"window:link":       loadWindowLinkMenu,
		"window:move":       loadWindowMoveMenu,
		"window:swap":       loadWindowSwapMenu,
		"window:rename":     loadWindowRenameMenu,
		"window:kill":       loadWindowKillMenu,
		"pane:switch":       loadPaneSwitchMenu,
		"pane:break":        loadPaneBreakMenu,
		"pane:join":         loadPaneJoinMenu,
		"pane:swap":         loadPaneSwapMenu,
		"pane:kill":         loadPaneKillMenu,
		"pane:rename":       loadPaneRenameMenu,
		"pane:layout":       loadPaneLayoutMenu,
		"pane:resize":       loadPaneResizeMenu,
		"pane:resize:left":  loadPaneResizeLeftMenu,
		"pane:resize:right": loadPaneResizeRightMenu,
		"pane:resize:up":    loadPaneResizeUpMenu,
		"pane:resize:down":  loadPaneResizeDownMenu,
	}
}

func WindowEntriesToItems(entries []WindowEntry) []Item {
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, Item{ID: entry.ID, Label: entry.Label})
	}
	return items
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
		for j := 1; j < len(runes); j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}
