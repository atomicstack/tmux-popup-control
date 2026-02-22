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
	defer client.Close()
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
	defer client.Close()
	window, err := findWindow(client, target)
	if err != nil {
		return err
	}
	if window == nil {
		return fmt.Errorf("window %s not found", target)
	}
	return window.Select()
}

func KillWindow(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	defer client.Close()
	window, err := findWindow(client, target)
	if err != nil {
		return err
	}
	if window == nil {
		return fmt.Errorf("window %s not found", target)
	}
	return window.Kill()
}

func UnlinkWindows(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	args := baseArgs(socketPath)
	for _, target := range targets {
		t := strings.TrimSpace(target)
		if t == "" {
			continue
		}
		cmd := runExecCommand("tmux", append(args, "unlink-window", "-k", "-t", t)...)
		if err := cmd.Run(); err != nil {
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
	defer client.Close()
	window, err := findWindow(client, target)
	if err != nil {
		return err
	}
	if window == nil {
		args := append(baseArgs(socketPath), "rename-window", "-t", target, newName)
		if err := runExecCommand("tmux", args...).Run(); err != nil {
			return fmt.Errorf("window %s not found", target)
		}
		return nil
	}
	return window.Rename(newName)
}

func LinkWindow(socketPath, source, targetSession string) error {
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "link-window", "-a", "-s", source, "-t", targetSession)
	if err := runExecCommand("tmux", args...).Run(); err != nil {
		return fmt.Errorf("failed to link window %s to %s: %w", source, targetSession, err)
	}
	return nil
}

func MoveWindow(socketPath, source, targetSession string) error {
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "move-window", "-a", "-s", source, "-t", targetSession)
	if err := runExecCommand("tmux", args...).Run(); err != nil {
		return fmt.Errorf("failed to move window %s to %s: %w", source, targetSession, err)
	}
	return nil
}

func SwapWindows(socketPath, first, second string) error {
	args := make([]string, 0, 8)
	if socketPath != "" {
		args = append(args, "-S", socketPath)
	}
	args = append(args, "swap-window", "-s", first, "-t", second)
	if err := runExecCommand("tmux", args...).Run(); err != nil {
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
	defer client.Close()
	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}
		window, err := findWindow(client, target)
		if err != nil {
			return err
		}
		if window == nil {
			args := append(baseArgs(socketPath), "kill-window", "-t", target)
			if err := runExecCommand("tmux", args...).Run(); err != nil {
				return fmt.Errorf("window %s not found", target)
			}
			continue
		}
		if err := window.Kill(); err != nil {
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
