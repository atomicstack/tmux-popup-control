package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

// PluginTracer emits structured trace events for plugin operations.
type PluginTracer struct{}

// Plugins is the singleton tracer for plugin events.
var Plugins = PluginTracer{}

func (PluginTracer) Install(name string) {
	logging.Trace("plugins.install", map[string]any{"name": name})
}

func (PluginTracer) Update(name string) {
	logging.Trace("plugins.update", map[string]any{"name": name})
}

func (PluginTracer) Uninstall(name string) {
	logging.Trace("plugins.uninstall", map[string]any{"name": name})
}

func (PluginTracer) Source(name string) {
	logging.Trace("plugins.source", map[string]any{"name": name})
}

func (PluginTracer) InitPlugins(count int) {
	logging.Trace("plugins.init", map[string]any{"count": count})
}
