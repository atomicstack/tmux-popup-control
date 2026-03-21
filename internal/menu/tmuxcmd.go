package menu

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/logging"
)

func tmuxArgs(socket string, extra ...string) []string {
	args := make([]string, 0, len(extra)+2)
	if trimmed := strings.TrimSpace(socket); trimmed != "" {
		args = append(args, "-S", trimmed)
	}
	args = append(args, extra...)
	return args
}

func tmuxCmd(socket string, extra ...string) *exec.Cmd {
	args := tmuxArgs(socket, extra...)
	cmd := exec.Command("tmux", args...)
	if dir := socketDir(socket); dir != "" {
		cmd.Env = append(os.Environ(), "TMUX_TMPDIR="+dir)
	}
	return cmd
}

func runTmuxCommand(socket string, extra ...string) error {
	span := logging.StartSpan("menu", "tmux.exec", logging.SpanOptions{
		Target: strings.Join(extra, " "),
		Attrs: map[string]interface{}{
			"socket_path": socket,
			"argv":        extra,
		},
	})
	err := tmuxCmd(socket, extra...).Run()
	span.End(err)
	return err
}

func socketDir(socket string) string {
	trimmed := strings.TrimSpace(socket)
	if trimmed == "" {
		return ""
	}
	return filepath.Dir(trimmed)
}
