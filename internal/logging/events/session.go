package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

type SessionTracer struct{}

type sessionReason string

const (
	SessionReasonEscape sessionReason = "escape"
	SessionReasonEmpty  sessionReason = "empty"
)

var Session = SessionTracer{}

func (SessionTracer) NewPrompt(existing int) {
	logging.Trace("session.new.prompt", map[string]any{"existing": existing})
}

func (SessionTracer) Switch(target string) {
	logging.Trace("session.switch", map[string]any{"target": target})
}

func (SessionTracer) RenamePrompt(target string) {
	logging.Trace("session.rename.prompt", map[string]any{"target": target})
}

func (SessionTracer) Detach(target string) {
	logging.Trace("session.detach", map[string]any{"target": target})
}

func (SessionTracer) Kill(target string) {
	logging.Trace("session.kill", map[string]any{"target": target})
}

func (SessionTracer) Create(name string) {
	logging.Trace("session.new.create", map[string]any{"name": name})
}

func (SessionTracer) Rename(target, name string) {
	logging.Trace("session.rename", map[string]any{"target": target, "name": name})
}

func (SessionTracer) CancelRename(target string, reason sessionReason) {
	logging.Trace("session.rename.cancel", map[string]any{"target": target, "reason": string(reason)})
}

func (SessionTracer) CancelNew(reason sessionReason) {
	logging.Trace("session.new.cancel", map[string]any{"reason": string(reason)})
}

func (SessionTracer) SubmitRename(target, name string) {
	logging.Trace("session.rename.submit", map[string]any{"target": target, "name": name})
}

func (SessionTracer) SubmitNew(name string) {
	logging.Trace("session.new.submit", map[string]any{"name": name})
}
