// Package clipboard copies text to the host's native system clipboard,
// dispatching on the running OS. it is consumer-agnostic: no tmux, bubbletea,
// or menu imports.
package clipboard

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// errUnsupportedOS is returned when no clipboard tool is known for the OS.
var errUnsupportedOS = errors.New("clipboard: unsupported operating system")

// runner writes stdin to the named command's standard input. it is a package
// var so tests can stub the actual os/exec invocation (mirrors runGitCommand /
// runExecCommand elsewhere in the repo).
type runner = func(name string, stdin string, args ...string) error

var runClipboardCommand runner = func(name string, stdin string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.Run()
}

// Copy writes text to the system clipboard using the native tool for the
// running OS.
func Copy(text string) error {
	return copyForOS(runtime.GOOS, text, runClipboardCommand)
}

// copyForOS dispatches to the native clipboard tool for goos, writing text to
// the tool's stdin. on linux it tries wl-copy, then xclip, then xsel, first
// working tool wins.
func copyForOS(goos string, text string, run runner) error {
	switch goos {
	case "darwin":
		return run("pbcopy", text)
	case "windows":
		return run("clip", text)
	case "linux":
		attempts := []struct {
			name string
			args []string
		}{
			{"wl-copy", nil},
			{"xclip", []string{"-selection", "clipboard"}},
			{"xsel", []string{"--clipboard", "--input"}},
		}
		var lastErr error
		for _, a := range attempts {
			if err := run(a.name, text, a.args...); err != nil {
				lastErr = err
				continue
			}
			return nil
		}
		if lastErr != nil {
			return lastErr
		}
		return errUnsupportedOS
	default:
		return fmt.Errorf("%w: %s", errUnsupportedOS, goos)
	}
}
