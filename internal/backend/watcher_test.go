package backend

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestThrottleWaitReturnsImmediatelyForNilOrZeroInterval(t *testing.T) {
	start := time.Now()
	var nilThrottle *throttle
	nilThrottle.wait(context.Background())
	newThrottle(0).wait(context.Background())
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("zero-interval throttle should return immediately, took %v", elapsed)
	}
}

func TestPollEmitsImmediatelyAndOnInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	w := &Watcher{
		socketPath: "test.sock",
		interval:   5 * time.Millisecond,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 4),
	}

	callCount := 0
	w.wg.Go(func() {
		w.poll(KindSessions, func(context.Context) (any, error) {
			callCount++
			return tmux.SessionSnapshot{
				Sessions: []tmux.Session{
					{Name: "main"},
					{Name: "extra"},
				}[:callCount],
			}, nil
		})
	})

	first := <-w.events
	second := <-w.events
	cancel()
	w.Wait()

	if first.Kind != KindSessions || second.Kind != KindSessions {
		t.Fatalf("expected session events, got %v and %v", first.Kind, second.Kind)
	}
	if got := len(first.Data.(tmux.SessionSnapshot).Sessions); got != 1 {
		t.Fatalf("first emit should be immediate with 1 session, got %d", got)
	}
	if got := len(second.Data.(tmux.SessionSnapshot).Sessions); got != 2 {
		t.Fatalf("second emit should come from ticker with 2 sessions, got %d", got)
	}
}

func TestPollPropagatesFetchError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	w := &Watcher{
		socketPath: "test.sock",
		interval:   time.Hour,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 1),
	}

	wantErr := errors.New("boom")
	w.wg.Go(func() {
		w.poll(KindWindows, func(context.Context) (any, error) {
			return nil, wantErr
		})
	})

	got := <-w.events
	cancel()
	w.Wait()

	if !errors.Is(got.Err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, got.Err)
	}
	if got.Kind != KindWindows {
		t.Fatalf("expected KindWindows, got %v", got.Kind)
	}
}

func TestThrottleWaitIsCancellable(t *testing.T) {
	// A long throttle interval combined with a cancelled context must return
	// promptly rather than sleeping for the full interval. This is what lets
	// watcher.Stop() drain pollers without a 250ms (or longer) delay.
	th := newThrottle(10 * time.Second)
	// Prime next so the second wait must block on the timer.
	if err := th.wait(context.Background()); err != nil {
		t.Fatalf("priming wait returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := th.wait(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("cancelled wait should return promptly, took %v", elapsed)
	}
}

func TestWatcherStopDrainsPromptlyThroughThrottle(t *testing.T) {
	// Stop() then Wait() must return promptly even though each poll goes
	// through a throttle. This guards the C2 teardown invariant: the watcher
	// fully drains before the shared tmux client is closed, and the C3a
	// cancellable throttle keeps the drain prompt.
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		socketPath: "test.sock",
		interval:   time.Millisecond,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 4),
	}

	throttle := newThrottle(10 * time.Second)
	w.wg.Go(func() {
		w.poll(KindSessions, func(pollCtx context.Context) (any, error) {
			if err := throttle.wait(pollCtx); err != nil {
				return nil, err
			}
			return tmux.SessionSnapshot{}, nil
		})
	})

	// Drain the immediate first emit.
	<-w.events

	start := time.Now()
	w.Stop()
	w.Wait()
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Stop+Wait should drain promptly, took %v", elapsed)
	}
}

func TestWatcherStopDrainsPromptlyThroughWedgedFetch(t *testing.T) {
	// A fetch that cannot observe cancellation must not block Stop+Wait
	// indefinitely. This reproduces the original shutdown hang, when poll's
	// fetch call was not cancellable at all — and it still guards the calls
	// that remain uncancellable now that gotmuxcc's list operations take a
	// context: ListClients and DisplayMessage have no context variants, so a
	// wedge in either would pin the poller exactly like this stub does.
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		socketPath: "test.sock",
		interval:   time.Hour,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 4),
	}

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	w.wg.Go(func() {
		w.poll(KindSessions, func(context.Context) (any, error) {
			<-release
			return tmux.SessionSnapshot{}, nil
		})
	})

	// The very first fetch is the wedged one, so there is no emit to drain
	// here — draining would block forever waiting on w.events.

	drained := make(chan struct{})
	start := time.Now()
	w.Stop()
	go func() {
		w.Wait()
		close(drained)
	}()
	select {
	case <-drained:
	case <-time.After(2 * time.Second):
		t.Fatalf("stop+wait did not drain within 2s: a wedged fetch blocks watcher shutdown")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("stop+wait should drain promptly, took %v", elapsed)
	}
}

