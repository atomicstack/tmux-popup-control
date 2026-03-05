package gotmuxcc

import "fmt"

// SubscriptionTarget specifies what a subscription monitors.
type SubscriptionTarget string

const (
	// SubSession monitors the session level (no specific pane or window).
	SubSession SubscriptionTarget = ""
	// SubAllPanes monitors all panes in the session.
	SubAllPanes SubscriptionTarget = "%*"
	// SubAllWindows monitors all windows in the session.
	SubAllWindows SubscriptionTarget = "@*"
)

// SubPane returns a subscription target for a specific pane (e.g. "%0").
func SubPane(paneID string) SubscriptionTarget {
	return SubscriptionTarget(paneID)
}

// SubWindow returns a subscription target for a specific window (e.g. "@0").
func SubWindow(windowID string) SubscriptionTarget {
	return SubscriptionTarget(windowID)
}

// Subscribe registers a control-mode subscription. tmux evaluates the format
// string once per second and sends a %subscription-changed notification when
// the value changes.
//
// The target selects what the format is evaluated against:
//   - SubSession: session level
//   - SubPane("%0"): a specific pane
//   - SubAllPanes: all panes in the session
//   - SubWindow("@0"): a specific window
//   - SubAllWindows: all windows in the session
func (t *Tmux) Subscribe(name string, target SubscriptionTarget, format string) error {
	arg := fmt.Sprintf("%s:%s:%s", name, string(target), format)
	_, err := t.runCommand(fmt.Sprintf("refresh-client -B %s", quoteArgument(arg)))
	if err != nil {
		return fmt.Errorf("failed to subscribe %q: %w", name, err)
	}
	return nil
}

// Unsubscribe removes a previously registered subscription by name.
func (t *Tmux) Unsubscribe(name string) error {
	_, err := t.runCommand(fmt.Sprintf("refresh-client -B %s", quoteArgument(name)))
	if err != nil {
		return fmt.Errorf("failed to unsubscribe %q: %w", name, err)
	}
	return nil
}
