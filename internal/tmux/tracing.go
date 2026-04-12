package tmux

import (
	"fmt"
	"maps"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
)

type tracedTmuxClient struct {
	socketPath string
	inner      tmuxClient
}

type tracedCommander struct {
	name string
	args []string
	cmd  commander
}

func newTracedTmuxClient(socketPath string, inner tmuxClient) tmuxClient {
	if inner == nil {
		return nil
	}
	return &tracedTmuxClient{socketPath: socketPath, inner: inner}
}

func (c *tracedTmuxClient) baseAttrs() map[string]any {
	return map[string]any{
		"socket_path": c.socketPath,
	}
}

func (c *tracedTmuxClient) traceErr(operation, target string, attrs map[string]any, fn func() error) error {
	span := logging.StartSpan("tmux.control", operation, logging.SpanOptions{
		Target: target,
		Attrs:  mergeTracingAttrs(c.baseAttrs(), attrs),
	})
	err := fn()
	span.End(err)
	return err
}

func traceValue[T any](component, operation, target string, attrs map[string]any, fn func() (T, error)) (T, error) {
	span := logging.StartSpan(component, operation, logging.SpanOptions{
		Target: target,
		Attrs:  attrs,
	})
	value, err := fn()
	span.End(err)
	return value, err
}

func (c *tracedTmuxClient) ListSessions() ([]*gotmux.Session, error) {
	return traceValue("tmux.control", "list_sessions", "sessions", c.baseAttrs(), c.inner.ListSessions)
}

func (c *tracedTmuxClient) ListAllWindows() ([]*gotmux.Window, error) {
	return traceValue("tmux.control", "list_windows", "windows", c.baseAttrs(), c.inner.ListAllWindows)
}

func (c *tracedTmuxClient) ListAllPanes() ([]*gotmux.Pane, error) {
	return traceValue("tmux.control", "list_panes", "panes", c.baseAttrs(), c.inner.ListAllPanes)
}

func (c *tracedTmuxClient) ListClients() ([]*gotmux.Client, error) {
	return traceValue("tmux.control", "list_clients", "clients", c.baseAttrs(), c.inner.ListClients)
}

func (c *tracedTmuxClient) SwitchClient(options *gotmux.SwitchClientOptions) error {
	target := ""
	if options != nil {
		target = strings.TrimSpace(options.TargetSession)
	}
	return c.traceErr("switch_client", target, c.baseAttrs(), func() error {
		return c.inner.SwitchClient(options)
	})
}

func (c *tracedTmuxClient) GetSessionByName(name string) (*gotmux.Session, error) {
	return traceValue("tmux.control", "get_session", name, c.baseAttrs(), func() (*gotmux.Session, error) {
		return c.inner.GetSessionByName(name)
	})
}

func (c *tracedTmuxClient) NewSession(options *gotmux.SessionOptions) (*gotmux.Session, error) {
	target := ""
	if options != nil {
		target = strings.TrimSpace(options.Name)
	}
	return traceValue("tmux.control", "new_session", target, c.baseAttrs(), func() (*gotmux.Session, error) {
		return c.inner.NewSession(options)
	})
}

func (c *tracedTmuxClient) KillServer() error {
	return c.traceErr("kill_server", "server", c.baseAttrs(), c.inner.KillServer)
}

func (c *tracedTmuxClient) Close() error {
	return c.traceErr("close", "client", c.baseAttrs(), c.inner.Close)
}

func (c *tracedTmuxClient) RenamePane(target, title string) error {
	return c.traceErr("rename_pane", target, mergeTracingAttrs(c.baseAttrs(), map[string]any{"title": title}), func() error {
		return c.inner.RenamePane(target, title)
	})
}

func (c *tracedTmuxClient) SwapPanes(first, second string) error {
	return c.traceErr("swap_panes", first, mergeTracingAttrs(c.baseAttrs(), map[string]any{"second": second}), func() error {
		return c.inner.SwapPanes(first, second)
	})
}

func (c *tracedTmuxClient) MovePane(source, target string) error {
	return c.traceErr("move_pane", source, mergeTracingAttrs(c.baseAttrs(), map[string]any{"target": target}), func() error {
		return c.inner.MovePane(source, target)
	})
}

func (c *tracedTmuxClient) BreakPane(source, destination string) error {
	return c.traceErr("break_pane", source, mergeTracingAttrs(c.baseAttrs(), map[string]any{"destination": destination}), func() error {
		return c.inner.BreakPane(source, destination)
	})
}

func (c *tracedTmuxClient) JoinPane(source, target string) error {
	return c.traceErr("join_pane", source, mergeTracingAttrs(c.baseAttrs(), map[string]any{"target": target}), func() error {
		return c.inner.JoinPane(source, target)
	})
}

func (c *tracedTmuxClient) SelectPane(target string) error {
	return c.traceErr("select_pane", target, c.baseAttrs(), func() error {
		return c.inner.SelectPane(target)
	})
}

func (c *tracedTmuxClient) CapturePane(target string, options *gotmux.CaptureOptions) (string, error) {
	return traceValue("tmux.control", "capture_pane", target, c.baseAttrs(), func() (string, error) {
		return c.inner.CapturePane(target, options)
	})
}

