package menu

import (
	"fmt"
	"os/exec"
	"strings"
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
