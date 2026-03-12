package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginDir_EnvVar(t *testing.T) {
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", "/custom/plugins")
	got := PluginDir()
	if got != "/custom/plugins" {
		t.Errorf("PluginDir() = %q, want %q", got, "/custom/plugins")
	}
}

func TestPluginDir_XDG(t *testing.T) {
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", "")
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	tmuxDir := filepath.Join(xdg, "tmux")
	if err := os.MkdirAll(tmuxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmuxDir, "tmux.conf"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := PluginDir()
	want := filepath.Join(xdg, "tmux", "plugins")
	if got != want {
		t.Errorf("PluginDir() = %q, want %q", got, want)
	}
}

func TestPluginDir_Default(t *testing.T) {
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	got := PluginDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".tmux", "plugins")
	if got != want {
		t.Errorf("PluginDir() = %q, want %q", got, want)
	}
}
