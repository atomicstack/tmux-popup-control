package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

// AllPluginsSentinel is the ID used for the "update all" toggle in the plugin menu.
const AllPluginsSentinel = "__all__"

func loadPluginsMenu(_ Context) ([]Item, error) {
	return menuItemsFromIDs([]string{"install", "update", "uninstall", "tidy"}), nil
}

func loadPluginsUpdateMenu(_ Context) ([]Item, error) {
	pluginDir := plugin.PluginDir()
	installed, err := plugin.Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(installed)+1)
	items = append(items, Item{ID: AllPluginsSentinel, Label: "all"})
	for _, p := range installed {
		items = append(items, Item{ID: p.Name, Label: p.Name})
	}
	return items, nil
}

func loadPluginsUninstallMenu(_ Context) ([]Item, error) {
	pluginDir := plugin.PluginDir()
	installed, err := plugin.Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(installed))
	for _, p := range installed {
		items = append(items, Item{ID: p.Name, Label: p.Name})
	}
	return items, nil
}

// PluginReloadPrompt is sent after a plugin operation succeeds.
type PluginReloadPrompt struct {
	Plugins   []plugin.Plugin
	PluginDir string
	Summary   string
}

// PluginConfirmPrompt is sent before destructive plugin operations.
type PluginConfirmPrompt struct {
	Plugins   []plugin.Plugin
	PluginDir string
	Operation string // "uninstall" or "tidy"
}

func PluginsInstallAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		plugins, err := plugin.ParseConfig(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: err}
		}
		var uninstalled int
		for _, p := range plugins {
			if !p.Installed {
				uninstalled++
			}
		}
		if uninstalled == 0 {
			return ActionResult{Info: "All plugins already installed"}
		}
		events.Plugins.Install("all")
		if err := plugin.Install(pluginDir, plugins); err != nil {
			return ActionResult{Err: err}
		}
		return PluginReloadPrompt{
			Plugins:   plugins,
			PluginDir: pluginDir,
			Summary:   fmt.Sprintf("Installed %d plugin(s)", uninstalled),
		}
	}
}

func PluginsUpdateAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		installed, err := plugin.Installed(pluginDir)
		if err != nil {
			return ActionResult{Err: err}
		}

		selected := parseMultiSelectIDs(item.ID)
		updateAll := false
		for _, id := range selected {
			if id == AllPluginsSentinel {
				updateAll = true
				break
			}
		}

		var toUpdate []plugin.Plugin
		if updateAll {
			toUpdate = installed
		} else {
			nameSet := make(map[string]struct{}, len(selected))
			for _, id := range selected {
				nameSet[id] = struct{}{}
			}
			for _, p := range installed {
				if _, ok := nameSet[p.Name]; ok {
					toUpdate = append(toUpdate, p)
				}
			}
		}

		for _, p := range toUpdate {
			events.Plugins.Update(p.Name)
		}
		if err := plugin.Update(pluginDir, toUpdate); err != nil {
			return ActionResult{Err: err}
		}
		return PluginReloadPrompt{
			Plugins:   toUpdate,
			PluginDir: pluginDir,
			Summary:   fmt.Sprintf("Updated %d plugin(s)", len(toUpdate)),
		}
	}
}

func PluginsUninstallAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		installed, err := plugin.Installed(pluginDir)
		if err != nil {
			return ActionResult{Err: err}
		}

		selected := parseMultiSelectIDs(item.ID)
		nameSet := make(map[string]struct{}, len(selected))
		for _, id := range selected {
			nameSet[id] = struct{}{}
		}
		var toRemove []plugin.Plugin
		for _, p := range installed {
			if _, ok := nameSet[p.Name]; ok {
				toRemove = append(toRemove, p)
			}
		}
		if len(toRemove) == 0 {
			return ActionResult{Info: "No plugins selected"}
		}
		return PluginConfirmPrompt{
			Plugins:   toRemove,
			PluginDir: pluginDir,
			Operation: "uninstall",
		}
	}
}

func PluginsTidyAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		declared, err := plugin.ParseConfig(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: err}
		}
		toRemove, err := plugin.Tidy(pluginDir, declared)
		if err != nil {
			return ActionResult{Err: err}
		}
		if len(toRemove) == 0 {
			return ActionResult{Info: "No undeclared plugins found"}
		}
		for _, p := range toRemove {
			events.Plugins.Tidy(p.Name)
		}
		return PluginConfirmPrompt{
			Plugins:   toRemove,
			PluginDir: pluginDir,
			Operation: "tidy",
		}
	}
}

// parseMultiSelectIDs splits a newline-joined ID string from multi-select.
func parseMultiSelectIDs(id string) []string {
	if id == "" {
		return nil
	}
	var ids []string
	for _, s := range strings.Split(id, "\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}
