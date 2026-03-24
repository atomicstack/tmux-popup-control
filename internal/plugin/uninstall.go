package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Uninstall removes the specified plugin directories.
func Uninstall(pluginDir string, plugins []Plugin) error {
	var errs []error
	cleanBase := filepath.Clean(pluginDir) + string(os.PathSeparator)
	for _, p := range plugins {
		if p.Dir == "" {
			continue
		}
		// Ensure the directory is actually inside pluginDir to prevent
		// accidental removal of unrelated paths.
		cleanDir := filepath.Clean(p.Dir)
		if !strings.HasPrefix(cleanDir, cleanBase) {
			errs = append(errs, fmt.Errorf("refusing to remove %s: outside plugin directory", p.Name))
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
