package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/app"
	"github.com/atomicstack/tmux-popup-control/internal/config"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
	"github.com/atomicstack/tmux-popup-control/internal/shquote"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"golang.org/x/term"
)

// Version is set at build time via ldflags.
var Version = "dev"

type MainDeps struct {
	ResolveSocketPath              func(string) (string, error)
	ResolveSaveDir                 func(string) (string, error)
	ResolvePaneContents            func(string) bool
	ResolveAutosaveIntervalMinutes func(string) int
	ResolveAutosaveMax             func(string) int
	ResolveAutosaveIcon            func(string) string
	ResolveAutosaveIconSeconds     func(string) int
	RunAutoSaveCommand             func(resurrect.StatusConfig, io.Writer) error
}

var mainDeps = MainDeps{
	ResolveSocketPath:              tmux.ResolveSocketPath,
	ResolveSaveDir:                 resurrect.ResolveDir,
	ResolvePaneContents:            resurrect.ResolvePaneContents,
	ResolveAutosaveIntervalMinutes: resurrect.ResolveAutosaveIntervalMinutes,
	ResolveAutosaveMax:             resurrect.ResolveAutosaveMax,
	ResolveAutosaveIcon:            resurrect.ResolveAutosaveIcon,
	ResolveAutosaveIconSeconds:     resurrect.ResolveAutosaveIconSeconds,
	RunAutoSaveCommand:             resurrect.RunAutoSaveCommand,
}

type commandHandler struct {
	ErrorLabel string
	Run        func(config.Config, MainDeps) error
}

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	return runWithDeps(mainDeps)
}

func runWithDeps(deps MainDeps) (exitCode int) {
	ensureZeroExitOnHangup()
	var exitStatus = "ok"
	var exitErr error
	defer func() {
		logging.Close(logging.RunResult{
			ExitCode:   exitCode,
			ExitStatus: exitStatus,
			Error:      exitErr,
		})
	}()

	runtimeCfg, err := config.Load()
	if errors.Is(err, config.ErrVersionRequested) {
		fmt.Println(Version)
		return 0
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		exitStatus = "config_error"
		exitErr = err
		return 2
	}
	if err := config.Validate(runtimeCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		exitStatus = "config_error"
		exitErr = err
		return 2
	}
	logging.Configure(runtimeCfg.Logging.FilePath)
	logging.SetTraceEnabled(runtimeCfg.Logging.Trace)
	if runtimeCfg.Logging.DebugToSQLite {
		if err := logging.EnableSQLiteDebug(buildSQLiteRunInfo(runtimeCfg)); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
			exitStatus = "sqlite_debug_error"
			exitErr = err
			return 2
		}
	}

	traceStartup(runtimeCfg)

	cmd := subcommand(runtimeCfg)
	if handler, ok := commandHandlers()[cmd]; ok {
		if err := handler.Run(runtimeCfg, deps); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", handler.ErrorLabel, err)
			exitStatus = "error"
			exitErr = err
			return 1
		}
		return 0
	}

	if err := app.Run(runtimeCfg.App); err != nil {
		logging.Error(err)
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exitStatus = "error"
		exitErr = err
		return 1
	}
	return 0
}

func commandHandlers() map[string]commandHandler {
	return map[string]commandHandler{
		"save-sessions": {
			ErrorLabel: "save-sessions",
			Run: func(cfg config.Config, _ MainDeps) error {
				return runSaveSessions(cfg)
			},
		},
		"restore-sessions": {
			ErrorLabel: "restore-sessions",
			Run: func(cfg config.Config, _ MainDeps) error {
				return runRestoreSessions(cfg)
			},
		},
		"autosave": {
			ErrorLabel: "autosave",
			Run:        runAutosave,
		},
		"autosave-status": {
			ErrorLabel: "autosave",
			Run:        runAutosave,
		},
		"install-and-init-plugins": {
			ErrorLabel: "Error",
			Run: func(cfg config.Config, _ MainDeps) error {
				return runInstallAndInitPlugins(cfg)
			},
		},
		"deferred-install": {
			ErrorLabel: "Error",
			Run: func(cfg config.Config, _ MainDeps) error {
				return runDeferredInstall(cfg)
			},
		},
	}
}

func runInstallAndInitPlugins(cfg config.Config) error {
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

	// Source already-installed plugins immediately.
	if err := plugin.Source(pluginDir, plugins); err != nil {
		return fmt.Errorf("sourcing plugins: %w", err)
	}

	// If any plugins need installing, schedule a deferred popup so the user
	// sees the interactive install UI once tmux finishes starting.
	for _, p := range plugins {
		if !p.Installed {
			return deferPluginInstall(socketPath)
		}
	}
	return nil
}

