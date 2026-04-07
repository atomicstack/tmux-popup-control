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
	logging.Trace("pane.switch", map[string]any{"target": target})
}

func (PaneTracer) Kill(targets []string) {
	logging.Trace("pane.kill", map[string]any{"targets": targets})
}

func (PaneTracer) Join(sources []string, target string) {
	logging.Trace("pane.join", map[string]any{"sources": sources, "target": target})
}

func (PaneTracer) Break(target, destination string) {
	logging.Trace("pane.break", map[string]any{"target": target, "destination": destination})
}

func (PaneTracer) SwapSelect(first string) {
	logging.Trace("pane.swap.select", map[string]any{"first": first})
}

func (PaneTracer) Swap(first, second string) {
	logging.Trace("pane.swap", map[string]any{"first": first, "second": second})
}

func (PaneTracer) Resize(direction string, amount int) {
	logging.Trace("pane.resize", map[string]any{"direction": direction, "amount": amount})
}

func (PaneTracer) RenamePrompt(target string) {
	logging.Trace("pane.rename.prompt", map[string]any{"target": target})
}

func (PaneTracer) Rename(target, title string) {
	logging.Trace("pane.rename", map[string]any{"target": target, "title": title})
}

func (PaneTracer) CancelRename(target string, reason paneReason) {
	logging.Trace("pane.rename.cancel", map[string]any{"target": target, "reason": string(reason)})
}

func (PaneTracer) SubmitRename(target, title string) {
	logging.Trace("pane.rename.submit", map[string]any{"target": target, "title": title})
}

func (PaneTracer) CapturePrompt(target string) {
	logging.Trace("pane.capture.prompt", map[string]any{"target": target})
}

func (PaneTracer) Capture(target, filePath string, escSeqs bool) {
	logging.Trace("pane.capture", map[string]any{"target": target, "file": filePath, "esc_seqs": escSeqs})
}

func (PaneTracer) CaptureCancel(reason paneReason) {
	logging.Trace("pane.capture.cancel", map[string]any{"reason": string(reason)})
}

func (PaneTracer) CaptureSubmit(filePath string) {
	logging.Trace("pane.capture.submit", map[string]any{"file": filePath})
}
