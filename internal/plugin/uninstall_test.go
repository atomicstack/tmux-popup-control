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

func TestTidy_ReturnsUndeclaredPlugins(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"tmux-sensible", "orphaned-plugin", "tmux-popup-control"} {
		if err := os.MkdirAll(filepath.Join(dir, name, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	declared := []Plugin{
		{Name: "tmux-sensible"},
	}

	toRemove, err := Tidy(dir, declared)
	if err != nil {
		t.Fatal(err)
	}

	// Should include orphaned-plugin but NOT tmux-popup-control (self) or tmux-sensible (declared)
	if len(toRemove) != 1 {
		t.Fatalf("got %d plugins to remove, want 1", len(toRemove))
	}
	if toRemove[0].Name != "orphaned-plugin" {
		t.Errorf("expected orphaned-plugin, got %s", toRemove[0].Name)
	}
}

func TestTidy_NeverRemovesSelf(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tmux-popup-control", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	toRemove, err := Tidy(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range toRemove {
		if p.Name == "tmux-popup-control" {
			t.Error("Tidy should never include tmux-popup-control")
		}
	}
}
