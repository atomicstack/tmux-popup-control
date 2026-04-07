package main

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/app"
	"github.com/atomicstack/tmux-popup-control/internal/config"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
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

	flagsValue, ok := payload["flags"].(map[string]interface{})
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

func TestAutoSaveOutputUsesResolvedSettings(t *testing.T) {
	var gotCfg resurrect.StatusConfig
	deps := MainDeps{
		ResolveSaveDir: func(string) (string, error) { return "/tmp/saves", nil },
		ResolvePaneContents: func(string) bool { return true },
		ResolveSocketPath: func(string) (string, error) { return "/tmp/tmux.sock", nil },
		ResolveAutosaveIntervalMinutes: func(string) int { return 7 },
		ResolveAutosaveMax: func(string) int { return 9 },
		ResolveAutosaveIcon: func(string) string { return "X " },
		ResolveAutosaveIconSeconds: func(string) int { return 5 },
		RunAutoSaveCommand: func(cfg resurrect.StatusConfig, outputWriter io.Writer) error {
		gotCfg = cfg
		_, err := outputWriter.Write([]byte("X \n"))
		return err
		},
	}

	output, err := autoSaveOutputWithDeps(config.Config{
		App: app.Config{SocketPath: "app-socket"},
		Command: []string{
			"autosave",
			"-socket", "flag-socket",
		},
	}, deps)
	if err != nil {
		t.Fatalf("autoSaveOutput: %v", err)
	}

	if output != "X \n" {
		t.Fatalf("expected autosave output %q, got %q", "X \n", output)
	}
	if gotCfg.SocketPath != "/tmp/tmux.sock" {
		t.Fatalf("expected socket path %q, got %q", "/tmp/tmux.sock", gotCfg.SocketPath)
	}
	if gotCfg.SaveDir != "/tmp/saves" {
		t.Fatalf("expected save dir %q, got %q", "/tmp/saves", gotCfg.SaveDir)
	}
	if !gotCfg.CapturePaneContents {
		t.Fatal("expected pane contents to be enabled")
	}
	if gotCfg.IntervalMinutes != 7 {
		t.Fatalf("expected autosave interval 7, got %d", gotCfg.IntervalMinutes)
	}
	if gotCfg.Max != 9 {
		t.Fatalf("expected autosave max 9, got %d", gotCfg.Max)
	}
	if gotCfg.Icon != "X " {
		t.Fatalf("expected autosave icon %q, got %q", "X ", gotCfg.Icon)
	}
	if gotCfg.IconSeconds != 5 {
		t.Fatalf("expected autosave icon duration 5, got %d", gotCfg.IconSeconds)
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

func TestCommandHandlersIncludesAutosaveAlias(t *testing.T) {
	handlers := commandHandlers()
	autosave, ok := handlers["autosave"]
	if !ok {
		t.Fatal("expected autosave handler")
	}
	autosaveStatus, ok := handlers["autosave-status"]
	if !ok {
		t.Fatal("expected autosave-status handler")
	}
	if autosave.ErrorLabel != "autosave" || autosaveStatus.ErrorLabel != "autosave" {
		t.Fatalf("expected autosave labels, got %q and %q", autosave.ErrorLabel, autosaveStatus.ErrorLabel)
	}
}
