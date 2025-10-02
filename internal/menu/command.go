package menu

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func loadCommandMenu(Context) ([]Item, error) {
	cmd := exec.Command("tmux", "list-commands")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux list-commands failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	lines := splitLines(strings.TrimSpace(string(output)))
	items := make([]Item, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		items = append(items, Item{ID: fields[0], Label: line})
	}
	return items, nil
}

func CommandAction(ctx Context, item Item) tea.Cmd {
	command := strings.TrimSpace(item.ID)
	if command == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid command selection")} }
	}
	return func() tea.Msg {
		if err := runTmuxCommand(ctx.SocketPath, "command-prompt", "-I", command); err != nil {
			return ActionResult{Err: fmt.Errorf("tmux command-prompt failed: %w", err)}
		}
		return ActionResult{Info: fmt.Sprintf("Prompted command %s", command)}
	}
}
