package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSource_ExecutesTmuxFiles(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(dir, "marker.txt")
	script := filepath.Join(pluginDir, "test.tmux")
	content := "#!/bin/sh\necho sourced > " + marker + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	plugins := []Plugin{{Name: "test-plugin", Dir: pluginDir, Installed: true}}
	if err := Source(dir, plugins); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal("marker file not created — script was not executed")
	}
	if string(data) != "sourced\n" {
		t.Errorf("marker content = %q, want %q", string(data), "sourced\n")
	}
}

func TestSource_SkipsUninstalledPlugins(t *testing.T) {
	plugins := []Plugin{{Name: "ghost", Dir: "/nonexistent", Installed: false}}
	if err := Source("/tmp", plugins); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSource_SkipsNonExecutableFiles(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	script := filepath.Join(pluginDir, "readme.tmux")
	if err := os.WriteFile(script, []byte("not a script"), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins := []Plugin{{Name: "test-plugin", Dir: pluginDir, Installed: true}}
	err := Source(dir, plugins)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
