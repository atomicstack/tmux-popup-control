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
	nilThrottle.wait()
	newThrottle(0).wait()
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Fatalf("zero-interval throttle should return immediately, took %v", elapsed)
	}
}

func TestPollEmitsImmediatelyAndOnInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		socketPath: "test.sock",
		interval:   5 * time.Millisecond,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 4),
	}

	callCount := 0
	w.wg.Add(1)
	go w.poll(KindSessions, func(context.Context) (any, error) {
		callCount++
		return tmux.SessionSnapshot{
			Sessions: []tmux.Session{
				{Name: "main"},
				{Name: "extra"},
			}[:callCount],
		}, nil
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w := &Watcher{
		socketPath: "test.sock",
		interval:   time.Hour,
		ctx:        ctx,
		cancel:     cancel,
		events:     make(chan Event, 1),
	}

	wantErr := errors.New("boom")
	w.wg.Add(1)
	go w.poll(KindWindows, func(context.Context) (any, error) {
		return nil, wantErr
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
