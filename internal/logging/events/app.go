package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

// AppTracer emits app-lifecycle trace events.
type AppTracer struct{}

// App is the shared AppTracer used to emit app-lifecycle trace events.
var App = AppTracer{}

// Start records the app start event with the given payload.
func (AppTracer) Start(payload map[string]any) {
	logging.Trace("app.start", payload)
}
