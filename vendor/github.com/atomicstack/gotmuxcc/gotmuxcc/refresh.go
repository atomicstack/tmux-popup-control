package gotmuxcc

import (
	"fmt"
	"strings"
)

// SetClientSize sets the overall terminal size for this control client.
// This is equivalent to `refresh-client -C WxH`.
func (t *Tmux) SetClientSize(width, height int) error {
	cmd := fmt.Sprintf("refresh-client -C %dx%d", width, height)
	_, err := t.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set client size: %w", err)
	}
	return nil
}

// SetWindowSize sets the size for a specific window on this control client.
// The windowID should include the @ prefix (e.g. "@0").
// This is equivalent to `refresh-client -C @wid:WxH`.
func (t *Tmux) SetWindowSize(windowID string, width, height int) error {
	cmd := fmt.Sprintf("refresh-client -C %s:%dx%d", windowID, width, height)
	_, err := t.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set window size: %w", err)
	}
	return nil
}

// ClearWindowSize clears a per-window size override, reverting to the client size.
// The windowID should include the @ prefix (e.g. "@0").
// This is equivalent to `refresh-client -C @wid:`.
func (t *Tmux) ClearWindowSize(windowID string) error {
	cmd := fmt.Sprintf("refresh-client -C %s:", windowID)
	_, err := t.runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to clear window size: %w", err)
	}
	return nil
}

// --- Flow control (refresh-client -f / -A) ---

// SetControlFlags sets control-mode client flags via `refresh-client -f`.
// Multiple flags can be comma-separated, e.g. "pause-after=1000,no-output".
//
// Supported flags:
//   - "no-output": suppress pane output notifications
//   - "pause-after": enable pause-after mode without age limit
//   - "pause-after=<ms>": enable pause-after with age threshold in seconds
//   - "wait-exit": wait for empty line before exit on detach
//
// Prefix a flag with "!" to toggle it off (e.g. "!no-output").
func (t *Tmux) SetControlFlags(flags string) error {
	_, err := t.runCommand(fmt.Sprintf("refresh-client -f %s", quoteArgument(flags)))
	if err != nil {
		return fmt.Errorf("failed to set control flags: %w", err)
	}
	return nil
}

// EnablePauseAfter enables pause-after flow control with the given age
// threshold in seconds. When a pane's buffered output exceeds this age,
// tmux pauses it and sends a %pause notification instead of disconnecting
// the client.
func (t *Tmux) EnablePauseAfter(seconds int) error {
	return t.SetControlFlags(fmt.Sprintf("pause-after=%d", seconds))
}

// DisablePauseAfter disables pause-after flow control.
func (t *Tmux) DisablePauseAfter() error {
	return t.SetControlFlags("!pause-after")
}

// SetPaneOutput controls output delivery for a specific pane.
//
// Possible actions:
//   - "on": enable output for the pane
//   - "off": suppress output for the pane (tmux stops reading from the pane
//     PTY if all control clients turn it off)
//   - "pause": manually pause the pane (discard queued output)
//   - "continue": resume a paused pane
//
// The paneID should include the % prefix (e.g. "%0").
func (t *Tmux) SetPaneOutput(paneID, action string) error {
	arg := fmt.Sprintf("%s:%s", paneID, action)
	_, err := t.runCommand(fmt.Sprintf("refresh-client -A %s", quoteArgument(arg)))
	if err != nil {
		return fmt.Errorf("failed to set pane output: %w", err)
	}
	return nil
}

// EnablePaneOutput enables output for a pane. If the pane was paused,
// output resumes.
func (t *Tmux) EnablePaneOutput(paneID string) error {
	return t.SetPaneOutput(paneID, "on")
}

// DisablePaneOutput suppresses output for a pane.
func (t *Tmux) DisablePaneOutput(paneID string) error {
	return t.SetPaneOutput(paneID, "off")
}

// PausePaneOutput manually pauses a pane, discarding queued output.
func (t *Tmux) PausePaneOutput(paneID string) error {
	return t.SetPaneOutput(paneID, "pause")
}

// ContinuePaneOutput resumes a paused pane.
func (t *Tmux) ContinuePaneOutput(paneID string) error {
	return t.SetPaneOutput(paneID, "continue")
}

// SetMultiplePaneOutputs sets output state for multiple panes in a single
// command. Each entry should be "paneID:action" (e.g. "%0:on", "%1:off").
func (t *Tmux) SetMultiplePaneOutputs(entries []string) error {
	if len(entries) == 0 {
		return nil
	}
	parts := make([]string, 0, len(entries)+2)
	parts = append(parts, "refresh-client")
	for _, e := range entries {
		parts = append(parts, "-A", quoteArgument(e))
	}
	_, err := t.runCommand(strings.Join(parts, " "))
	if err != nil {
		return fmt.Errorf("failed to set pane outputs: %w", err)
	}
	return nil
}

// --- Pane color reports (refresh-client -r) ---

// ReportPaneColors relays a terminal color query response back to tmux for
// a specific pane. This allows the control client to provide pane fg/bg
// color information that tmux can use to replicate colors.
//
// The paneID should include the % prefix (e.g. "%0").
// The report is the raw color response string from the terminal (e.g. the
// payload from an OSC 10/11 response).
//
// This is equivalent to `refresh-client -r %id:report`.
func (t *Tmux) ReportPaneColors(paneID, report string) error {
	arg := fmt.Sprintf("%s:%s", paneID, report)
	_, err := t.runCommand(fmt.Sprintf("refresh-client -r %s", quoteArgument(arg)))
	if err != nil {
		return fmt.Errorf("failed to report pane colors: %w", err)
	}
	return nil
}
