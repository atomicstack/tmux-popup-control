package tmux

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// CreateSession creates a new tmux session with the given name, starting
// directory, and optional startup command (for pane content restore).
func CreateSession(socketPath, name, dir, command string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.NewSession(&gotmux.SessionOptions{
		Name:           name,
		StartDirectory: dir,
		ShellCommand:   command,
	})
	return err
}

// CreateWindow creates a new window at the given index within a session.
// The window is created detached (-d) to avoid switching focus.
// An optional startup command is appended for pane content restore.
func CreateWindow(socketPath, session string, index int, name, dir, command string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	target := fmt.Sprintf("%s:%d", session, index)
	args := []string{"new-window", "-t", target, "-n", name, "-c", dir, "-d"}
	if command != "" {
		args = append(args, command)
	}
	_, err = client.Command(args...)
	return err
}

// SplitPane splits the pane at the given target, starting in dir.
// The new pane is created detached to avoid disturbing focus.
// An optional startup command is appended for pane content restore.
func SplitPane(socketPath, target, dir, command string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	return client.SplitWindow(target, &gotmux.SplitWindowOptions{
		StartDirectory: dir,
		Detached:       true,
		ShellCommand:   command,
	})
}

// SelectLayoutTarget applies the named layout to the given target window.
func SelectLayoutTarget(socketPath, target, layout string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	return client.SelectLayout(target, layout)
}

// SelectPane selects the given pane by target.
func SelectPane(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	return client.SelectPane(target)
}

// WindowIndices returns the set of occupied window indices for the given session.
func WindowIndices(socketPath, sessionName string) (map[int]bool, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	out, err := client.Command("list-windows", "-t", sessionName, "-F", "#{window_index}")
	if err != nil {
		return nil, err
	}
	indices := make(map[int]bool)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		indices[idx] = true
	}
	return indices, nil
}

// CapturePaneContents captures the full scrollback of the target pane,
// preserving trailing whitespace and ANSI escape sequences (colours, bold, etc.).
func CapturePaneContents(socketPath, target string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	return client.CapturePane(target, &gotmux.CaptureOptions{
		EscTxtNBgAttr:    true,
		PreserveTrailing: true,
		StartLine:        "-",
	})
}

// SessionOption queries a session-level option. Returns empty string if unset.
// Uses display-message with format syntax rather than show-option, because
// show-option -qv doesn't reliably return values through control mode.
func SessionOption(socketPath, session, option string) string {
	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}
	format := "#{" + option + "}"
	out, err := client.Command("display-message", "-t", session+":", "-p", format)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// SetSessionOption sets a session-level option.
func SetSessionOption(socketPath, session, option, value string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("set-option", "-t", session, option, value)
	return err
}

// ShowOption queries a tmux server-level (global) option value. Returns an
// empty string if the option is not set or an error occurs.
func ShowOption(socketPath, option string) string {
	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}
	val, err := client.GlobalOption(option)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

// DefaultCommand returns the tmux default-command setting. If unset or empty
// it falls back to the SHELL environment variable, then /bin/sh.
func DefaultCommand(socketPath string) string {
	if cmd := ShowOption(socketPath, "default-command"); cmd != "" {
		return cmd
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/sh"
}
