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
	cmd.Env = tmuxCommandEnv(socket)
	return cmd
}

func runTmuxCommand(socket string, extra ...string) error {
	span := logging.StartSpan("menu", "tmux.exec", logging.SpanOptions{
		Target: strings.Join(extra, " "),
		Attrs: map[string]any{
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

func tmuxCommandEnv(socket string) []string {
	env := os.Environ()
	if dir := socketDir(socket); dir != "" {
		env = append(env, "TMUX_TMPDIR="+dir)
	}
	// display-popup clears TMUX_PANE, but tmux uses it to resolve the current
	// pane/session/window context for commands like "move-window -r". main.sh
	// captures the host pane before opening the popup, so restore that here for
	// subprocess-based command execution.
	if strings.TrimSpace(os.Getenv("TMUX_PANE")) == "" {
		if pane := strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_ID")); pane != "" {
			env = append(env, "TMUX_PANE="+pane)
		}
	}
	return env
}
