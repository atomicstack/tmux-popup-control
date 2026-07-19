package tmux

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

type SessionSpec struct {
	SocketPath string
	Name       string
	Dir        string
	Command    string
}

type WindowSpec struct {
	SocketPath string
	Session    string
	Index      int
	Name       string
	Dir        string
	Command    string
}

type PaneSpec struct {
	SocketPath string
	Target     string
	Dir        string
	Command    string
}

// CreateSession creates a new tmux session with the given name, starting
// directory, and optional startup command (for pane content restore).
func CreateSession(spec SessionSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	args := []string{"new-session", "-d", "-s", spec.Name}
	if spec.Dir != "" {
		args = append(args, "-c", spec.Dir)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
}

// CreateWindow creates a new window at the given index within a session.
// The window is created detached (-d) to avoid switching focus.
// An optional startup command is appended for pane content restore.
func CreateWindow(spec WindowSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	target := fmt.Sprintf("%s:%d", spec.Session, spec.Index)
	args := []string{"new-window", "-t", target, "-n", spec.Name, "-c", spec.Dir, "-d"}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
}

// SplitPane splits the pane at the given target, starting in dir.
// The new pane is created detached to avoid disturbing focus.
// An optional startup command is appended for pane content restore.
func SplitPane(spec PaneSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	args := []string{"split-window", "-d", "-t", spec.Target}
	if spec.Dir != "" {
		args = append(args, "-c", spec.Dir)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
}

// RespawnPane kills the running command in the target pane and restarts it in
// the given directory with an optional command. This is used during restore to
// set the first pane's working directory without polluting session_path.
func RespawnPane(spec PaneSpec) error {
	client, err := newTmux(spec.SocketPath)
	if err != nil {
		return err
	}
	args := []string{"respawn-pane", "-k", "-t", spec.Target}
	if spec.Dir != "" {
		args = append(args, "-c", spec.Dir)
	}
	if spec.Command != "" {
		args = append(args, spec.Command)
	}
	_, err = client.Command(args...)
	return err
}

// WaitFor blocks until the tmux wait-for channel has been signaled, the
// context is cancelled, or (when timeout > 0) the timeout elapses.
//
// This runs `tmux wait-for <channel>` as a one-shot exec subprocess rather
// than over the shared control-mode connection. A blocking wait-for on the
// control client would serialize every later command behind it, and if the
// signaling side (a pane's content-replay cat) dies before calling
// `wait-for -S`, the control connection would wedge permanently. exec +
// context.CommandContext gives us a bounded, cancellable wait whose process is
// killed on deadline/cancel.
func WaitFor(ctx context.Context, socketPath, channel string, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	args := append(baseArgs(socketPath), "wait-for", channel)
	if err := runExecCommandContext(ctx, "tmux", args...).Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("wait-for %s: %w", channel, ctxErr)
		}
		return fmt.Errorf("wait-for %s: %w", channel, err)
	}
	return nil
}

// SelectLayoutTarget applies the named layout to the given target window.
func SelectLayoutTarget(socketPath, target, layout string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	return client.SelectLayout(target, layout)
}

// SelectPane selects the given pane by target.
func SelectPane(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	return client.SelectPane(target)
}

// WindowIndices returns the set of occupied window indices for the given session.
func WindowIndices(socketPath, sessionName string) (map[int]bool, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	out, err := client.Command("list-windows", "-t", sessionName, "-F", "#{window_index}")
	if err != nil {
		return nil, err
	}
	indices := make(map[int]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		indices[idx] = true
	}
	return indices, nil
}

// CapturePaneContents captures the full scrollback of the target pane,
// preserving trailing whitespace and ANSI escape sequences (colours, bold, etc.).
func CapturePaneContents(socketPath, target string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	return client.CapturePane(target, &gotmux.CaptureOptions{
		EscTxtNBgAttr:    true,
		PreserveTrailing: true,
		StartLine:        "-",
	})
}

// SessionOption queries a session-level option. Returns empty string if unset.
// The option name is passed to show-options as a separate argument so it is
// never interpreted as a tmux format string — this avoids interpolating an
// untrusted option name (e.g. one derived from a session name read from a save
// file) into a "#{...}" format. show-options -qv returns the bare value (or an
// empty string with no error when the option is unset).
func SessionOption(socketPath, session, option string) string {
	client, err := newTmux(socketPath)
	if err != nil {
		return ""
	}
	out, err := client.Command("show-options", "-qv", "-t", session, option)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// SetSessionOption sets a session-level option.
func SetSessionOption(socketPath, session, option, value string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("set-option", "-t", session, option, value)
	return err
}

// ShowOption queries a tmux server-level (global) option value. Returns an
// empty string if the option is not set or an error occurs. Results are
// memoized per (socket, option) for the life of the process — see cache.go.
//
// This reads via a one-shot `tmux show-options -gqv`, not the control-mode
// connection. ShowOption feeds the autosave-status path (which tmux runs
// roughly once per second) and the restore path; a control-mode attach in
// either forces a full server state-sync and saturates the single-threaded
// server during a large restore. The per-process memoization below keeps the
// exec cost to one invocation per (socket, option).
func ShowOption(socketPath, option string) string {
	key := optionCacheKey(socketPath, option)
	optionCacheMu.RLock()
	if v, ok := optionCache[key]; ok {
		optionCacheMu.RUnlock()
		return v
	}
	optionCacheMu.RUnlock()

	args := append(baseArgs(socketPath), "show-options", "-gqv", option)
	output, err := runExecCommand("tmux", args...).Output()
	if err != nil {
		return ""
	}
	trimmed := strings.TrimSpace(string(output))
	optionCacheMu.Lock()
	optionCache[key] = trimmed
	optionCacheMu.Unlock()
	return trimmed
}

// DefaultCommand returns the tmux default-command setting. If unset or empty
// it falls back to the SHELL environment variable, then /bin/sh.
func DefaultCommand(socketPath string) string {
	if cmd := ShowOption(socketPath, "default-command"); cmd != "" {
		return cmd
	}
	// tmux itself falls back from default-command to default-shell, so a
	// configured shell must win over the process environment's $SHELL.
	if sh := ShowOption(socketPath, "default-shell"); sh != "" {
		return sh
	}
	if sh := os.Getenv("SHELL"); sh != "" {
		return sh
	}
	return "/bin/sh"
}
