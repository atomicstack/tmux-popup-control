package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/app"
)

// Config captures runtime configuration for the application.
type Config struct {
	App      app.Config
	Logging  Logging
	Features Features
	Flags    map[string]string
	Args     []string
}

type Logging struct {
	FilePath string
	Trace    bool
}

type Features struct {
	Verbose bool
}

const (
	envSocketPath = "TMUX_POPUP_CONTROL_SOCKET"
	envWidth      = "TMUX_POPUP_CONTROL_WIDTH"
	envHeight     = "TMUX_POPUP_CONTROL_HEIGHT"
	envShowFooter = "TMUX_POPUP_CONTROL_FOOTER"
	envVerbose    = "TMUX_POPUP_CONTROL_VERBOSE"
	envTrace      = "TMUX_POPUP_CONTROL_TRACE"
	envLogFile    = "TMUX_POPUP_CONTROL_LOG_FILE"
)

// Load parses configuration from CLI arguments and environment variables.
func Load() (Config, error) {
	return LoadArgs(os.Args[1:], os.Environ())
}

// LoadArgs allows tests to supply specific args/environment.
func LoadArgs(args []string, environ []string) (Config, error) {
	env := parseEnv(environ)

	fs := flag.NewFlagSet("tmux-popup-control", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder))

	socket := fs.String("socket", envOrDefault(env, envSocketPath, ""), "path to the tmux socket (overrides environment detection)")
	width := fs.Int("width", envOrInt(env, envWidth, 0), "desired viewport width in cells (0 uses terminal width)")
	height := fs.Int("height", envOrInt(env, envHeight, 0), "desired viewport height in rows (0 uses terminal height)")
	footer := fs.Bool("footer", envOrBool(env, envShowFooter, false), "enable footer hint row (disabled by default)")
	trace := fs.Bool("trace", envOrBool(env, envTrace, false), "enable verbose JSON trace logging")
	verbose := fs.Bool("verbose", envOrBool(env, envVerbose, false), "print success messages for actions")
	logFile := fs.String("log-file", envOrDefault(env, envLogFile, ""), "path to the log file")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if *width < 0 {
		return Config{}, fmt.Errorf("width must be >= 0 (got %d)", *width)
	}
	if *height < 0 {
		return Config{}, fmt.Errorf("height must be >= 0 (got %d)", *height)
	}

	cfg := Config{
		App: app.Config{
			SocketPath: *socket,
			Width:      *width,
			Height:     *height,
			ShowFooter: *footer,
			Verbose:    *verbose,
		},
		Logging: Logging{
			FilePath: *logFile,
			Trace:    *trace,
		},
		Features: Features{
			Verbose: *verbose,
		},
		Flags: map[string]string{
			"socket":  *socket,
			"width":   strconv.Itoa(*width),
			"height":  strconv.Itoa(*height),
			"footer":  strconv.FormatBool(*footer),
			"trace":   strconv.FormatBool(*trace),
			"verbose": strconv.FormatBool(*verbose),
			"logFile": *logFile,
		},
		Args: append([]string(nil), args...),
	}

	return cfg, nil
}

func parseEnv(environ []string) map[string]string {
	values := make(map[string]string, len(environ))
	for _, entry := range environ {
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[parts[0]] = parts[1]
	}
	return values
}

func envOrDefault(env map[string]string, key, fallback string) string {
	if v, ok := env[key]; ok {
		return v
	}
	return fallback
}

func envOrInt(env map[string]string, key string, fallback int) int {
	v, ok := env[key]
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func envOrBool(env map[string]string, key string, fallback bool) bool {
	v, ok := env[key]
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

// MustLoad returns configuration or exits.
func MustLoad() Config {
	cfg, err := Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(2)
	}
	return cfg
}

// Validate ensures required minimum configuration is present.
func Validate(cfg Config) error {
	// Additional validation hooks can be placed here as configuration evolves.
	return nil
}
