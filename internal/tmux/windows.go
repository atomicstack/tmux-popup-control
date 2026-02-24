package tmux

import (
	"fmt"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func SwitchClient(socketPath, clientID, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	opts := &gotmux.SwitchClientOptions{TargetSession: target}
	if strings.TrimSpace(clientID) != "" {
		opts.TargetClient = clientID
	}
	return client.SwitchClient(opts)
}

func SelectWindow(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	return client.SelectWindow(target)
}

func KillWindow(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	_, err = client.Command("kill-window", "-t", strings.TrimSpace(target))
	return err
}

func UnlinkWindows(socketPath string, targets []string) error {
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
		if err := client.UnlinkWindow(t); err != nil {
			return err
		}
	}
	return nil
}

func RenameWindow(socketPath, target, newName string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	window, err := findWindow(client, target)
	if err != nil {
		return err
	}
	if window == nil {
		// Fall back to renaming by target string via control-mode.
		_, err = client.Command("rename-window", "-t", target, newName)
		return err
	}
	return window.Rename(newName)
}

func LinkWindow(socketPath, source, targetSession string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	if err := client.LinkWindow(source, targetSession); err != nil {
		return fmt.Errorf("failed to link window %s to %s: %w", source, targetSession, err)
	}
	return nil
}

func MoveWindow(socketPath, source, targetSession string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	if err := client.MoveWindowToSession(source, targetSession); err != nil {
		return fmt.Errorf("failed to move window %s to %s: %w", source, targetSession, err)
	}
	return nil
}

func SwapWindows(socketPath, first, second string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	if err := client.SwapWindows(first, second); err != nil {
		return fmt.Errorf("failed to swap windows %s and %s: %w", first, second, err)
	}
	return nil
}

func KillWindows(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}

	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		if _, err := client.Command("kill-window", "-t", target); err != nil {
			return err
		}
	}
	return nil
}

func findWindow(client tmuxClient, target string) (windowHandle, error) {
	windows, err := client.ListAllWindows()
	if err != nil {
		return nil, err
	}
	for _, w := range windows {
		session := firstSession(w)
		if session == "" {
			session = strings.TrimSpace(w.Session)
		}
		candidates := []string{w.Id}
		if session != "" {
			candidates = append(candidates, fmt.Sprintf("%s:%d", session, w.Index))
		}
		for _, c := range candidates {
			if c == target {
				return newWindowHandle(w), nil
			}
		}
	}
	return nil, nil
}

func firstSession(w *gotmux.Window) string {
	if len(w.ActiveSessionsList) > 0 {
		return w.ActiveSessionsList[0]
	}
	if len(w.LinkedSessionsList) > 0 {
		return w.LinkedSessionsList[0]
	}
	return ""
}
