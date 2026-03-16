package plugin

import "fmt"

// UpdateOne pulls the latest changes for a single plugin and updates submodules.
func UpdateOne(p Plugin) error {
	if !p.Installed || p.Dir == "" {
		return fmt.Errorf("plugin %s is not installed", p.Name)
	}
	if _, err := runGitCommand("-C", p.Dir, "pull"); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	if _, err := runGitCommand("-C", p.Dir, "submodule", "update", "--init", "--recursive"); err != nil {
		return fmt.Errorf("submodule update failed: %w", err)
	}
	return nil
}

// Update pulls the latest changes for each plugin and updates submodules.
func Update(pluginDir string, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if !p.Installed || p.Dir == "" {
			continue
		}
		if _, err := runGitCommand("-C", p.Dir, "pull"); err != nil {
			errs = append(errs, fmt.Errorf("updating %s: %w", p.Name, err))
			continue
		}
		if _, err := runGitCommand("-C", p.Dir, "submodule", "update", "--init", "--recursive"); err != nil {
			errs = append(errs, fmt.Errorf("updating submodules for %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to update: %v", len(errs), errs)
	}
	return nil
}