// deferPluginInstall schedules a background tmux command that waits for
// startup to complete, then opens the install TUI in a display-popup.
// Uses a plain CLI call rather than gotmuxcc control-mode to avoid creating
// phantom sessions during server startup.
func deferPluginInstall(socketPath string) error {
	cmd, err := buildSelfCommand("deferred-install", "-socket", socketPath)
	if err != nil {
		return err
	}
	args := []string{"run-shell", "-b", cmd}
	if socketPath != "" {
		args = append([]string{"-S", socketPath}, args...)
	}
	return exec.Command("tmux", args...).Run()
}

// runDeferredInstall is invoked via run-shell -b after the main
// install-and-init-plugins command completes. It waits for tmux to be
// fully ready, then opens the install TUI in a display-popup.
func runDeferredInstall(cfg config.Config) error {
	socketPath, err := tmux.ResolveSocketPath(cfg.App.SocketPath)
	if err != nil {
		return fmt.Errorf("resolving socket: %w", err)
	}

	plugins, err := plugin.ParseConfig(socketPath)
	if err != nil {
		return fmt.Errorf("reading plugin config: %w", err)
	}

	var needInstall bool
	for _, p := range plugins {
		if !p.Installed {
			needInstall = true
			break
		}
	}
	if !needInstall {
		return nil
	}

	// Wait for tmux to finish starting so display-popup has a client.
	time.Sleep(500 * time.Millisecond)

	// Find the real terminal client — display-popup must target it rather
	// than the control-mode connection gotmuxcc uses.
	clientName, err := tmux.FindTerminalClient(socketPath)
	if err != nil {
		return fmt.Errorf("finding terminal client: %w", err)
	}

	return showPopup(socketPath, clientName, "--root-menu", "plugins:install", "-socket", socketPath)
}

