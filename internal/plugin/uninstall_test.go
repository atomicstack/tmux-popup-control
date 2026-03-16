package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstall_RemovesPluginDirs(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "tmux-sensible")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := Uninstall(dir, []Plugin{{Name: "tmux-sensible", Dir: pluginDir}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Error("plugin directory should have been removed")
	}
}

