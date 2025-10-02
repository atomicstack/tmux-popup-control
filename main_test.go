package main

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/app"
	"github.com/atomicstack/tmux-popup-control/internal/config"
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
			FilePath: "trace.log",
			Trace:    true,
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

	if _, ok := payload["tty"].(ttyDetails); !ok {
		t.Fatalf("expected tty details in payload")
	}
	if cfgValue, ok := payload["config"].(config.Config); !ok {
		t.Fatalf("expected config in payload")
	} else if cfgValue.App != cfg.App {
		t.Fatalf("expected app config %#v, got %#v", cfg.App, cfgValue.App)
	}
}
