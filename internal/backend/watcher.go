package backend

import (
	"context"
	"sync"
	"time"

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
	Data interface{}
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
	go w.poll(KindSessions, func(ctx context.Context) (interface{}, error) {
		throttle.wait()
		return tmux.FetchSessions(w.socketPath)
	})
}

func (w *Watcher) startWindowPoller() {
	throttle := newThrottle(250 * time.Millisecond)
	w.wg.Add(1)
	go w.poll(KindWindows, func(ctx context.Context) (interface{}, error) {
		throttle.wait()
		return tmux.FetchWindows(w.socketPath)
	})
}

func (w *Watcher) startPanePoller() {
	throttle := newThrottle(250 * time.Millisecond)
	w.wg.Add(1)
	go w.poll(KindPanes, func(ctx context.Context) (interface{}, error) {
		throttle.wait()
		return tmux.FetchPanes(w.socketPath)
	})
}

func (w *Watcher) poll(kind Kind, fetch func(context.Context) (interface{}, error)) {
	defer w.wg.Done()

	emit := func() bool {
		data, err := fetch(w.ctx)
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
