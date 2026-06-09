package resurrect

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	envAutosaveIntervalMinutes = "TMUX_POPUP_CONTROL_AUTOSAVE_INTERVAL_MINUTES"
	envAutosaveMax             = "TMUX_POPUP_CONTROL_AUTOSAVE_MAX"
	envAutosaveIcon            = "TMUX_POPUP_CONTROL_AUTOSAVE_ICON"
	envAutosaveIconSeconds     = "TMUX_POPUP_CONTROL_AUTOSAVE_ICON_SECONDS"
	optAutosaveIntervalMinutes = "@tmux-popup-control-autosave-interval-minutes"
	optAutosaveMax             = "@tmux-popup-control-autosave-max"
	optAutosaveIcon            = "@tmux-popup-control-autosave-icon"
	optAutosaveIconSeconds     = "@tmux-popup-control-autosave-icon-seconds"
)

// resolveOption resolves a configuration value from, in order, an environment
// variable, a tmux server option, then a default. parse converts a raw string
// to a value and reports whether it should be accepted (false falls through to
// the next source). This collapses the otherwise-duplicated env→option→default
// chains used throughout the package.
func resolveOption[T any](socket, envKey, optKey string, parse func(string) (T, bool), def T) T {
	if v, ok := parse(os.Getenv(envKey)); ok {
		return v
	}
	if v, ok := parse(storageDeps.ShowOption(socket, optKey)); ok {
		return v
	}
	return def
}

func ResolveAutosaveIntervalMinutes(socketPath string) int {
	return resolveOption(socketPath, envAutosaveIntervalMinutes, optAutosaveIntervalMinutes, parsePositiveInt, 0)
}

func ResolveAutosaveMax(socketPath string) int {
	return resolveOption(socketPath, envAutosaveMax, optAutosaveMax, parseAutosaveMax, 5)
}

func ResolveAutosaveIconSeconds(socketPath string) int {
	return resolveOption(socketPath, envAutosaveIconSeconds, optAutosaveIconSeconds, parsePositiveInt, 0)
}

func ResolveAutosaveIcon(socketPath string) string {
	return resolveOption(socketPath, envAutosaveIcon, optAutosaveIcon, parseNonEmpty, defaultAutosaveStatusIcon)
}

// ResolvePaneContents reports whether pane content capture is enabled.
// Lookup chain:
//  1. TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS env var
//  2. @tmux-popup-control-restore-pane-contents tmux option
//  3. false (default)
func ResolvePaneContents(socketPath string) bool {
	return resolveOption(socketPath,
		"TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS",
		"@tmux-popup-control-restore-pane-contents",
		func(s string) (bool, bool) {
			if s == "" {
				return false, false
			}
			return parseBool(s), true
		},
		false,
	)
}

// parseNonEmpty accepts any non-empty string verbatim.
func parseNonEmpty(s string) (string, bool) {
	if s == "" {
		return "", false
	}
	return s, true
}

// parsePositiveInt accepts a trimmed, strictly-positive integer; anything else
// (blank, non-numeric, or <= 0) is rejected so the caller falls through.
func parsePositiveInt(value string) (int, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		// preserve original behaviour: a present-but-invalid value resolves
		// to 0 (the default) rather than falling through to the next source.
		return 0, true
	}
	return n, true
}

// parseAutosaveMax accepts a present value, clamping invalid or sub-1 values to
// 1; a blank value falls through to the default of 5.
func parseAutosaveMax(value string) (int, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 5, true
	}
	if n < 1 {
		return 1, true
	}
	return n, true
}

// parseBool returns true for common truthy values.
func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func AutoSaveName(ts time.Time) string {
	return "auto-" + ts.Format("2006-01-02T15-04-05")
}
