//go:build darwin || linux

package plugin

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureGitCancellation(cmd *exec.Cmd) {
	// Git can spawn ssh and submodule helpers that keep its output pipes open.
	// Give this command its own group and cancel only that owned process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
