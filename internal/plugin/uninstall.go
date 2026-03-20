package plugin

import (
	"fmt"
	"os"
)

// Uninstall removes the specified plugin directories.
func Uninstall(pluginDir string, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if p.Dir == "" {
			continue
		}
		if err := os.RemoveAll(p.Dir); err != nil {
			errs = append(errs, fmt.Errorf("removing %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to uninstall: %v", len(errs), errs)
	}
	return nil
}
