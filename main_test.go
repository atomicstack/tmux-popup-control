package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/app"
	"github.com/atomicstack/tmux-popup-control/internal/config"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestCollectTTYDetailsIncludesStandardDescriptors(t *testing.T) {
	info := collectTTYDetails()
	if len(info.Probes) != 3 {
		t.Fatalf("expected 3 probe entries, got %d", len(info.Probes))
	}
	expected := []string{"stdin", "stdout", "stderr"}
	for i, name := range expected {
		if info.Probes[i].Name != name {
			t.Fatalf("expected probe %d name %q, got %q", i, name, info.Probes[i].Name)
		}
	}
}

func TestStartupTracePayloadIncludesFlags(t *testing.T) {
	cfg := config.Config{
		App: app.Config{
			SocketPath: "socket-path",
			Width:      80,
			Height:     24,
			ShowFooter: true,
			Verbose:    true,
		},
		Logging: config.Logging{
			FilePath:      "trace.log",
			Trace:         true,
			DebugToSQLite: true,
		},
		Flags: map[string]string{
			"socket":  "socket-path",
			"width":   "80",
			"height":  "24",
			"footer":  "true",
			"verbose": "true",
		},
		Args: []string{"--socket", "socket-path"},
	}

	payload := startupTracePayload(cfg)

	flagsValue, ok := payload["flags"].(map[string]any)
	if !ok {
		t.Fatalf("expected flags map in payload")
	}
	if flagsValue["socket"] != "socket-path" {
		t.Fatalf("expected socket flag %q, got %v", "socket-path", flagsValue["socket"])
	}
	if flagsValue["width"] != "80" {
		t.Fatalf("expected width 80, got %v", flagsValue["width"])
	}
	if flagsValue["height"] != "24" {
		t.Fatalf("expected height 24, got %v", flagsValue["height"])
	}
	if flagsValue["footer"] != "true" {
		t.Fatalf("expected footer flag true, got %v", flagsValue["footer"])
	}
	if flagsValue["trace"] != true {
		t.Fatalf("expected trace flag true, got %v", flagsValue["trace"])
	}
	if flagsValue["verbose"] != "true" {
		t.Fatalf("expected verbose flag true, got %v", flagsValue["verbose"])
	}
	if flagsValue["logFile"] != "trace.log" {
		t.Fatalf("expected log file trace.log, got %v", flagsValue["logFile"])
	}
	if flagsValue["debugToSQLite"] != true {
		t.Fatalf("expected debugToSQLite flag true, got %v", flagsValue["debugToSQLite"])
	}

	if _, ok := payload["tty"].(ttyDetails); !ok {
		t.Fatalf("expected tty details in payload")
	}
	if cfgValue, ok := payload["config"].(config.Config); !ok {
		t.Fatalf("expected config in payload")
	} else if cfgValue.App != cfg.App {
		t.Fatalf("expected app config %#v, got %#v", cfg.App, cfgValue.App)
	}
}

func TestSubcommandHelpersUseParsedCommandArgs(t *testing.T) {
	cfg := config.Config{
		Command: []string{"install-and-init-plugins", "--foo", "bar"},
	}
	if got := subcommand(cfg); got != "install-and-init-plugins" {
		t.Fatalf("expected install-and-init-plugins, got %q", got)
	}
	wantArgs := []string{"--foo", "bar"}
	if got := subcommandArgs(cfg); !reflect.DeepEqual(got, wantArgs) {
		t.Fatalf("expected %#v, got %#v", wantArgs, got)
	}
}

func TestAutoSaveNowUsesResolvedSettings(t *testing.T) {
	var gotSocketArg string
	var got resurrect.NowConfig
	deps := MainDeps{
		ResolveSocketPath: func(s string) (string, error) {
			gotSocketArg = s
			return "/tmp/tmux.sock", nil
		},
		ResolveSaveDir:      func(string) (string, error) { return "/tmp/saves", nil },
		ResolvePaneContents: func(string) bool { return true },
		ResolveAutosaveMax:  func(string) int { return 9 },
		ResolveAutosaveIcon: func(string) string { return "X " },
		RunAutoSaveNow: func(cfg resurrect.NowConfig) error {
			got = cfg
			return nil
		},
	}

	err := runAutosaveNow(config.Config{
		App:     app.Config{SocketPath: "app-socket"},
		Command: []string{"autosave-now", "-socket", "flag-socket"},
	}, deps)
	if err != nil {
		t.Fatalf("runAutosaveNow: %v", err)
	}
	if gotSocketArg != "flag-socket" {
		t.Fatalf("expected the -socket flag to win, got %q", gotSocketArg)
	}
	want := resurrect.NowConfig{SocketPath: "/tmp/tmux.sock", SaveDir: "/tmp/saves", CapturePaneContents: true, Max: 9, Icon: "X "}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestBuildSelfCommandQuotesArguments(t *testing.T) {
	got, err := buildSelfCommand("save-sessions", "-name", "space's here")
	if err != nil {
		t.Fatalf("buildSelfCommand: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty command")
	}
	if !strings.Contains(got, "'save-sessions'") {
		t.Fatalf("expected quoted subcommand, got %q", got)
	}
	if !strings.Contains(got, "'space'\\''s here'") {
		t.Fatalf("expected shell-quoted value, got %q", got)
	}
}

func TestCommandHandlersIncludeAutosaveNowOnly(t *testing.T) {
	handlers := commandHandlers()
	if _, ok := handlers["autosave-now"]; !ok {
		t.Fatal("expected autosave-now handler")
	}
	for _, gone := range []string{"autosave", "autosave-status"} {
		if _, ok := handlers[gone]; ok {
			t.Fatalf("the #() autosave daemon %q should be gone", gone)
		}
	}
}

func TestPopupArgsOpensModalFloatingPaneOnClientSession(t *testing.T) {
	style := tmux.PopupStyle{BorderLines: "rounded"}
	got := popupArgs("/dev/ttys001", style, "/bin/tpc --root-menu x")
	want := []string{
		"new-pane", "-t", "/dev/ttys001:", "-O", "-K", "-W",
		"-x", "50%", "-y", "50%", "-X", "25%", "-Y", "25%",
		"-T", "tmux-popup-control",
		"-e", "TMUX_POPUP_CONTROL_CLIENT=/dev/ttys001",
		"-B", "rounded",
		"/bin/tpc --root-menu x",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected args\n got: %q\nwant: %q", got, want)
	}
}

// TestShowPopupRejectsEmptyClient guards the popup launcher: the popup is
// targeted at "<client>:", which tmux resolves to that client's session. An
// empty name would instead resolve relative to our control-mode connection
// and open the popup somewhere the user isn't looking, so it is refused.
func TestShowPopupRejectsEmptyClient(t *testing.T) {
	err := showPopup("/nonexistent/tmux.sock", "  ")
	if err == nil || !strings.Contains(err.Error(), "client") {
		t.Fatalf("expected a client error, got %v", err)
	}
}
