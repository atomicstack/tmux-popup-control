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
	logging.Trace("window.switch", map[string]interface{}{"target": target})
}

func (WindowTracer) Kill(targets []string) {
	logging.Trace("window.kill", map[string]interface{}{"targets": targets})
}

func (WindowTracer) RenamePrompt(target string) {
	logging.Trace("window.rename.prompt", map[string]interface{}{"target": target})
}

func (WindowTracer) Rename(target, name string) {
	logging.Trace("window.rename", map[string]interface{}{"target": target, "name": name})
}

func (WindowTracer) Link(source, session string) {
	logging.Trace("window.link", map[string]interface{}{"source": source, "session": session})
}

func (WindowTracer) Move(source, session string) {
	logging.Trace("window.move", map[string]interface{}{"source": source, "session": session})
}

func (WindowTracer) SwapSelect(first string) {
	logging.Trace("window.swap.select", map[string]interface{}{"first": first})
}

func (WindowTracer) Swap(first, second string) {
	logging.Trace("window.swap", map[string]interface{}{"first": first, "second": second})
}

func (WindowTracer) CancelRename(target string, reason windowReason) {
	logging.Trace("window.rename.cancel", map[string]interface{}{"target": target, "reason": string(reason)})
}

func (WindowTracer) SubmitRename(target, name string) {
	logging.Trace("window.rename.submit", map[string]interface{}{"target": target, "name": name})
}
