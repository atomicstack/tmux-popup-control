package plugin

import (
	"os"
	"os/exec"
)

// runGitCommand executes a git command and returns its combined output.
var runGitCommand = defaultRunGitCommand

func defaultRunGitCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd.CombinedOutput()
}

// withStubGit replaces runGitCommand for the duration of a test.
func withStubGit(t interface{ Cleanup(func()) }, fn func(args ...string) ([]byte, error)) {
	orig := runGitCommand
	runGitCommand = fn
	t.Cleanup(func() { runGitCommand = orig })
}
