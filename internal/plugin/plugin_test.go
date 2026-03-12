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

func TestInstalled_ScansDirectory(t *testing.T) {
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "tmux-sensible")
	gitDir := filepath.Join(pluginDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	symlinkTarget := t.TempDir()
	symlinkPath := filepath.Join(dir, "tmux-popup-control")
	if err := os.Symlink(symlinkTarget, symlinkPath); err != nil {
		t.Fatal(err)
	}

	plugins, err := Installed(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}

	byName := map[string]Plugin{}
	for _, p := range plugins {
		byName[p.Name] = p
	}

	sensible, ok := byName["tmux-sensible"]
	if !ok {
		t.Fatal("missing tmux-sensible")
	}
	if sensible.IsSymlink {
		t.Error("tmux-sensible should not be a symlink")
	}
	if !sensible.Installed {
		t.Error("tmux-sensible should be marked installed")
	}

	popup, ok := byName["tmux-popup-control"]
	if !ok {
		t.Fatal("missing tmux-popup-control")
	}
	if !popup.IsSymlink {
		t.Error("tmux-popup-control should be a symlink")
	}
}

func TestInstalled_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	plugins, err := Installed(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestInstalled_NonexistentDir(t *testing.T) {
	plugins, err := Installed("/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}
