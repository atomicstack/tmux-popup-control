package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

type WindowTracer struct{}

type windowReason string

const (
	ReasonEscape windowReason = "escape"
	ReasonEmpty  windowReason = "empty"
)

var Window = WindowTracer{}

func (WindowTracer) Switch(target string) {
	logging.Trace("window.switch", map[string]any{"target": target})
}

func (WindowTracer) Kill(targets []string) {
	logging.Trace("window.kill", map[string]any{"targets": targets})
}

func (WindowTracer) RenamePrompt(target string) {
	logging.Trace("window.rename.prompt", map[string]any{"target": target})
}

func (WindowTracer) Rename(target, name string) {
	logging.Trace("window.rename", map[string]any{"target": target, "name": name})
}

func (WindowTracer) Link(source, session string) {
	logging.Trace("window.link", map[string]any{"source": source, "session": session})
}

func (WindowTracer) PullFromSession(source, session string) {
	logging.Trace("window.pull-from-session", map[string]any{"source": source, "session": session})
}

func (WindowTracer) PushToSession(source, session string) {
	logging.Trace("window.push-to-session", map[string]any{"source": source, "session": session})
}

func (WindowTracer) SwapSelect(first string) {
	logging.Trace("window.swap.select", map[string]any{"first": first})
}

func (WindowTracer) Swap(first, second string) {
	logging.Trace("window.swap", map[string]any{"first": first, "second": second})
}

func (WindowTracer) CancelRename(target string, reason windowReason) {
	logging.Trace("window.rename.cancel", map[string]any{"target": target, "reason": string(reason)})
}

func (WindowTracer) SubmitRename(target, name string) {
	logging.Trace("window.rename.submit", map[string]any{"target": target, "name": name})
}

func (WindowTracer) Layout(layout string) {
	logging.Trace("window.layout", map[string]any{"layout": layout})
}
