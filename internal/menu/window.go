package menu

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

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

func loadWindowSwitchMenu(ctx Context) ([]Item, error) {
	return WindowEntriesToItems(ctx.Windows), nil
}

func loadWindowKillMenu(ctx Context) ([]Item, error) {
	return WindowEntriesToItems(ctx.Windows), nil
}

func WindowSwitchAction(ctx Context, item Item) tea.Cmd {
	windowID := item.ID
	parts := strings.SplitN(windowID, ":", 2)
	if len(parts) != 2 {
		err := fmt.Errorf("invalid window id: %s", windowID)
		return func() tea.Msg { return ActionResult{Err: err} }
	}
	session := parts[0]
	label := item.Label
	return func() tea.Msg {
		logging.Trace("window.switch", map[string]interface{}{"target": windowID})
		if err := tmux.SwitchClient(ctx.SocketPath, session); err != nil {
			return ActionResult{Err: err}
		}
		if err := tmux.SelectWindow(ctx.SocketPath, windowID); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Switched to %s", label)}
	}
}

func WindowKillAction(ctx Context, item Item) tea.Cmd {
	windowID := item.ID
	label := item.Label
	return func() tea.Msg {
		logging.Trace("window.kill", map[string]interface{}{"target": windowID})
		if err := tmux.KillWindow(ctx.SocketPath, windowID); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Killed %s", label)}
	}
}

func WindowEntriesFromTmux(windows []tmux.Window) []WindowEntry {
	entries := make([]WindowEntry, 0, len(windows))
	for _, w := range windows {
		id := fmt.Sprintf("%s:%d", w.Session, w.Index)
		label := fmt.Sprintf("%s:%d %s", w.Session, w.Index, w.Name)
		entries = append(entries, WindowEntry{ID: id, Label: label})
	}
	return entries
}
