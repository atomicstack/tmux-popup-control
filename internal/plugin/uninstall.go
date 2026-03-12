package plugin

import (
	"fmt"
	"os"
)

const selfName = "tmux-popup-control"

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

// Tidy computes the set of installed plugins not in the declared list.
// Never includes self (tmux-popup-control). Does not delete anything —
// callers use Uninstall after confirmation.
func Tidy(pluginDir string, declared []Plugin) ([]Plugin, error) {
	installed, err := Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	declaredNames := make(map[string]struct{}, len(declared))
	for _, d := range declared {
		declaredNames[d.Name] = struct{}{}
	}
	var toRemove []Plugin
	for _, p := range installed {
		if p.Name == selfName {
			continue
		}
		if _, ok := declaredNames[p.Name]; !ok {
			toRemove = append(toRemove, p)
		}
	}
	return toRemove, nil
}
