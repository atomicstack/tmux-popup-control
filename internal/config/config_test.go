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
