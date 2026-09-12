package backend

import (
	"context"
	"fmt"
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

	events      chan Event
	wg          sync.WaitGroup
	closeEvents sync.Once
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

	go w.Wait()

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
	w.closeEvents.Do(func() { close(w.events) })
}

// fetchFunc retrieves a snapshot of one resource kind for the given socket.
// The context is the watcher's own: fetchers are expected to abandon work once
// it is cancelled, which is what keeps Stop/Wait bounded.
type fetchFunc func(ctx context.Context, socketPath string) (any, error)

// startPollers launches one poller goroutine per resource kind. The pollers
// differ only by Kind and the fetch function, so they share a single start
// helper driven by a small table.
func (w *Watcher) startPollers() {
	pollers := []struct {
		kind  Kind
		fetch fetchFunc
	}{
		{KindSessions, func(ctx context.Context, socketPath string) (any, error) {
			return tmux.FetchSessionsContext(ctx, socketPath)
		}},
		{KindWindows, func(ctx context.Context, socketPath string) (any, error) {
			return tmux.FetchWindowsContext(ctx, socketPath)
		}},
		{KindPanes, func(ctx context.Context, socketPath string) (any, error) {
			return tmux.FetchPanesContext(ctx, socketPath)
		}},
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
			return fetch(ctx, w.socketPath)
		})
	})
}

// fetchAbandonGrace bounds how long a poller waits for an in-flight fetch to
// land after cancellation before abandoning it.
//
// Since gotmuxcc v0.2.0 the list calls take a context and return promptly once
// it is cancelled, so this window is normally never reached. It survives as a
// backstop for the calls that still have no context variant — ListClients and
// DisplayMessage, reached through realAttachedClients and currentSessionName —
// which run near the end of each fetch and would otherwise pin the poller
// exactly as the uncancellable list calls once did. Those two are fast in
// practice, so letting them land inside the window lets the poller exit
// cleanly rather than detaching a goroutine whose result is discarded anyway.
const fetchAbandonGrace = 250 * time.Millisecond

// awaitFetch runs fetch on its own goroutine so a call that cannot observe
// cancellation still cannot pin the poller. It returns as soon as the fetch
// lands; once ctx is cancelled it waits only fetchAbandonGrace longer, then
// gives up on the result.
//
// Giving up is safe rather than lossy: gotmuxcc's own context cancellation is
// caller-side only — an already-written command stays in the router's pending
// queue and its reply is discarded on arrival — and Close is idempotent and
// safe alongside in-flight commands, so the tmux.Shutdown that follows
// teardown reclaims anything still outstanding.
func awaitFetch(ctx context.Context, fetch func(context.Context) (any, error)) (any, error) {
	type result struct {
		data any
		err  error
	}
	// Buffered so an abandoned fetch can always deliver and exit.
	done := make(chan result, 1)
	go func() {
		data, err := fetch(ctx)
		done <- result{data: data, err: err}
	}()

	select {
	case r := <-done:
		return r.data, r.err
	case <-ctx.Done():
	}

	timer := time.NewTimer(fetchAbandonGrace)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.data, r.err
	case <-timer.C:
		return nil, fmt.Errorf("fetch abandoned after cancellation: %w", ctx.Err())
	}
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
		data, err := awaitFetch(w.ctx, fetch)
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
