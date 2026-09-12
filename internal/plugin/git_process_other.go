//go:build !darwin && !linux

package plugin

import "os/exec"

func configureGitCancellation(cmd *exec.Cmd) {}
