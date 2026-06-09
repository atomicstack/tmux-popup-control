package backend

import (
	"context"
	"errors"
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