func TestPollAwaitsInFlightFetchWithinGrace(t *testing.T) {
	// The grace window must not sacrifice the normal-shutdown invariant: a
	// fetch that is about to land should still be allowed to finish, not be
	// abandoned just because ctx was cancelled a moment earlier.
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		socketPath: "test.sock",
		interval:   5 * time.Millisecond,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 4),
	}

	var calls atomic.Int32
	var finished atomic.Bool
	inFlight := make(chan struct{})

	w.wg.Go(func() {
		w.poll(KindSessions, func(context.Context) (any, error) {
			// Only the second call is the slow one under test. Guard on the
			// exact call number: the ticker can drive further polls if cancel
			// races the emit, and closing inFlight twice would panic.
			if calls.Add(1) == 2 {
				close(inFlight)
				time.Sleep(50 * time.Millisecond)
				finished.Store(true)
			}
			return tmux.SessionSnapshot{}, nil
		})
	})

	// Drain the immediate first emit so the ticker triggers the second fetch.
	<-w.events

	select {
	case <-inFlight:
	case <-time.After(2 * time.Second):
		t.Fatalf("second fetch never started")
	}

	// Cancel while the second fetch is mid-flight (well inside the 250ms
	// grace window, since it only sleeps 50ms).
	cancel()
	w.Wait()

	if !finished.Load() {
		t.Fatalf("in-flight fetch should have been allowed to finish within the grace window")
	}
}

func TestAwaitFetchAbandonsWedgedFetchWithCancelledContext(t *testing.T) {
	// Direct unit test of awaitFetch: an already-cancelled context plus a
	// fetch that never returns must still yield a bounded result.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	start := time.Now()
	data, err := awaitFetch(ctx, func(context.Context) (any, error) {
		<-release
		return tmux.SessionSnapshot{}, nil
	})
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Fatalf("awaitFetch should abandon a wedged fetch promptly, took %v", elapsed)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if data != nil {
		t.Fatalf("expected nil data, got %v", data)
	}
}

func TestStartForwardsWatcherContextToFetch(t *testing.T) {
	// start must hand the watcher's context down to the fetch function rather
	// than dropping it. Without this the fetchers cannot honour cancellation
	// at all, which is what let a wedged tmux call hang Stop+Wait.
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		socketPath: "test.sock",
		interval:   time.Hour,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 4),
	}

	type call struct {
		ctx    context.Context
		socket string
	}
	calls := make(chan call, 1)

	w.start(KindSessions, func(fetchCtx context.Context, socketPath string) (any, error) {
		select {
		case calls <- call{ctx: fetchCtx, socket: socketPath}:
		default:
		}
		return tmux.SessionSnapshot{}, nil
	})

	<-w.events

	var got call
	select {
	case got = <-calls:
	case <-time.After(2 * time.Second):
		t.Fatalf("fetch was never called")
	}

	if got.socket != "test.sock" {
		t.Fatalf("expected socket path test.sock, got %q", got.socket)
	}
	if got.ctx == nil {
		t.Fatalf("fetch received a nil context")
	}
	if err := got.ctx.Err(); err != nil {
		t.Fatalf("fetch context should be live before stop, got %v", err)
	}

	w.Stop()
	w.Wait()

	if err := got.ctx.Err(); !errors.Is(err, context.Canceled) {
		t.Fatalf("stop should cancel the context handed to fetch, got %v", err)
	}
}

func TestWatcherItemCount(t *testing.T) {
	tests := []struct {
		name string
		data any
		want int
	}{
		{
			name: "sessions",
			data: tmux.SessionSnapshot{Sessions: []tmux.Session{{Name: "a"}, {Name: "b"}}},
			want: 2,
		},
		{
			name: "windows",
			data: tmux.WindowSnapshot{Windows: []tmux.Window{{ID: "1"}}},
			want: 1,
		},
		{
			name: "panes",
			data: tmux.PaneSnapshot{Panes: []tmux.Pane{{ID: "%1"}, {ID: "%2"}, {ID: "%3"}}},
			want: 3,
		},
		{
			name: "unknown",
			data: "nope",
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := watcherItemCount(tt.data); got != tt.want {
				t.Fatalf("watcherItemCount(%T) = %d, want %d", tt.data, got, tt.want)
			}
		})
	}
}