func (c *tracedTmuxClient) UnlinkWindow(target string) error {
	return c.traceErr("unlink_window", target, c.baseAttrs(), func() error {
		return c.inner.UnlinkWindow(target)
	})
}

func (c *tracedTmuxClient) LinkWindow(source, targetSession string) error {
	return c.traceErr("link_window", source, mergeTracingAttrs(c.baseAttrs(), map[string]any{"target_session": targetSession}), func() error {
		return c.inner.LinkWindow(source, targetSession)
	})
}

func (c *tracedTmuxClient) MoveWindowToSession(source, targetSession string) error {
	return c.traceErr("move_window", source, mergeTracingAttrs(c.baseAttrs(), map[string]any{"target_session": targetSession}), func() error {
		return c.inner.MoveWindowToSession(source, targetSession)
	})
}

func (c *tracedTmuxClient) SwapWindows(first, second string) error {
	return c.traceErr("swap_windows", first, mergeTracingAttrs(c.baseAttrs(), map[string]any{"second": second}), func() error {
		return c.inner.SwapWindows(first, second)
	})
}

func (c *tracedTmuxClient) SelectWindow(target string) error {
	return c.traceErr("select_window", target, c.baseAttrs(), func() error {
		return c.inner.SelectWindow(target)
	})
}

func (c *tracedTmuxClient) SelectLayout(target string, layout string) error {
	return c.traceErr("select_layout", target, mergeTracingAttrs(c.baseAttrs(), map[string]any{"layout": layout}), func() error {
		return c.inner.SelectLayout(target, layout)
	})
}

func (c *tracedTmuxClient) SplitWindow(target string, options *gotmux.SplitWindowOptions) error {
	return c.traceErr("split_window", target, c.baseAttrs(), func() error {
		return c.inner.SplitWindow(target, options)
	})
}

func (c *tracedTmuxClient) GlobalOption(key string) (string, error) {
	return traceValue("tmux.control", "global_option", key, c.baseAttrs(), func() (string, error) {
		return c.inner.GlobalOption(key)
	})
}

func (c *tracedTmuxClient) Options(target, level string) ([]*gotmux.Option, error) {
	attrs := mergeTracingAttrs(c.baseAttrs(), map[string]any{"level": level, "target": target})
	return traceValue("tmux.control", "options", target, attrs, func() ([]*gotmux.Option, error) {
		return c.inner.Options(target, level)
	})
}

func (c *tracedTmuxClient) DisplayMessage(target, format string) (string, error) {
	return traceValue("tmux.control", "display_message", target, mergeTracingAttrs(c.baseAttrs(), map[string]any{"format": format}), func() (string, error) {
		return c.inner.DisplayMessage(target, format)
	})
}

func (c *tracedTmuxClient) ListSessionsFormat(format string) ([]string, error) {
	return traceValue("tmux.control", "list_sessions_format", format, c.baseAttrs(), func() ([]string, error) {
		return c.inner.ListSessionsFormat(format)
	})
}

func (c *tracedTmuxClient) ListWindowsFormat(target, filter, format string) ([]string, error) {
	return traceValue("tmux.control", "list_windows_format", target, mergeTracingAttrs(c.baseAttrs(), map[string]any{
		"filter": filter,
		"format": format,
	}), func() ([]string, error) {
		return c.inner.ListWindowsFormat(target, filter, format)
	})
}

func (c *tracedTmuxClient) ListPanesFormat(target, filter, format string) ([]string, error) {
	return traceValue("tmux.control", "list_panes_format", target, mergeTracingAttrs(c.baseAttrs(), map[string]any{
		"filter": filter,
		"format": format,
	}), func() ([]string, error) {
		return c.inner.ListPanesFormat(target, filter, format)
	})
}

func (c *tracedTmuxClient) Command(parts ...string) (string, error) {
	target := ""
	if len(parts) > 0 {
		target = parts[0]
	}
	return traceValue("tmux.control", "command", target, mergeTracingAttrs(c.baseAttrs(), map[string]any{
		"argv": append([]string(nil), parts...),
	}), func() (string, error) {
		return c.inner.Command(parts...)
	})
}

func (c tracedCommander) Run() error {
	span := logging.StartSpan("tmux.exec", "run", logging.SpanOptions{
		Target: commandTarget(c.name, c.args),
		Attrs: map[string]any{
			"argv": append([]string(nil), c.args...),
		},
	})
	err := c.cmd.Run()
	span.End(err)
	return err
}

func (c tracedCommander) Output() ([]byte, error) {
	span := logging.StartSpan("tmux.exec", "output", logging.SpanOptions{
		Target: commandTarget(c.name, c.args),
		Attrs: map[string]any{
			"argv": append([]string(nil), c.args...),
		},
	})
	output, err := c.cmd.Output()
	span.AddAttr("output_bytes", len(output))
	span.End(err)
	return output, err
}

func mergeTracingAttrs(base, extra map[string]any) map[string]any {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	merged := make(map[string]any, len(base)+len(extra))
	maps.Copy(merged, base)
	maps.Copy(merged, extra)
	return merged
}

func commandTarget(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return fmt.Sprintf("%s %s", name, strings.Join(args, " "))
}
