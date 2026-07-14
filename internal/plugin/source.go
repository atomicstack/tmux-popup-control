package plugin

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// executablePath resolves the running binary's path. It is a package-level
// var so tests can point SourceSelf at a fake plugin directory.
var executablePath = os.Executable

// Source executes each installed plugin's *.tmux files.
func Source(pluginDir string, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if !p.Installed || p.Dir == "" {
			continue
		}
		if err := sourceDir(p.Dir); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to source: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// SourceSelf executes the *.tmux files that ship alongside the running
// tmux-popup-control binary — its own keybindings — so the plugin does not
// need an explicit `@plugin` declaration just to bind its keys. It is a no-op
// when a declared plugin already covers the binary's directory (that path is
// sourced by Source), so keys are never bound twice.
func SourceSelf(declared []Plugin) error {
	exe, err := executablePath()
	if err != nil {
		return fmt.Errorf("locating executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	selfDir := filepath.Dir(exe)
	for _, p := range declared {
		if p.Installed && p.Dir != "" && sameDir(p.Dir, selfDir) {
			return nil // already sourced by Source
		}
	}
	return sourceDir(selfDir)
}

// sourceDir executes each executable *.tmux file in dir.
func sourceDir(dir string) error {
	pattern := filepath.Join(dir, "*.tmux")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("globbing %s: %w", pattern, err)
	}
	var errs []error
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
	return errors.Join(errs...)
}

// sameDir reports whether two directory paths refer to the same location,
// resolving symlinks and relative components where possible.
func sameDir(a, b string) bool {
	return resolveDir(a) == resolveDir(b)
}

func resolveDir(dir string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(dir)
}
