package plugin

import (
	"context"
	"os"
	"os/exec"
	"time"
)

// runGitCommand executes a git command and returns its combined output.
var runGitCommand = defaultRunGitCommand

// runGitCommandContext is the cancellation-aware git seam.
var runGitCommandContext = func(ctx context.Context, args ...string) ([]byte, error) {
	// Preserve the legacy seam for callers using the compatibility wrappers.
	if ctx.Done() == nil {
		return runGitCommand(args...)
	}
	return defaultRunGitCommandContext(ctx, args...)
}

func defaultRunGitCommand(args ...string) ([]byte, error) {
	return defaultRunGitCommandContext(context.Background(), args...)
}

func defaultRunGitCommandContext(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	configureGitCancellation(cmd)
	cmd.WaitDelay = time.Second
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return out, ctx.Err()
	}
	return out, err
}

// withStubGit replaces both git seams for the duration of a test.
func withStubGit(t interface{ Cleanup(func()) }, fn func(args ...string) ([]byte, error)) {
	orig, origContext := runGitCommand, runGitCommandContext
	runGitCommand = fn
	runGitCommandContext = func(ctx context.Context, args ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return fn(args...)
	}
	t.Cleanup(func() { runGitCommand, runGitCommandContext = orig, origContext })
}
