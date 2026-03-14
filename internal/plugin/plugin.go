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

// Installed scans pluginDir and returns a Plugin for each subdirectory found.
// Returns nil (not error) for nonexistent directories.
func Installed(pluginDir string) ([]Plugin, error) {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var plugins []Plugin
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := entry.Name()
		dir := filepath.Join(pluginDir, name)

		info, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		isSymlink := info.Mode()&os.ModeSymlink != 0

		var updatedAt time.Time
		gitDir := filepath.Join(dir, ".git")
		if gi, err := os.Stat(gitDir); err == nil {
			updatedAt = gi.ModTime()
		} else {
			updatedAt = info.ModTime()
		}

		plugins = append(plugins, Plugin{
			Name:      name,
			Dir:       dir,
			Installed: true,
			UpdatedAt: updatedAt,
			IsSymlink: isSymlink,
		})
	}
	return plugins, nil
}
