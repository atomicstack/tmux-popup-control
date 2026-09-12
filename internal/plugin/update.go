package plugin

import (
	"context"
	"errors"
	"fmt"
)

// UpdatePullOne pulls the latest changes for a single plugin.
func UpdatePullOne(p Plugin) error {
	return UpdatePullOneContext(context.Background(), p)
}

// UpdatePullOneContext is the cancellable variant of UpdatePullOne.
func UpdatePullOneContext(ctx context.Context, p Plugin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.Installed || p.Dir == "" {
		return fmt.Errorf("plugin %s is not installed", p.Name)
	}
	if _, err := runGitCommandContext(ctx, "-C", p.Dir, "pull"); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	return nil
}

// UpdateSubmodulesOne updates submodules for a single plugin.
func UpdateSubmodulesOne(p Plugin) error {
	return UpdateSubmodulesOneContext(context.Background(), p)
}

// UpdateSubmodulesOneContext is the cancellable variant of UpdateSubmodulesOne.
func UpdateSubmodulesOneContext(ctx context.Context, p Plugin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !p.Installed || p.Dir == "" {
		return fmt.Errorf("plugin %s is not installed", p.Name)
	}
	if _, err := runGitCommandContext(ctx, "-C", p.Dir, "submodule", "update", "--init", "--recursive"); err != nil {
		return fmt.Errorf("submodule update failed: %w", err)
	}
	return nil
}

// UpdateOne pulls the latest changes for a single plugin and updates submodules.
func UpdateOne(p Plugin) error {
	return UpdateOneContext(context.Background(), p)
}

// UpdateOneContext is the cancellable variant of UpdateOne.
func UpdateOneContext(ctx context.Context, p Plugin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := UpdatePullOneContext(ctx, p); err != nil {
		return err
	}
	return UpdateSubmodulesOneContext(ctx, p)
}

// Update pulls the latest changes for each plugin and updates submodules.
func Update(pluginDir string, plugins []Plugin) error {
	return UpdateContext(context.Background(), pluginDir, plugins)
}

// UpdateContext is the cancellable variant of Update.
func UpdateContext(ctx context.Context, pluginDir string, plugins []Plugin) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var errs []error
	for _, p := range plugins {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if !p.Installed || p.Dir == "" {
			continue
		}
		if err := UpdatePullOneContext(ctx, p); err != nil {
			errs = append(errs, fmt.Errorf("updating %s: %w", p.Name, err))
			continue
		}
		if err := UpdateSubmodulesOneContext(ctx, p); err != nil {
			errs = append(errs, fmt.Errorf("updating submodules for %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to update: %w", len(errs), errors.Join(errs...))
	}
	return nil
}
