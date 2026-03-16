package menu

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

// AllPluginsSentinel is the ID used for the "update all" toggle in the plugin menu.
const AllPluginsSentinel = "__all__"

func loadPluginsMenu(_ Context) ([]Item, error) {
	return menuItemsFromIDs([]string{"install", "update", "uninstall"}), nil
}

func loadPluginsUpdateMenu(ctx Context) ([]Item, error) {
	pluginDir := plugin.PluginDir()
	installed, err := plugin.Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	declaredNames := declaredPluginNames(ctx.SocketPath)
	rows := make([][]string, len(installed))
	for i, p := range installed {
		status := pluginDeclStatus(declaredNames, p.Name)
		date := ""
		if !p.UpdatedAt.IsZero() {
			date = p.UpdatedAt.Format(time.DateOnly)
		}
		rows[i] = []string{p.Name, status, date}
	}
	aligned := table.Format(rows, []table.Alignment{table.AlignLeft, table.AlignLeft, table.AlignRight})
	items := make([]Item, 0, len(installed)+1)
	items = append(items, Item{ID: AllPluginsSentinel, Label: "[all]"})
	for i, p := range installed {
		status := pluginDeclStatus(declaredNames, p.Name)
		items = append(items, Item{
			ID:          p.Name,
			Label:       aligned[i],
			StyledLabel: pluginStatusReplace(aligned[i], status),
		})
	}
	return items, nil
}

func loadPluginsUninstallMenu(ctx Context) ([]Item, error) {
	pluginDir := plugin.PluginDir()
	installed, err := plugin.Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	declaredNames := declaredPluginNames(ctx.SocketPath)
	rows := make([][]string, len(installed))
	for i, p := range installed {
		rows[i] = []string{p.Name, pluginDeclStatus(declaredNames, p.Name)}
	}
	aligned := table.Format(rows, []table.Alignment{table.AlignLeft, table.AlignLeft})
	items := make([]Item, 0, len(installed)+1)
	items = append(items, Item{ID: AllPluginsSentinel, Label: "[all]"})
	for i, p := range installed {
		status := pluginDeclStatus(declaredNames, p.Name)
		items = append(items, Item{
			ID:          p.Name,
			Label:       aligned[i],
			StyledLabel: pluginStatusReplace(aligned[i], status),
		})
	}
	return items, nil
}

// PluginInstallStart carries the list of uninstalled plugins for per-plugin
// progress tracking in the UI.
type PluginInstallStart struct {
	Plugins   []plugin.Plugin
	PluginDir string
}

// PluginUpdateStart carries the list of plugins to update with per-plugin
// progress tracking in the UI.
type PluginUpdateStart struct {
	Plugins   []plugin.Plugin
	PluginDir string
}

// PluginConfirmPrompt is sent before destructive plugin operations.
type PluginConfirmPrompt struct {
	Plugins   []plugin.Plugin
	PluginDir string
	Operation string // "uninstall"
}

func PluginsInstallAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		plugins, err := plugin.ParseConfig(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: err}
		}
		var toInstall []plugin.Plugin
		for _, p := range plugins {
			if !p.Installed {
				toInstall = append(toInstall, p)
			}
		}
		if len(toInstall) == 0 {
			return ActionResult{Info: "All plugins already installed"}
		}
		return PluginInstallStart{
			Plugins:   toInstall,
			PluginDir: pluginDir,
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

		if len(toUpdate) == 0 {
			return ActionResult{Info: "No plugins selected"}
		}
		return PluginUpdateStart{
			Plugins:   toUpdate,
			PluginDir: pluginDir,
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
		uninstallAll := false
		for _, id := range selected {
			if id == AllPluginsSentinel {
				uninstallAll = true
				break
			}
		}

		var toRemove []plugin.Plugin
		if uninstallAll {
			toRemove = installed
		} else {
			nameSet := make(map[string]struct{}, len(selected))
			for _, id := range selected {
				nameSet[id] = struct{}{}
			}
			for _, p := range installed {
				if _, ok := nameSet[p.Name]; ok {
					toRemove = append(toRemove, p)
				}
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

// declaredPluginNames returns the set of plugin names declared in tmux config.
func declaredPluginNames(socketPath string) map[string]struct{} {
	if socketPath == "" {
		return nil
	}
	declared, err := plugin.ParseConfig(socketPath)
	if err != nil {
		return nil
	}
	names := make(map[string]struct{}, len(declared))
	for _, p := range declared {
		names[p.Name] = struct{}{}
	}
	return names
}

// pluginDeclStatus returns "installed" or "undeclared" based on whether the
// plugin name appears in the declared set.
func pluginDeclStatus(declaredNames map[string]struct{}, name string) string {
	if declaredNames == nil {
		return "installed"
	}
	if _, ok := declaredNames[name]; ok {
		return "installed"
	}
	return "undeclared"
}

// pluginStatusReplace returns a copy of line with the status keyword replaced
// by an ANSI-colored version. Uses foreground-only SGR codes (\x1b[39m resets
// fg without affecting background) so the menu line's highlight is preserved.
func pluginStatusReplace(line, status string) string {
	colored := pluginStatusColored(status)
	if colored == status {
		return line
	}
	return strings.Replace(line, status, colored, 1)
}

// Plugin status colors — same values as the preview panel (34, 172, 93).
// Uses ansi.Style with ForegroundColor/DefaultForegroundColor so the
// enclosing line's background is preserved (no full reset).
var (
	pluginStatusFgInstalled  = ansi.NewStyle().ForegroundColor(ansi.IndexedColor(34)).String()
	pluginStatusFgNot        = ansi.NewStyle().ForegroundColor(ansi.IndexedColor(172)).String()
	pluginStatusFgUndeclared = ansi.NewStyle().ForegroundColor(ansi.IndexedColor(93)).String()
	pluginStatusFgReset      = ansi.NewStyle().ForegroundColor(nil).String()
)

func pluginStatusColored(status string) string {
	var open string
	switch status {
	case "installed":
		open = pluginStatusFgInstalled
	case "not installed":
		open = pluginStatusFgNot
	case "undeclared":
		open = pluginStatusFgUndeclared
	default:
		return status
	}
	return open + status + pluginStatusFgReset
}
