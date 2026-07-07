package tmux

import gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"

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

// CopyText stores text in a tmux paste buffer (mvp: buffer only, no system
// clipboard / OSC-52).
func CopyText(socketPath, text string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("set-buffer", "--", text)
	return err
}