func ensureZeroExitOnHangup() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP)
	go func() {
		for range ch {
			logging.Close(logging.RunResult{
				ExitCode:   0,
				ExitStatus: "sighup",
			})
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
	flags["debugToSQLite"] = cfg.Logging.DebugToSQLite
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

func buildSQLiteRunInfo(cfg config.Config) logging.SQLiteRunInfo {
	info := logging.SQLiteRunInfo{
		Version:     Version,
		Args:        append([]string(nil), cfg.Args...),
		Flags:       cloneStringMap(cfg.Flags),
		SocketPath:  cfg.App.SocketPath,
		RootMenu:    cfg.App.RootMenu,
		MenuArgs:    cfg.App.MenuArgs,
		ClientID:    cfg.App.ClientID,
		SessionName: cfg.App.SessionName,
	}
	if exe, err := os.Executable(); err == nil {
		info.ExecutablePath = exe
	}
	if cwd, err := os.Getwd(); err == nil {
		info.CWD = cwd
	}
	return info
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func subcommand(cfg config.Config) string {
	if len(cfg.Command) == 0 {
		return ""
	}
	return strings.TrimSpace(cfg.Command[0])
}

func subcommandArgs(cfg config.Config) []string {
	if len(cfg.Command) <= 1 {
		return nil
	}
	return append([]string(nil), cfg.Command[1:]...)
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

// runSaveSessions handles the "save-sessions" subcommand.
// Without --resurrect-popup it launches a display-popup; with it, it enters
// the progress UI directly.
func runSaveSessions(cfg config.Config) error {
	fs := flag.NewFlagSet("save-sessions", flag.ContinueOnError)
	name := fs.String("name", os.Getenv("TMUX_POPUP_CONTROL_RESURRECT_NAME"), "snapshot name")
	popup := fs.Bool("resurrect-popup", false, "run inside popup (internal)")
	socket := fs.String("socket", cfg.App.SocketPath, "tmux socket path")
	if err := fs.Parse(subcommandArgs(cfg)); err != nil {
		return err
	}

	if *popup {
		cfg.App.SocketPath = *socket
		cfg.App.ResurrectOp = "save"
		cfg.App.ResurrectName = *name
		return app.Run(cfg.App)
	}

	// outer invocation: launch popup
	socketPath, err := tmux.ResolveSocketPath(*socket)
	if err != nil {
		return fmt.Errorf("resolving socket: %w", err)
	}
	clientName, err := tmux.FindTerminalClient(socketPath)
	if err != nil {
		return fmt.Errorf("finding terminal client: %w", err)
	}
	args := []string{"save-sessions", "--resurrect-popup", "-socket", socketPath}
	if *name != "" {
		args = append(args, "-name", *name)
	}
	return showPopup(socketPath, clientName, args...)
}

// runRestoreSessions handles the "restore-sessions" subcommand.
func runRestoreSessions(cfg config.Config) error {
	fs := flag.NewFlagSet("restore-sessions", flag.ContinueOnError)
	from := fs.String("from", os.Getenv("TMUX_POPUP_CONTROL_RESURRECT_FROM"), "save file name or path")
	popup := fs.Bool("resurrect-popup", false, "run inside popup (internal)")
	socket := fs.String("socket", cfg.App.SocketPath, "tmux socket path")
	if err := fs.Parse(subcommandArgs(cfg)); err != nil {
		return err
	}

	if *popup {
		cfg.App.SocketPath = *socket
		cfg.App.ResurrectOp = "restore"
		cfg.App.ResurrectFrom = *from
		return app.Run(cfg.App)
	}

	// outer invocation: launch popup
	socketPath, err := tmux.ResolveSocketPath(*socket)
	if err != nil {
		return fmt.Errorf("resolving socket: %w", err)
	}
	clientName, err := tmux.FindTerminalClient(socketPath)
	if err != nil {
		return fmt.Errorf("finding terminal client: %w", err)
	}
	args := []string{"restore-sessions", "--resurrect-popup", "-socket", socketPath}
	if *from != "" {
		args = append(args, "-from", *from)
	}
	return showPopup(socketPath, clientName, args...)
}

func runAutosave(cfg config.Config, deps MainDeps) error {
	autoSaveCfg, err := buildAutoSaveConfig(cfg, deps)
	if err != nil {
		return err
	}
	return deps.RunAutoSaveCommand(autoSaveCfg, os.Stdout)
}

func autoSaveOutput(cfg config.Config) (string, error) {
	return autoSaveOutputWithDeps(cfg, mainDeps)
}

func autoSaveOutputWithDeps(cfg config.Config, deps MainDeps) (string, error) {
	autoSaveCfg, err := buildAutoSaveConfig(cfg, deps)
	if err != nil {
		return "", err
	}
	var output strings.Builder
	if err := deps.RunAutoSaveCommand(autoSaveCfg, &output); err != nil {
		return "", err
	}
	return output.String(), nil
}

func buildAutoSaveConfig(cfg config.Config, deps MainDeps) (resurrect.StatusConfig, error) {
	socketPath := cfg.App.SocketPath
	if parsed := autoSaveSocketFlag(cfg); parsed != "" {
		socketPath = parsed
	}
	resolvedSocketPath, err := deps.ResolveSocketPath(socketPath)
	if err != nil {
		return resurrect.StatusConfig{}, fmt.Errorf("resolving socket: %w", err)
	}
	saveDir, err := deps.ResolveSaveDir(resolvedSocketPath)
	if err != nil {
		return resurrect.StatusConfig{}, fmt.Errorf("resolving save dir: %w", err)
	}
	return resurrect.StatusConfig{
		SocketPath:          resolvedSocketPath,
		SaveDir:             saveDir,
		CapturePaneContents: deps.ResolvePaneContents(resolvedSocketPath),
		IntervalMinutes:     deps.ResolveAutosaveIntervalMinutes(resolvedSocketPath),
		Max:                 deps.ResolveAutosaveMax(resolvedSocketPath),
		Icon:                deps.ResolveAutosaveIcon(resolvedSocketPath),
		IconSeconds:         deps.ResolveAutosaveIconSeconds(resolvedSocketPath),
	}, nil
}

func autoSaveSocketFlag(cfg config.Config) string {
	fs := flag.NewFlagSet("autosave", flag.ContinueOnError)
	socket := fs.String("socket", cfg.App.SocketPath, "tmux socket path")
	if err := fs.Parse(subcommandArgs(cfg)); err != nil {
		return cfg.App.SocketPath
	}
	return *socket
}

func buildSelfCommand(args ...string) (string, error) {
	binary, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolving executable: %w", err)
	}
	return shquote.JoinCommand(append([]string{binary}, args...)...), nil
}

func showPopup(socketPath, clientName string, args ...string) error {
	popupCmd, err := buildSelfCommand(args...)
	if err != nil {
		return err
	}
	_, err = tmux.RunCommand(socketPath, "display-popup", "-c", clientName, "-E", popupCmd)
	return err
}
