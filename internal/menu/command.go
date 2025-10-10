package menu

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// CommandPromptMsg requests that the UI open the tmux command prompt with an initial value.
type CommandPromptMsg struct {
	Command string
	Label   string
}

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
