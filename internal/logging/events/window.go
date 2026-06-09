package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

// WindowTracer emits window-related trace events.
type WindowTracer struct{}

type windowReason string

// Window cancel reasons recorded alongside cancel trace events.
const (
	ReasonEscape windowReason = "escape"
	ReasonEmpty  windowReason = "empty"
)

// Window is the shared WindowTracer used to emit window trace events.
var Window = WindowTracer{}

// Switch records a window switch to target.
func (WindowTracer) Switch(target string) {
	logging.Trace("window.switch", map[string]any{"target": target})
}

// Kill records a kill of the given window targets.
func (WindowTracer) Kill(targets []string) {
	logging.Trace("window.kill", map[string]any{"targets": targets})
}

// RenamePrompt records that the rename prompt was opened for target.
func (WindowTracer) RenamePrompt(target string) {
	logging.Trace("window.rename.prompt", map[string]any{"target": target})
}

// Rename records a window rename from target to name.
func (WindowTracer) Rename(target, name string) {
	logging.Trace("window.rename", map[string]any{"target": target, "name": name})
}

// Link records linking a window from source into session.
func (WindowTracer) Link(source, session string) {
	logging.Trace("window.link", map[string]any{"source": source, "session": session})
}

// PullFromSession records pulling a window from another session.
func (WindowTracer) PullFromSession(source, session string) {
	logging.Trace("window.pull-from-session", map[string]any{"source": source, "session": session})
}

// PushToSession records pushing a window to another session.
func (WindowTracer) PushToSession(source, session string) {
	logging.Trace("window.push-to-session", map[string]any{"source": source, "session": session})
}

// SwapSelect records selecting the first window in a swap operation.
func (WindowTracer) SwapSelect(first string) {
	logging.Trace("window.swap.select", map[string]any{"first": first})
}

// Swap records swapping two windows.
func (WindowTracer) Swap(first, second string) {
	logging.Trace("window.swap", map[string]any{"first": first, "second": second})
}

// CancelRename records that a window rename was cancelled.
func (WindowTracer) CancelRename(target string, reason windowReason) {
	logging.Trace("window.rename.cancel", map[string]any{"target": target, "reason": string(reason)})
}

// SubmitRename records that a window rename was submitted.
func (WindowTracer) SubmitRename(target, name string) {
	logging.Trace("window.rename.submit", map[string]any{"target": target, "name": name})
}

// Layout records applying the given window layout.
func (WindowTracer) Layout(layout string) {
	logging.Trace("window.layout", map[string]any{"layout": layout})
}
