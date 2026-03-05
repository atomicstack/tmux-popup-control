package gotmuxcc

import (
	"fmt"
	"strings"
)

// DisplayMessage evaluates a tmux format string and returns the result.
// If target is non-empty, it is used with -t to set the context for
// format variable evaluation. This is equivalent to:
//
//	tmux display-message [-t target] -p <format>
func (t *Tmux) DisplayMessage(target, format string) (string, error) {
	q := t.query().cmd("display-message")
	if target != "" {
		q.fargs("-t", target)
	}
	q.fargs("-p", format)

	result, err := q.run()
	if err != nil {
		return "", fmt.Errorf("failed to display message: %w", err)
	}
	return strings.TrimRight(result.raw(), "\n"), nil
}

// ListSessionsFormat runs list-sessions with a custom format string and
// returns the raw output lines. This allows callers to use arbitrary -F
// format strings instead of the structured ListSessions() method.
func (t *Tmux) ListSessionsFormat(format string) ([]string, error) {
	q := t.query().
		cmd("list-sessions").
		fargs("-F", format)
	result, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	return result.result.Lines, nil
}

// ListWindowsFormat runs list-windows with a custom format string and
// returns the raw output lines.
//
// If target is non-empty, it scopes the listing to that session via -t.
// If target is empty, -a is used to list all windows.
// If filter is non-empty, it is passed via -f.
func (t *Tmux) ListWindowsFormat(target, filter, format string) ([]string, error) {
	q := t.query().cmd("list-windows")
	if target != "" {
		q.fargs("-t", target)
	} else {
		q.fargs("-a")
	}
	if filter != "" {
		q.fargs("-f", filter)
	}
	q.fargs("-F", format)

	result, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to list windows: %w", err)
	}
	return result.result.Lines, nil
}

// ListPanesFormat runs list-panes with a custom format string and
// returns the raw output lines.
//
// If target is non-empty, it scopes the listing to that window/session via -t.
// If target is empty, -a is used to list all panes.
// If filter is non-empty, it is passed via -f.
func (t *Tmux) ListPanesFormat(target, filter, format string) ([]string, error) {
	q := t.query().cmd("list-panes")
	if target != "" {
		q.fargs("-t", target)
	} else {
		q.fargs("-a")
	}
	if filter != "" {
		q.fargs("-f", filter)
	}
	q.fargs("-F", format)

	result, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to list panes: %w", err)
	}
	return result.result.Lines, nil
}
