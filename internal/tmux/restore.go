package tmux

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

type SessionSpec struct {
	SocketPath string
	Name       string
	Dir        string
	Command    string
}

type WindowSpec struct {
	SocketPath string
	Session    string
	Index      int
	Name       string
	Dir        string
	Command    string
}

type PaneSpec struct {
	SocketPath string
	Target     string
	Dir        string
	Command    string
}

// CreateSession creates a new tmux session with the given name, starting
// directory, and optional startup command (for pane content restore).
func CreateSession(spec SessionSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	args := []string{"new-session", "-d", "-s", spec.Name}
	if spec.Dir != "" {
		args = append(args, "-c", spec.Dir)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
}

// CreateWindow creates a new window at the given index within a session.
// The window is created detached (-d) to avoid switching focus.
// An optional startup command is appended for pane content restore.
func CreateWindow(spec WindowSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	target := fmt.Sprintf("%s:%d", spec.Session, spec.Index)
	args := []string{"new-window", "-t", target, "-n", spec.Name, "-c", spec.Dir, "-d"}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
}

// SplitPane splits the pane at the given target, starting in dir.
// The new pane is created detached to avoid disturbing focus.
// An optional startup command is appended for pane content restore.
func SplitPane(spec PaneSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	args := []string{"split-window", "-d", "-t", spec.Target}
	if spec.Dir != "" {
		args = append(args, "-c", spec.Dir)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
}

// RespawnPane kills the running command in the target pane and restarts it in
// the given directory with an optional command. This is used during restore to
// set the first pane's working directory without polluting session_path.
func RespawnPane(spec PaneSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	args := []string{"respawn-pane", "-k", "-t", spec.Target}
	if spec.Dir != "" {
		args = append(args, "-c", spec.Dir)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
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
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
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
