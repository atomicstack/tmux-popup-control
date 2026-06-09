package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

// SessionTracer emits session-related trace events.
type SessionTracer struct{}

type sessionReason string

// Session cancel reasons recorded alongside cancel trace events.
const (
	SessionReasonEscape sessionReason = "escape"
	SessionReasonEmpty  sessionReason = "empty"
)

// Session is the shared SessionTracer used to emit session trace events.
var Session = SessionTracer{}

// NewPrompt records that the new-session prompt was opened.
func (SessionTracer) NewPrompt(existing int) {
	logging.Trace("session.new.prompt", map[string]any{"existing": existing})
}

// Switch records a session switch to the given target.
func (SessionTracer) Switch(target string) {
	logging.Trace("session.switch", map[string]any{"target": target})
}

// RenamePrompt records that the rename prompt was opened for target.
func (SessionTracer) RenamePrompt(target string) {
	logging.Trace("session.rename.prompt", map[string]any{"target": target})
}

// Detach records a session detach for target.
func (SessionTracer) Detach(target string) {
	logging.Trace("session.detach", map[string]any{"target": target})
}

// Kill records a session kill for target.
func (SessionTracer) Kill(target string) {
	logging.Trace("session.kill", map[string]any{"target": target})
}

// Create records the creation of a new session.
func (SessionTracer) Create(name string) {
	logging.Trace("session.new.create", map[string]any{"name": name})
}

// Rename records a session rename from target to name.
func (SessionTracer) Rename(target, name string) {
	logging.Trace("session.rename", map[string]any{"target": target, "name": name})
}

// CancelRename records that a rename was cancelled for the given reason.
func (SessionTracer) CancelRename(target string, reason sessionReason) {
	logging.Trace("session.rename.cancel", map[string]any{"target": target, "reason": string(reason)})
}

// CancelNew records that creating a new session was cancelled.
func (SessionTracer) CancelNew(reason sessionReason) {
	logging.Trace("session.new.cancel", map[string]any{"reason": string(reason)})
}

// SubmitRename records that a rename was submitted for target.
func (SessionTracer) SubmitRename(target, name string) {
	logging.Trace("session.rename.submit", map[string]any{"target": target, "name": name})
}

// SubmitNew records that a new-session create was submitted.
func (SessionTracer) SubmitNew(name string) {
	logging.Trace("session.new.submit", map[string]any{"name": name})
}
