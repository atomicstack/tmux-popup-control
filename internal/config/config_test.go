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

func TestFlagsOverrideEnvironment(t *testing.T) {
	cfg, err := LoadArgs(
		[]string{"--menu-args", "flag-value", "--width", "80", "--trace"},
		[]string{
			"TMUX_POPUP_CONTROL_MENU_ARGS=env-value",
			"TMUX_POPUP_CONTROL_WIDTH=20",
			"TMUX_POPUP_CONTROL_TRACE=false",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.MenuArgs != "flag-value" {
		t.Fatalf("expected flag to override env menu args, got %q", cfg.App.MenuArgs)
	}
	if cfg.App.Width != 80 {
		t.Fatalf("expected flag to override env width, got %d", cfg.App.Width)
	}
	if !cfg.Logging.Trace {
		t.Fatal("expected trace flag to override env")
	}
}

func TestNegativeDimensionsRejected(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "width", args: []string{"--width", "-1"}},
		{name: "height", args: []string{"--height", "-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := LoadArgs(tt.args, nil); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestInvalidEnvironmentValuesFallBackToDefaults(t *testing.T) {
	cfg, err := LoadArgs(nil, []string{
		"TMUX_POPUP_CONTROL_WIDTH=oops",
		"TMUX_POPUP_CONTROL_HEIGHT=",
		"TMUX_POPUP_CONTROL_FOOTER=maybe",
		"TMUX_POPUP_CONTROL_TRACE=wat",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Width != 0 || cfg.App.Height != 0 {
		t.Fatalf("invalid numeric env vars should fall back to 0, got width=%d height=%d", cfg.App.Width, cfg.App.Height)
	}
	if cfg.App.ShowFooter {
		t.Fatal("invalid footer env var should fall back to false")
	}
	if cfg.Logging.Trace {
		t.Fatal("invalid trace env var should fall back to false")
	}
}

func TestVersionRequested(t *testing.T) {
	if _, err := LoadArgs([]string{"--version"}, nil); err != ErrVersionRequested {
		t.Fatalf("expected ErrVersionRequested, got %v", err)
	}
}
