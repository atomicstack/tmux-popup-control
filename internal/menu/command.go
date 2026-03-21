package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
)

// listCommandsFn fetches the tmux command list. Swappable for tests.
var listCommandsFn = func(socket string) (string, error) {
	span := logging.StartSpan("menu", "tmux.list_commands", logging.SpanOptions{
		Target: "list-commands",
		Attrs: map[string]interface{}{
			"socket_path": socket,
		},
	})
	out, err := tmuxCmd(socket, "list-commands").Output()
	span.AddAttr("output_bytes", len(out))
	span.End(err)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func loadCommandMenu(ctx Context) ([]Item, error) {
	output, err := listCommandsFn(ctx.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("tmux list-commands failed: %w", err)
	}
	lines := splitLines(strings.TrimSpace(output))
	items := make([]Item, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		items = append(items, Item{ID: fields[0], Label: line})
	}
	return items, nil
}

// RunCommand executes an arbitrary tmux command given as a single string.
// defaultTarget is injected as "-t defaultTarget" when the command does not
// already contain a "-t" flag.
func RunCommand(socketPath, command, defaultTarget string) tea.Cmd {
	return func() tea.Msg {
		span := logging.StartSpan("menu", "tmux.run_command", logging.SpanOptions{
			Target: command,
			Attrs: map[string]interface{}{
				"socket_path":    socketPath,
				"default_target": defaultTarget,
			},
		})
		args := strings.Fields(command)
		if len(args) == 0 {
			err := fmt.Errorf("empty command")
			span.End(err)
			return ActionResult{Err: err}
		}
		if defaultTarget != "" && !hasFlag(args, "-t") {
			args = append(args[:1], append([]string{"-t", defaultTarget}, args[1:]...)...)
		}
		cmd := tmuxCmd(socketPath, args...)
		out, err := cmd.CombinedOutput()
		span.AddAttr("argv", args)
		span.AddAttr("output_bytes", len(out))
		if err != nil {
			detail := strings.TrimSpace(string(out))
			ran := "tmux " + strings.Join(args, " ")
			span.End(err)
			if detail != "" {
				return ActionResult{Err: fmt.Errorf("%s: %s", ran, detail)}
			}
			return ActionResult{Err: fmt.Errorf("%s: %w", ran, err)}
		}
		span.End(nil)
		return ActionResult{Info: fmt.Sprintf("Ran: %s", command)}
	}
}

// hasFlag reports whether args contain the given flag.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
