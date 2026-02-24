package menu

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var listKeysFn = tmux.ListKeys

func loadKeybindingMenu(ctx Context) ([]Item, error) {
	output, err := listKeysFn(ctx.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("tmux list-keys failed: %w", err)
	}
	lines := splitLines(strings.TrimSpace(output))
	items := make([]Item, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		items = append(items, Item{ID: line, Label: line})
	}
	return items, nil
}

func KeybindingAction(ctx Context, item Item) tea.Cmd {
	binding := strings.TrimSpace(item.ID)
	if binding == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid key binding selection")} }
	}
	return func() tea.Msg {
		needsCopyMode := strings.Contains(binding, "copy-mode") && !strings.Contains(binding, "prefix")
		if needsCopyMode {
			if err := runTmuxCommand(ctx.SocketPath, "copy-mode"); err != nil {
				return ActionResult{Err: fmt.Errorf("tmux copy-mode failed: %w", err)}
			}
		}
		cmdArgs, err := keybindingCommandArgs(binding)
		if err != nil {
			return ActionResult{Err: err}
		}
		if err := runTmuxCommand(ctx.SocketPath, cmdArgs...); err != nil {
			return ActionResult{Err: fmt.Errorf("tmux %s failed: %w", strings.Join(cmdArgs, " "), err)}
		}
		return ActionResult{Info: fmt.Sprintf("Executed %s", strings.Join(cmdArgs, " "))}
	}
}

func keybindingCommandArgs(binding string) ([]string, error) {
	tokens := strings.Fields(binding)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty key binding")
	}
	if tokens[0] != "bind-key" && tokens[0] != "bind" {
		return nil, fmt.Errorf("unsupported key binding format")
	}
	idx := 1
	for idx < len(tokens) {
		tok := tokens[idx]
		if !strings.HasPrefix(tok, "-") {
			break
		}
		switch tok {
		case "-n", "-r", "-N":
			idx++
		case "-T", "-t", "-R":
			idx += 2
		default:
			idx++
		}
	}
	// skip key table/key specification
	if idx < len(tokens) {
		idx++
	}
	if idx >= len(tokens) {
		return nil, fmt.Errorf("unable to parse command from binding")
	}
	return tokens[idx:], nil
}
