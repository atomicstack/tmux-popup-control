package main

import (
	"errors"
	"flag"
	"fmt"
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
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"golang.org/x/term"
)

// Version is set at build time via ldflags.
var Version = "dev"

var resolveSocketPathFn = tmux.ResolveSocketPath
var resolveSaveDirFn = resurrect.ResolveDir
var resolvePaneContentsFn = resurrect.ResolvePaneContents
var resolveAutosaveIntervalMinutesFn = resurrect.ResolveAutosaveIntervalMinutes
var resolveAutosaveMaxFn = resurrect.ResolveAutosaveMax
var resolveAutosaveIconFn = resurrect.ResolveAutosaveIcon
var resolveAutosaveIconSecondsFn = resurrect.ResolveAutosaveIconSeconds
var renderAutoSaveStatusFn = resurrect.AutoSaveStatus

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
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

	if subcommand(runtimeCfg) == "save-sessions" {
		if err := runSaveSessions(runtimeCfg); err != nil {
			fmt.Fprintf(os.Stderr, "save-sessions: %v\n", err)
			exitStatus = "error"
			exitErr = err
			return 1
		}
		return 0
	}

	if subcommand(runtimeCfg) == "restore-sessions" {
		if err := runRestoreSessions(runtimeCfg); err != nil {
			fmt.Fprintf(os.Stderr, "restore-sessions: %v\n", err)
			exitStatus = "error"
			exitErr = err
			return 1
		}
		return 0
	}

	if subcommand(runtimeCfg) == "autosave-status" {
		if err := runAutosaveStatus(runtimeCfg); err != nil {
			fmt.Fprintf(os.Stderr, "autosave-status: %v\n", err)
			exitStatus = "error"
			exitErr = err
			return 1
		}
		return 0
	}

	if subcommand(runtimeCfg) == "install-and-init-plugins" {
		if err := runInstallAndInitPlugins(runtimeCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exitStatus = "error"
			exitErr = err
			return 1
		}
		return 0
	}

	if subcommand(runtimeCfg) == "deferred-install" {
		if err := runDeferredInstall(runtimeCfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	cmd := fmt.Sprintf("%s deferred-install -socket %s",
		shellQuote(binary), shellQuote(socketPath))
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

	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	popupCmd := fmt.Sprintf("%s --root-menu plugins:install -socket %s",
		shellQuote(binary), shellQuote(socketPath))
	_, err = tmux.RunCommand(socketPath, "display-popup", "-c", clientName, "-E", popupCmd)
	return err
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	popupCmd := fmt.Sprintf("%s save-sessions --resurrect-popup -socket %s",
		shellQuote(binary), shellQuote(socketPath))
	if *name != "" {
		popupCmd += fmt.Sprintf(" -name %s", shellQuote(*name))
	}
	_, err = tmux.RunCommand(socketPath, "display-popup", "-c", clientName, "-E", popupCmd)
	return err
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
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable: %w", err)
	}
	popupCmd := fmt.Sprintf("%s restore-sessions --resurrect-popup -socket %s",
		shellQuote(binary), shellQuote(socketPath))
	if *from != "" {
		popupCmd += fmt.Sprintf(" -from %s", shellQuote(*from))
	}
	_, err = tmux.RunCommand(socketPath, "display-popup", "-c", clientName, "-E", popupCmd)
	return err
}

func runAutosaveStatus(cfg config.Config) error {
	output, err := autosaveStatusOutput(cfg)
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}

func autosaveStatusOutput(cfg config.Config) (string, error) {
	fs := flag.NewFlagSet("autosave-status", flag.ContinueOnError)
	socket := fs.String("socket", cfg.App.SocketPath, "tmux socket path")
	if err := fs.Parse(subcommandArgs(cfg)); err != nil {
		return "", err
	}

	socketPath, err := resolveSocketPathFn(*socket)
	if err != nil {
		return "", fmt.Errorf("resolving socket: %w", err)
	}
	saveDir, err := resolveSaveDirFn(socketPath)
	if err != nil {
		return "", fmt.Errorf("resolving save dir: %w", err)
	}
	return renderAutoSaveStatusFn(resurrect.StatusConfig{
		SocketPath:          socketPath,
		SaveDir:             saveDir,
		CapturePaneContents: resolvePaneContentsFn(socketPath),
		IntervalMinutes:     resolveAutosaveIntervalMinutesFn(socketPath),
		Max:                 resolveAutosaveMaxFn(socketPath),
		Icon:                resolveAutosaveIconFn(socketPath),
		IconSeconds:         resolveAutosaveIconSecondsFn(socketPath),
	})
}
