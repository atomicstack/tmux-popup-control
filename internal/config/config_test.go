package config

import "testing"

func TestMenuArgsFlag(t *testing.T) {
	cfg, err := LoadArgs([]string{"--menu-args", "expanded"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.MenuArgs != "expanded" {
		t.Fatalf("expected MenuArgs=expanded, got %q", cfg.App.MenuArgs)
	}
}

func TestMenuArgsEnvVar(t *testing.T) {
	cfg, err := LoadArgs(nil, []string{"TMUX_POPUP_CONTROL_MENU_ARGS=expanded"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.MenuArgs != "expanded" {
		t.Fatalf("expected MenuArgs=expanded, got %q", cfg.App.MenuArgs)
	}
}

func TestDebugToSQLiteFlag(t *testing.T) {
	cfg, err := LoadArgs([]string{"--debug-to-sqlite"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Logging.DebugToSQLite {
		t.Fatalf("expected DebugToSQLite=true")
	}
}

func TestCommandArgsRetainedAfterGlobalFlags(t *testing.T) {
	cfg, err := LoadArgs([]string{"--debug-to-sqlite", "install-and-init-plugins"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Command) != 1 || cfg.Command[0] != "install-and-init-plugins" {
		t.Fatalf("expected command [install-and-init-plugins], got %#v", cfg.Command)
	}
}
