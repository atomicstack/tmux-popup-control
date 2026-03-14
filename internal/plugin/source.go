package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Source executes each installed plugin's *.tmux files.
func Source(pluginDir string, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if !p.Installed || p.Dir == "" {
			continue
		}
		pattern := filepath.Join(p.Dir, "*.tmux")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("globbing %s: %w", pattern, err))
			continue
		}
		for _, tmuxFile := range matches {
			info, err := os.Stat(tmuxFile)
			if err != nil || info.Mode()&0o111 == 0 {
				continue // skip non-executable
			}
			cmd := exec.Command(tmuxFile)
			cmd.Stdout = nil
			cmd.Stderr = nil
			if err := cmd.Run(); err != nil {
				errs = append(errs, fmt.Errorf("sourcing %s: %w", tmuxFile, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors sourcing plugins: %v", errs)
	}
	return nil
}
