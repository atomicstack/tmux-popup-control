package menu

import (
	tea "charm.land/bubbletea/v2"
)

// Item represents a selectable menu entry.
type Item struct {
	ID          string
	Label       string
	StyledLabel string // optional; when set, used for display instead of Label
	Header      bool   // non-selectable header row (e.g. column titles)
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
	ClientID             string
	MenuArgs             string
	Sessions             []SessionEntry
	Current              string
	IncludeCurrent       bool
	Windows              []WindowEntry
	CurrentWindowID      string
	CurrentWindowLabel   string
	CurrentWindowSession string
	CurrentWindowLayout  string
	WindowIncludeCurrent bool
	Panes                []PaneEntry
	CurrentPaneID        string
	CurrentPaneLabel     string
	PaneIncludeCurrent   bool
}

// WindowEntry represents a tmux window reference for menu loaders.
type WindowEntry struct {
	ID         string
	Label      string
	Name       string
	Session    string
	Index      int
	InternalID string
	Current    bool
	Layout     string
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
	Command   string
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
	Output string
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
		{ID: "plugins", Label: "plugins"},
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
		"plugins":    loadPluginsMenu,
	}
}

// ActionHandlers maps submenu identifiers to their execution logic.
func ActionHandlers() map[string]Action {
	return map[string]Action{
		"session:new":          SessionNewAction,
		"session:switch":       SessionSwitchAction,
		"session:rename":       SessionRenameAction,
		"session:detach":       SessionDetachAction,
		"session:kill":         SessionKillAction,
		"session:tree":         SessionTreeAction,
		"session:save":         SessionSaveAction,
		"session:save-as":      SessionSaveAsAction,
		"session:restore":      SessionRestoreAction,
		"session:restore-from": SessionRestoreFromAction,
		"window:switch":        WindowSwitchAction,
		"window:link":          WindowLinkAction,
		"window:pull-from-session": WindowPullFromSessionAction,
		"window:push-to-session":   WindowPushToSessionAction,
		"window:swap":          WindowSwapAction,
		"window:rename":        WindowRenameAction,
		"window:kill":          WindowKillAction,
		"window:layout":        WindowLayoutAction,
		"keybinding":           KeybindingAction,
		"pane:switch":          PaneSwitchAction,
		"pane:break":           PaneBreakAction,
		"pane:join":            PaneJoinAction,
		"pane:swap":            PaneSwapAction,
		"pane:kill":            PaneKillAction,
		"pane:rename":          PaneRenameAction,
		"pane:capture":         PaneCaptureAction,
		"pane:resize:left":     PaneResizeLeftAction,
		"pane:resize:right":    PaneResizeRightAction,
		"pane:resize:up":       PaneResizeUpAction,
		"pane:resize:down":     PaneResizeDownAction,
		"plugins:install":      PluginsInstallAction,
		"plugins:update":       PluginsUpdateAction,
		"plugins:uninstall":    PluginsUninstallAction,
	}
}

// ActionLoaders enumerates loaders for nested submenu actions.
func ActionLoaders() map[string]Loader {
	return map[string]Loader{
		"session:switch":       loadSessionSwitchMenu,
		"session:rename":       loadSessionRenameMenu,
		"session:detach":       loadSessionDetachMenu,
		"session:kill":         loadSessionKillMenu,
		"session:tree":         loadSessionTreeMenu,
		"session:restore-from": loadSessionRestoreFromMenu,
		"window:switch":        loadWindowSwitchMenu,
		"window:link":          loadWindowLinkMenu,
		"window:pull-from-session": loadWindowPullFromSessionMenu,
		"window:push-to-session":   loadWindowPushToSessionMenu,
		"window:swap":          loadWindowSwapMenu,
		"window:rename":        loadWindowRenameMenu,
		"window:kill":          loadWindowKillMenu,
		"window:layout":        loadWindowLayoutMenu,
		"pane:switch":          loadPaneSwitchMenu,
		"pane:break":           loadPaneBreakMenu,
		"pane:join":            loadPaneJoinMenu,
		"pane:swap":            loadPaneSwapMenu,
		"pane:kill":            loadPaneKillMenu,
		"pane:rename":          loadPaneRenameMenu,
		"pane:resize":          loadPaneResizeMenu,
		"pane:resize:left":     loadPaneResizeLeftMenu,
		"pane:resize:right":    loadPaneResizeRightMenu,
		"pane:resize:up":       loadPaneResizeUpMenu,
		"pane:resize:down":     loadPaneResizeDownMenu,
		"plugins:update":       loadPluginsUpdateMenu,
		"plugins:uninstall":    loadPluginsUninstallMenu,
	}
}

func WindowEntriesToItems(entries []WindowEntry) []Item {
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}
// menuItemsFromIDs builds menu items using each id as both the ID and
// the display label. IDs must be strictly kebab-case (e.g. "push-to-session").
func menuItemsFromIDs(ids []string) []Item {
	items := make([]Item, 0, len(ids))
	for _, id := range ids {
		items = append(items, Item{ID: id, Label: id})
	}
	return items
}
