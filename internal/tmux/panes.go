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

// JoinPane joins source into target's window as a split (tmux `join-pane`).
// This is the correct command for the pane:join action: tmux's `move-pane`
// (see MovePane) was repurposed in tmux next-3.7 to reposition *floating*
// panes and rejects ordinary panes with "pane is not floating".
func JoinPane(socketPath, source, target string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("pane source required")
	}
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("pane target required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	return client.JoinPane(source, target)
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

// ZoomWindow zooms the active pane of the current window via control-mode
// (`resize-pane -Z`). Used to restore a zoom that select-layout undid.
func ZoomWindow(socketPath string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("resize-pane", "-Z")
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

// SwitchPane moves the user's client to the pane identified by ref: it
// switches the client to the pane's session, selects the window and then the
// pane. Every target is a tmux id, never a name, because session and window
// names may contain ':' and '.' on tmux next-3.8. When the session id is not
// known the pane id doubles as the switch-client target (tmux resolves a %N
// target to the pane's session).
func SwitchPane(socketPath, clientID string, ref PaneRef) error {
	paneID := strings.TrimSpace(ref.PaneID)
	if paneID == "" {
		return fmt.Errorf("pane id required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	sessionTarget := strings.TrimSpace(ref.SessionID)
	if sessionTarget == "" {
		sessionTarget = paneID
	}
	switchOpts := &gotmux.SwitchClientOptions{TargetSession: sessionTarget}
	if id := strings.TrimSpace(clientID); isValidClientName(id) {
		switchOpts.TargetClient = id
	}
	if err := client.SwitchClient(switchOpts); err != nil {
		return err
	}
	if windowID := strings.TrimSpace(ref.WindowID); windowID != "" {
		if err := client.SelectWindow(windowID); err != nil {
			return err
		}
	}
	return client.SelectPane(paneID)
}
