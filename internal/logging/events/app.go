package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

type AppTracer struct{}

var App = AppTracer{}

func (AppTracer) Start(payload map[string]any) {
	logging.Trace("app.start", payload)
}
