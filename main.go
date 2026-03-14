package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/atomicstack/tmux-popup-control/internal/app"
	"github.com/atomicstack/tmux-popup-control/internal/config"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"golang.org/x/term"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println(Version)
		os.Exit(0)
	}
	ensureZeroExitOnHangup()
	runtimeCfg := config.MustLoad()
	if err := config.Validate(runtimeCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(2)
	}
	logging.Configure(runtimeCfg.Logging.FilePath)
	logging.SetTraceEnabled(runtimeCfg.Logging.Trace)

	traceStartup(runtimeCfg)

	if len(os.Args) > 1 && os.Args[1] == "init-plugins" {
		if err := runInitPlugins(runtimeCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if err := app.Run(runtimeCfg.App); err != nil {
		logging.Error(err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runInitPlugins(cfg config.Config) error {
	socketPath, err := tmux.ResolveSocketPath(cfg.App.SocketPath)
	if err != nil {
		return fmt.Errorf("resolving socket: %w", err)
	}

	plugins, err := plugin.ParseConfig(socketPath)
	if err != nil {
		return fmt.Errorf("reading plugin config: %w", err)
	}

	pluginDir := plugin.PluginDir()
	events.Plugins.InitPlugins(len(plugins))

	if err := plugin.Source(pluginDir, plugins); err != nil {
		return fmt.Errorf("sourcing plugins: %w", err)
	}
	return nil
}

func ensureZeroExitOnHangup() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			os.Exit(0)
		}
	}()
}

func traceStartup(cfg config.Config) {
	events.App.Start(startupTracePayload(cfg))
}

// startupTracePayload bundles runtime context for trace logging.
func startupTracePayload(cfg config.Config) map[string]interface{} {
	flags := make(map[string]interface{}, len(cfg.Flags))
	for k, v := range cfg.Flags {
		flags[k] = v
	}
	flags["trace"] = cfg.Logging.Trace
	flags["logFile"] = cfg.Logging.FilePath
	payload := map[string]interface{}{
		"argv":   cfg.Args,
		"flags":  flags,
		"config": cfg,
	}
	if exe, err := os.Executable(); err == nil {
		payload["executable"] = exe
	} else {
		payload["executableError"] = err.Error()
	}
	if cwd, err := os.Getwd(); err == nil {
		payload["cwd"] = cwd
	} else {
		payload["cwdError"] = err.Error()
	}
	payload["tty"] = collectTTYDetails()
	return payload
}

type ttyDetails struct {
	Detected *ttyDetected     `json:"detected,omitempty"`
	Probes   []ttyProbeResult `json:"probes"`
}

type ttyDetected struct {
	Source string `json:"source"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type ttyProbeResult struct {
	Name       string `json:"name"`
	IsTerminal bool   `json:"is_terminal"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Error      string `json:"error,omitempty"`
}

// collectTTYDetails inspects standard descriptors for terminal support and dimensions.
func collectTTYDetails() ttyDetails {
	probes := []struct {
		name string
		fd   uintptr
	}{
		{"stdin", os.Stdin.Fd()},
		{"stdout", os.Stdout.Fd()},
		{"stderr", os.Stderr.Fd()},
	}
	results := make([]ttyProbeResult, 0, len(probes))
	var detected *ttyDetected
	for _, probe := range probes {
		entry := ttyProbeResult{Name: probe.name}
		fd := int(probe.fd)
		if fd >= 0 && term.IsTerminal(fd) {
			entry.IsTerminal = true
			if width, height, err := term.GetSize(fd); err == nil {
				entry.Width = width
				entry.Height = height
				if detected == nil {
					detected = &ttyDetected{Source: probe.name, Width: width, Height: height}
				}
			} else {
				entry.Error = err.Error()
			}
		} else {
			entry.IsTerminal = false
		}
		results = append(results, entry)
	}
	return ttyDetails{Detected: detected, Probes: results}
}
