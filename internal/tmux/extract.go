package tmux

import (
	"sort"
	"strconv"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// CaptureVisible returns the visible screen of target (no scrollback),
// joining wrapped lines like extrakto's `capture-pane -pJ`. It uses the
// shared control-mode client. Leaving StartLine/EndLine empty restricts the
// capture to the visible screen (no scrollback history).
func CaptureVisible(socketPath, target string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	return client.CapturePane(target, &gotmux.CaptureOptions{
		PreserveAndJoin: true, // -J: join wrapped lines
	})
}

// CaptureScrollback returns the full scrollback of target (joined wrapped
// lines), starting from the top of history. It uses the shared control-mode
// client.
func CaptureScrollback(socketPath, target string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	return client.CapturePane(target, &gotmux.CaptureOptions{
		PreserveAndJoin: true, // -J: join wrapped lines
		StartLine:       "-",  // start of history
	})
}

// WindowPaneIDs returns the pane ids of the window containing paneTarget,
// ordered by pane index ascending.
func WindowPaneIDs(socketPath, paneTarget string) ([]string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return nil, err
	}
	lines, err := client.ListPanesFormat(paneTarget, "", "#{pane_index}\t#{pane_id}")
	if err != nil {
		return nil, err
	}

	type indexedPane struct {
		index int
		id    string
	}
	panes := make([]indexedPane, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Split(trimmed, "\t")
		if len(fields) != 2 || fields[0] == "" || fields[1] == "" {
			continue
		}
		index, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		panes = append(panes, indexedPane{index: index, id: fields[1]})
	}

	sort.SliceStable(panes, func(i, j int) bool { return panes[i].index < panes[j].index })

	ids := make([]string, len(panes))
	for i, p := range panes {
		ids[i] = p.id
	}
	return ids, nil
}

// InsertText sets a paste buffer to text and pastes it into target without
// adding a trailing newline (-p writes to the paste buffer used by the next
// paste-buffer call).
func InsertText(socketPath, target, text string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	if _, err := client.Command("set-buffer", "--", text); err != nil {
		return err
	}
	_, err = client.Command("paste-buffer", "-p", "-t", target)
	return err
}

// CopyText stores text in a tmux paste buffer only; the system clipboard is
// handled separately by the caller (see internal/clipboard), not here.
func CopyText(socketPath, text string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("set-buffer", "--", text)
	return err
}
