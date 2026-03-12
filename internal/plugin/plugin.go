package plugin

import (
	"os"
	"path/filepath"
	"time"
)

// Plugin represents a declared or installed tmux plugin.
type Plugin struct {
	Name      string
	Source    string
	Branch    string
	Dir       string
	Installed bool
	UpdatedAt time.Time
	IsSymlink bool
}

// AllPluginsSentinel is the ID used for the "update all" toggle in the menu.
const AllPluginsSentinel = "__all__"

// PluginDir resolves the plugin installation directory.
// Priority: TMUX_PLUGIN_MANAGER_PATH env var > XDG > ~/.tmux/plugins/
func PluginDir() string {
	if p := os.Getenv("TMUX_PLUGIN_MANAGER_PATH"); p != "" {
		return p
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		conf := filepath.Join(xdg, "tmux", "tmux.conf")
		if _, err := os.Stat(conf); err == nil {
			return filepath.Join(xdg, "tmux", "plugins")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".tmux", "plugins")
	}
	return filepath.Join(home, ".tmux", "plugins")
}
