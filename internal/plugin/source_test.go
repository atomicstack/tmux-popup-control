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

func TestSourceSelf_SourcesOwnTmuxWhenNotDeclared(t *testing.T) {
	selfDir := t.TempDir()
	fakeBinary := filepath.Join(selfDir, "tmux-popup-control")
	if err := os.WriteFile(fakeBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(selfDir, "marker.txt")
	script := filepath.Join(selfDir, "main.tmux")
	content := "#!/bin/sh\necho sourced > " + marker + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	orig := executablePath
	executablePath = func() (string, error) { return fakeBinary, nil }
	defer func() { executablePath = orig }()

	// No @plugin declaration for tmux-popup-control — its keys must still bind.
	if err := SourceSelf(nil); err != nil {
		t.Fatalf("SourceSelf: %v", err)
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal("marker file not created — own main.tmux was not sourced")
	}
	if string(data) != "sourced\n" {
		t.Errorf("marker content = %q, want %q", string(data), "sourced\n")
	}
}

func TestSourceSelf_SkipsWhenAlreadyDeclared(t *testing.T) {
	selfDir := t.TempDir()
	fakeBinary := filepath.Join(selfDir, "tmux-popup-control")
	if err := os.WriteFile(fakeBinary, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(selfDir, "marker.txt")
	script := filepath.Join(selfDir, "main.tmux")
	content := "#!/bin/sh\necho sourced > " + marker + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	orig := executablePath
	executablePath = func() (string, error) { return fakeBinary, nil }
	defer func() { executablePath = orig }()

	// A declared plugin already covers the binary's directory, so Source will
	// source main.tmux; SourceSelf must skip it to avoid binding keys twice.
	declared := []Plugin{{Name: "tmux-popup-control", Dir: selfDir, Installed: true}}
	if err := SourceSelf(declared); err != nil {
		t.Fatalf("SourceSelf: %v", err)
	}

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("marker file created — SourceSelf should have skipped the already-declared self dir")
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
