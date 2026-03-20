package tmux

import (
	"fmt"
	"os"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// CreateSession creates a new tmux session with the given name and starting directory.
func CreateSession(socketPath, name, dir string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.NewSession(&gotmux.SessionOptions{Name: name, StartDirectory: dir})
	return err
}

// CreateWindow creates a new window at the given index within a session.
// The window is created detached (-d) to avoid switching focus.
func CreateWindow(socketPath, session string, index int, name, dir string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	target := fmt.Sprintf("%s:%d", session, index)
	_, err = client.Command("new-window", "-t", target, "-n", name, "-c", dir, "-d")
	return err
}

// SplitPane splits the pane at the given target, starting in dir.
// The new pane is created detached (-d) to avoid disturbing focus.
func SplitPane(socketPath, target, dir string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("split-window", "-t", target, "-c", dir, "-d")
	return err
}

// SelectLayoutTarget applies the named layout to the given target window.
// Unlike the existing SelectLayout, this takes an explicit target parameter.
func SelectLayoutTarget(socketPath, target, layout string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("select-layout", "-t", target, layout)
	return err
}

// SelectPane selects the given pane by target.
func SelectPane(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("select-pane", "-t", target)
	return err
}

// SendPaneContents writes contents to a temp file, loads it into the tmux
// buffer, pastes it into the target pane, and removes the temp file.
func SendPaneContents(socketPath, target, contents string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	f, err := os.CreateTemp("", "tmux-restore-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)

	if _, err := f.WriteString(contents); err != nil {
		f.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if _, err := client.Command("load-buffer", tmpPath); err != nil {
		return err
	}
	_, err = client.Command("paste-buffer", "-t", target, "-d")
	return err
}

// CapturePaneContents captures the full scrollback of the target pane,
// preserving trailing whitespace.
func CapturePaneContents(socketPath, target string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	return client.CapturePane(target, &gotmux.CaptureOptions{
		PreserveTrailing: true,
		StartLine:        "-",
	})
}

// ShowOption queries a tmux server option value. Returns an empty string if
// the option is not set or an error occurs.
func ShowOption(socketPath, option string) string {
	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}
	val, err := client.Command("show-option", "-gqv", option)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}
