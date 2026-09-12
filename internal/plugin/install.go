package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const cloneStagingPrefix = ".plugin-install-"

// InstallOne clones a single plugin that is not yet installed.
func InstallOne(pluginDir string, p Plugin) error {
	return InstallOneContext(context.Background(), pluginDir, p)
}

// InstallOneContext is the cancellable variant of InstallOne.
func InstallOneContext(ctx context.Context, pluginDir string, p Plugin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}
	return clonePluginContext(ctx, p)
}

// Install clones plugins that are not yet installed.
func Install(pluginDir string, plugins []Plugin) error {
	return InstallContext(context.Background(), pluginDir, plugins)
}

// InstallContext is the cancellable variant of Install.
func InstallContext(ctx context.Context, pluginDir string, plugins []Plugin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}
	var errs []error
	for _, p := range plugins {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if p.Installed {
			continue
		}
		if err := clonePluginContext(ctx, p); err != nil {
			errs = append(errs, fmt.Errorf("installing %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to install: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

func clonePluginContext(ctx context.Context, p Plugin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := []string{"clone"}
	if p.Branch != "" {
		args = append(args, "-b", p.Branch)
	}
	args = append(args, "--single-branch", "--recursive")

	// Use "--" to terminate option parsing so p.Source cannot be
	// interpreted as a git flag (e.g. --upload-pack=...).
	// Stage each attempt inside a directory owned exclusively by this call.
	// Keeping it beside the destination guarantees publication stays on one filesystem.
	if _, err := os.Lstat(p.Dir); err == nil {
		return fmt.Errorf("plugin destination already exists: %s", p.Dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking plugin destination: %w", err)
	}
	stage, err := os.MkdirTemp(filepath.Dir(p.Dir), cloneStagingPrefix+"*")
	if err != nil {
		return fmt.Errorf("creating clone staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	cloneDir := filepath.Join(stage, "clone")
	sources := []string{p.Source, fmt.Sprintf("https://git::@github.com/%s", p.Source)}
	for _, source := range sources {
		cloneArgs := append(append([]string{}, args...), "--", source, cloneDir)
		_, err = runGitCommandContext(ctx, cloneArgs...)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			// Never overwrite an existing file, directory (even empty), or symlink.
			if err := publishClone(cloneDir, p.Dir); err != nil {
				return fmt.Errorf("publishing plugin %s: %w", p.Name, err)
			}
			return nil
		}
		if cleanupErr := os.RemoveAll(cloneDir); cleanupErr != nil {
			return fmt.Errorf("cleaning partial clone: %w", cleanupErr)
		}
	}
	return fmt.Errorf("git clone failed for %s: %w", p.Source, err)
}
