package plugin

import (
	"fmt"
	"os"
)

// Install clones plugins that are not yet installed.
func Install(pluginDir string, plugins []Plugin) error {
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}
	var errs []error
	for _, p := range plugins {
		if p.Installed {
			continue
		}
		if err := clonePlugin(p); err != nil {
			errs = append(errs, fmt.Errorf("installing %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to install: %v", len(errs), errs)
	}
	return nil
}

func clonePlugin(p Plugin) error {
	args := []string{"clone"}
	if p.Branch != "" {
		args = append(args, "-b", p.Branch)
	}
	args = append(args, "--single-branch", "--recursive")

	directArgs := append(append([]string{}, args...), p.Source, p.Dir)
	if _, err := runGitCommand(directArgs...); err == nil {
		return nil
	}

	ghURL := fmt.Sprintf("https://git::@github.com/%s", p.Source)
	fallbackArgs := append(append([]string{}, args...), ghURL, p.Dir)
	if _, err := runGitCommand(fallbackArgs...); err != nil {
		return fmt.Errorf("git clone failed for %s: %w", p.Source, err)
	}
	return nil
}
