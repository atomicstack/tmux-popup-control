package menu

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// CommandPromptMsg requests that the UI open the tmux command prompt with an initial value.
type CommandPromptMsg struct {
	Command string
	Label   string
}

var listCommandsFn = tmux.ListCommands

func loadCommandMenu(ctx Context) ([]Item, error) {
	output, err := listCommandsFn(ctx.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("tmux list-commands failed: %w", err)
	}
	lines := splitLines(strings.TrimSpace(output))
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
	if command == "command-prompt" {
		return func() tea.Msg {
			if err := runTmuxCommand(ctx.SocketPath, "command-prompt"); err != nil {
				return ActionResult{Err: fmt.Errorf("tmux command-prompt failed: %w", err)}
			}
			return ActionResult{Info: "Opened command prompt"}
		}
	}
	initial := command
	if !strings.HasSuffix(initial, " ") {
		initial += " "
	}
	return func() tea.Msg {
		return CommandPromptMsg{Command: initial, Label: item.Label}
	}
}

// CommandPrompt opens the tmux command prompt with the provided initial text.
// The command executes out-of-band to allow the popup to close cleanly first.
func CommandPrompt(socketPath, initial string) error {
	script := strings.Builder{}
	script.WriteString("sleep 0.03; tmux command-prompt")
	if strings.TrimSpace(initial) != "" {
		script.WriteString(" -I ")
		script.WriteString(fmt.Sprintf("%q", initial))
	}
	return runTmuxCommand(socketPath, "run-shell", "-b", script.String())
}
