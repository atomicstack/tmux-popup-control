package tmux

import (
	"fmt"
	"strconv"
	"strings"
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
	args := append(baseArgs(socketPath), "rename-pane", "-t", trimmedTarget, trimmedTitle)
	return runExecCommand("tmux", args...).Run()
}

func KillPanes(socketPath string, targets []string) error {
	if len(targets) == 0 {
		return nil
	}
	args := baseArgs(socketPath)
	for _, target := range targets {
		t := strings.TrimSpace(target)
		if t == "" {
			continue
		}
		cmd := runExecCommand("tmux", append(args, "kill-pane", "-t", t)...)
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func SwapPanes(socketPath, first, second string) error {
	if strings.TrimSpace(first) == "" || strings.TrimSpace(second) == "" {
		return fmt.Errorf("pane ids required")
	}
	args := append(baseArgs(socketPath), "swap-pane", "-s", first, "-t", second)
	return runExecCommand("tmux", args...).Run()
}

func MovePane(socketPath, source, target string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("pane source required")
	}
	args := append(baseArgs(socketPath), "move-pane", "-s", source)
	if strings.TrimSpace(target) != "" {
		args = append(args, "-t", target)
	}
	return runExecCommand("tmux", args...).Run()
}

func BreakPane(socketPath, source, destination string) error {
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("pane source required")
	}
	args := append(baseArgs(socketPath), "break-pane", "-s", source)
	if strings.TrimSpace(destination) != "" {
		args = append(args, "-t", destination)
	}
	return runExecCommand("tmux", args...).Run()
}

func SelectLayout(socketPath, layout string) error {
	if strings.TrimSpace(layout) == "" {
		return fmt.Errorf("layout required")
	}
	args := append(baseArgs(socketPath), "select-layout", layout)
	return runExecCommand("tmux", args...).Run()
}

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
	args := append(baseArgs(socketPath), "resize-pane", flag, strconv.Itoa(amount))
	return runExecCommand("tmux", args...).Run()
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
	if err := SwitchClient(socketPath, clientID, session); err != nil {
		return err
	}
	if err := SelectWindow(socketPath, window); err != nil {
		return err
	}
	args := append(baseArgs(socketPath), "select-pane", "-t", target)
	return runExecCommand("tmux", args...).Run()
}
