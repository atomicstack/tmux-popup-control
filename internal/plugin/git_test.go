package plugin

import (
	"strings"
	"testing"
)

func TestInstallPlugin_ClonesUninstalledPlugins(t *testing.T) {
	var calls [][]string
	withStubGit(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte("ok"), nil
	})

	plugins := []Plugin{
		{Name: "tmux-sensible", Source: "tmux-plugins/tmux-sensible", Dir: "/tmp/plugins/tmux-sensible", Installed: false},
		{Name: "already-here", Source: "user/already-here", Dir: "/tmp/plugins/already-here", Installed: true},
	}

	err := Install("/tmp/plugins", plugins)
	if err != nil {
		t.Fatal(err)
	}

	if len(calls) != 1 {
		t.Fatalf("got %d git calls, want 1", len(calls))
	}
	args := strings.Join(calls[0], " ")
	if !strings.Contains(args, "clone") {
		t.Errorf("expected clone command, got: %s", args)
	}
	if !strings.Contains(args, "--single-branch") {
		t.Errorf("expected --single-branch, got: %s", args)
	}
	if !strings.Contains(args, "--recursive") {
		t.Errorf("expected --recursive, got: %s", args)
	}
}

func TestInstallPlugin_WithBranch(t *testing.T) {
	var calls [][]string
	withStubGit(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte("ok"), nil
	})

	plugins := []Plugin{
		{Name: "my-plugin", Source: "user/my-plugin", Branch: "dev", Dir: "/tmp/plugins/my-plugin"},
	}

	if err := Install("/tmp/plugins", plugins); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(calls[0], " ")
	if !strings.Contains(args, "-b dev") {
		t.Errorf("expected -b dev, got: %s", args)
	}
}

func TestUpdate_PullsAndUpdatesSubmodules(t *testing.T) {
	var calls [][]string
	withStubGit(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte("ok"), nil
	})

	plugins := []Plugin{
		{Name: "tmux-sensible", Dir: "/tmp/plugins/tmux-sensible", Installed: true},
	}

	if err := Update("/tmp/plugins", plugins); err != nil {
		t.Fatal(err)
	}

	if len(calls) != 2 {
		t.Fatalf("got %d git calls, want 2 (pull + submodule update)", len(calls))
	}

	pullArgs := strings.Join(calls[0], " ")
	if !strings.Contains(pullArgs, "pull") {
		t.Errorf("first call should be pull, got: %s", pullArgs)
	}

	subArgs := strings.Join(calls[1], " ")
	if !strings.Contains(subArgs, "submodule") {
		t.Errorf("second call should be submodule update, got: %s", subArgs)
	}
}
