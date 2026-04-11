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
		Attrs: map[string]any{
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

var runCommandOutputFn = func(socket string, args ...string) ([]byte, error) {
	return tmuxCmd(socket, args...).CombinedOutput()
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
func RunCommand(socketPath, command string) tea.Cmd {
	return func() tea.Msg {
		span := logging.StartSpan("menu", "tmux.run_command", logging.SpanOptions{
			Target: command,
			Attrs: map[string]any{
				"socket_path": socketPath,
			},
		})
		args := strings.Fields(command)
		if len(args) == 0 {
			err := fmt.Errorf("empty command")
			span.End(err)
			return ActionResult{Err: err}
		}
		out, err := runCommandOutputFn(socketPath, args...)
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
		result := ActionResult{Info: fmt.Sprintf("Ran: %s", command)}
		if output := strings.TrimRight(string(out), "\r\n"); strings.TrimSpace(output) != "" {
			result.Output = output
		} else if placeholder := showOptionEmptyPlaceholder(args); placeholder != "" {
			result.Output = placeholder
		}
		return result
	}
}

// showOptionEmptyPlaceholder returns a human-readable placeholder for empty
// output from a `show-options`, `show-window-options`, or `show-hooks`
// invocation that queried a single option/hook name. Returns "" when the
// command is not a show-options variant or no single-target name was given.
func showOptionEmptyPlaceholder(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var kind string
	switch args[0] {
	case "show-options", "show", "show-window-options", "showw":
		kind = "option"
	case "show-hooks":
		kind = "hook"
	default:
		return ""
	}
	target := showOptionPositional(args[1:])
	if target == "" {
		return ""
	}
	return fmt.Sprintf("[%s %s has no value]", kind, target)
}

// showOptionPositional walks the argument list for a `show-*` command and
// returns the first positional (the option or hook name), skipping bool-flag
// clusters and the `-t target` arg flag. Returns "" when no positional exists.
func showOptionPositional(args []string) string {
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if strings.HasPrefix(tok, "-") && len(tok) >= 2 {
			// Bool-flag cluster ending in `t` consumes the next token as
			// the target value (e.g. `-gqt main` or `-t main`).
			if strings.HasSuffix(tok, "t") {
				i++
			}
			continue
		}
		return tok
	}
	return ""
}
