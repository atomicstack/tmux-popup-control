package menu

import (
	"fmt"
	"os/exec"
	"strings"
)

func loadKeybindingMenu(Context) ([]Item, error) {
	cmd := exec.Command("tmux", "list-keys")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tmux list-keys failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	lines := splitLines(strings.TrimSpace(string(output)))
	items := make([]Item, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		items = append(items, Item{ID: line, Label: line})
	}
	return items, nil
}
