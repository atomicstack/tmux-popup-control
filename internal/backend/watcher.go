package backend

import (
	"context"
	"sync"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// Kind represents the type of data emitted by the backend watcher.
type Kind int

const (
	KindSessions Kind = iota
	KindWindows
	KindPanes
)

// Event conveys updated data or an error from a backend poll.
type Event struct {
	Kind Kind
	Data any
	Err  error
}

// Watcher polls tmux at a fixed interval and publishes events.
type Watcher struct {
	socketPath string
	interval   time.Duration

	ctx    context.Context
	cancel context.CancelFunc

	events chan Event
	wg     sync.WaitGroup
}

func (k Kind) String() string {
	switch k {
	case KindSessions:
		return "sessions"
	case KindWindows:
		return "windows"
	case KindPanes:
		return "panes"
	default:
		return "unknown"
	}
}

// NewWatcher creates a backend watcher that polls tmux every interval.
func NewWatcher(socketPath string, interval time.Duration) *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		socketPath: socketPath,
		interval:   interval,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 16),
	}

	w.startSessionPoller()
	w.startWindowPoller()
	w.startPanePoller()

	go func() {
		w.wg.Wait()
		close(w.events)
	}()

	return w
}

// Events returns a channel of backend events.
func (w *Watcher) Events() <-chan Event {
	return w.events
}

// Stop cancels the watcher. Pollers exit after their current fetch completes;
// use Wait if a clean drain is required (e.g. in tests).
func (w *Watcher) Stop() {
	w.cancel()
}

// Wait blocks until all poller goroutines have exited and the events channel
// is closed. Call after Stop when a clean shutdown is required.
func (w *Watcher) Wait() {
	w.wg.Wait()
}

func (w *Watcher) startSessionPoller() {
	throttle := newThrottle(250 * time.Millisecond)
	w.wg.Add(1)
	go w.poll(KindSessions, func(context.Context) (any, error) {
		throttle.wait()
		return tmux.FetchSessions(w.socketPath)
	})
}

func (w *Watcher) startWindowPoller() {
	throttle := newThrottle(250 * time.Millisecond)
	w.wg.Add(1)
	go w.poll(KindWindows, func(context.Context) (any, error) {
		throttle.wait()
		return tmux.FetchWindows(w.socketPath)
	})
}

func (w *Watcher) startPanePoller() {
	throttle := newThrottle(250 * time.Millisecond)
	w.wg.Add(1)
	go w.poll(KindPanes, func(context.Context) (any, error) {
		throttle.wait()
		return tmux.FetchPanes(w.socketPath)
	})
}

func (w *Watcher) poll(kind Kind, fetch func(context.Context) (any, error)) {
	defer w.wg.Done()

	emit := func() bool {
		span := logging.StartSpan("backend", "poll", logging.SpanOptions{
			Target: kind.String(),
			Attrs: map[string]any{
				"socket_path": w.socketPath,
				"interval_ms": w.interval.Milliseconds(),
			},
		})
		data, err := fetch(w.ctx)
		if count := watcherItemCount(data); count >= 0 {
			span.AddAttr("item_count", count)
		}
		span.End(err)
		evt := Event{Kind: kind, Data: data, Err: err}
		select {
		case <-w.ctx.Done():
			return false
		case w.events <- evt:
			return true
		}
	}

	if !emit() {
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if !emit() {
				return
			}
		}
	}
}

func watcherItemCount(data any) int {
	switch value := data.(type) {
	case tmux.SessionSnapshot:
		return len(value.Sessions)
	case tmux.WindowSnapshot:
		return len(value.Windows)
	case tmux.PaneSnapshot:
		return len(value.Panes)
	default:
		return -1
	}
}
