package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

type PaneTracer struct{}

type paneReason string

const (
	PaneReasonEscape paneReason = "escape"
	PaneReasonEmpty  paneReason = "empty"
)

var Pane = PaneTracer{}

func (PaneTracer) Switch(target string) {
	logging.Trace("pane.switch", map[string]interface{}{"target": target})
}

func (PaneTracer) Kill(targets []string) {
	logging.Trace("pane.kill", map[string]interface{}{"targets": targets})
}

func (PaneTracer) Join(sources []string, target string) {
	logging.Trace("pane.join", map[string]interface{}{"sources": sources, "target": target})
}

func (PaneTracer) Break(target, destination string) {
	logging.Trace("pane.break", map[string]interface{}{"target": target, "destination": destination})
}

func (PaneTracer) SwapSelect(first string) {
	logging.Trace("pane.swap.select", map[string]interface{}{"first": first})
}

func (PaneTracer) Swap(first, second string) {
	logging.Trace("pane.swap", map[string]interface{}{"first": first, "second": second})
}

func (PaneTracer) Layout(layout string) {
	logging.Trace("pane.layout", map[string]interface{}{"layout": layout})
}

func (PaneTracer) Resize(direction string, amount int) {
	logging.Trace("pane.resize", map[string]interface{}{"direction": direction, "amount": amount})
}

func (PaneTracer) RenamePrompt(target string) {
	logging.Trace("pane.rename.prompt", map[string]interface{}{"target": target})
}

func (PaneTracer) Rename(target, title string) {
	logging.Trace("pane.rename", map[string]interface{}{"target": target, "title": title})
}

func (PaneTracer) CancelRename(target string, reason paneReason) {
	logging.Trace("pane.rename.cancel", map[string]interface{}{"target": target, "reason": string(reason)})
}

func (PaneTracer) SubmitRename(target, title string) {
	logging.Trace("pane.rename.submit", map[string]interface{}{"target": target, "title": title})
}
