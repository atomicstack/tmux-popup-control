package tmux

import (
	"fmt"
	"strconv"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func RenamePane(socketPath, target, newTitle string) error {
	trimmedTarget := strings.TrimSpace(target)
	if trimmedTarget == "" {
		return fmt.Errorf("pane target required")
	}
	trimmedTitle := strings.TrimSpace(newTitle)
	if trimmedTitle == "" {
		return fmt.Errorf("pane title required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	return client.RenamePane(trimmedTarget, trimmedTitle)
}

func KillPanes(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	for _, target := range targets {
		t := strings.TrimSpace(target)
		if t == "" {
			continue
		}
		if _, err := client.Command("kill-pane", "-t", t); err != nil {
			return err
		}
	}
	return nil
}

func SwapPanes(socketPath, first, second string) error {
	if strings.TrimSpace(first) == "" || strings.TrimSpace(second) == "" {
		return fmt.Errorf("pane ids required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	return client.SwapPanes(first, second)
}

func MovePane(socketPath, source, target string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("pane source required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	return client.MovePane(source, target)
}

func BreakPane(socketPath, source, destination string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("pane source required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	return client.BreakPane(source, destination)
}

// SelectLayout applies a layout to the current window via control-mode.
// No explicit window target is used; tmux applies the layout to whatever
// window is currently active for the control-mode session.
func SelectLayout(socketPath, layout string) error {
	if strings.TrimSpace(layout) == "" {
		return fmt.Errorf("layout required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	_, err = client.Command("select-layout", layout)
	return err
}

// ResizePane resizes the current pane via control-mode.
// No explicit pane target is used; tmux applies the resize to the
// currently active pane.
func ResizePane(socketPath, direction string, amount int) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	flag := ""
	switch direction {
	case "left":
		flag = "-L"
	case "right":
		flag = "-R"
	case "up":
		flag = "-U"
	case "down":
		flag = "-D"
	default:
		return fmt.Errorf("unknown direction %q", direction)
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	_, err = client.Command("resize-pane", flag, strconv.Itoa(amount))
	return err
}

func SwitchPane(socketPath, clientID, target string) error {
	parts := strings.SplitN(target, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid pane target %q", target)
	}
	session := parts[0]
	windowPart := parts[1]
	windowParts := strings.SplitN(windowPart, ".", 2)
	if len(windowParts) != 2 {
		return fmt.Errorf("invalid pane target %q", target)
	}
	window := fmt.Sprintf("%s:%s", session, windowParts[0])
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	switchOpts := &gotmux.SwitchClientOptions{TargetSession: session}
	if id := strings.TrimSpace(clientID); isValidClientName(id) {
		switchOpts.TargetClient = id
	}
	if err := client.SwitchClient(switchOpts); err != nil {
		return err
	}
	if err := client.SelectWindow(window); err != nil {
		return err
	}
	return client.SelectPane(target)
}
