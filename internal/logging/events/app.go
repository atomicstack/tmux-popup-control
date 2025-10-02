package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

type AppTracer struct{}

var App = AppTracer{}

func (AppTracer) Start(payload map[string]interface{}) {
	logging.Trace("app.start", payload)
}
