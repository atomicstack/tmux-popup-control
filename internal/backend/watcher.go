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

	w.startPollers()

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

// fetchFunc retrieves a snapshot of one resource kind for the given socket.
type fetchFunc func(socketPath string) (any, error)

// startPollers launches one poller goroutine per resource kind. The pollers
// differ only by Kind and the fetch function, so they share a single start
// helper driven by a small table.
func (w *Watcher) startPollers() {
	pollers := []struct {
		kind  Kind
		fetch fetchFunc
	}{
		{KindSessions, func(socketPath string) (any, error) { return tmux.FetchSessions(socketPath) }},
		{KindWindows, func(socketPath string) (any, error) { return tmux.FetchWindows(socketPath) }},
		{KindPanes, func(socketPath string) (any, error) { return tmux.FetchPanes(socketPath) }},
	}
	for _, p := range pollers {
		w.start(p.kind, p.fetch)
	}
}

func (w *Watcher) start(kind Kind, fetch fetchFunc) {
	throttle := newThrottle(250 * time.Millisecond)
	w.wg.Go(func() {
		w.poll(kind, func(ctx context.Context) (any, error) {
			if err := throttle.wait(ctx); err != nil {
				return nil, err
			}
			return fetch(w.socketPath)
		})
	})
}

func (w *Watcher) poll(kind Kind, fetch func(context.Context) (any, error)) {
	emit := func() bool {
		span := logging.StartSpan("backend", "poll", logging.SpanOptions{
			Target: kind.String(),
			Attrs: map[string]any{
				"socket_path": w.socketPath,
				"interval_ms": w.interval.Milliseconds(),
			},
		})
		t0 := time.Now()
		logging.Trace("backend.poll.start", map[string]any{"kind": kind.String()})
		data, err := fetch(w.ctx)
		dur := time.Since(t0)
		count := watcherItemCount(data)
		if count >= 0 {
			span.AddAttr("item_count", count)
		}
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		logging.Trace("backend.poll.done", map[string]any{
			"kind":     kind.String(),
			"items":    count,
			"err":      errStr,
			"duration": dur.String(),
		})
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
