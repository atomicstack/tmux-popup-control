package backend

import (
	"context"
	"sync"
	"time"
)

// throttle ensures a minimum interval between successive operations.
type throttle struct {
	interval time.Duration

	mu   sync.Mutex
	next time.Time
}

func newThrottle(interval time.Duration) *throttle {
	if interval <= 0 {
		return &throttle{}
	}
	return &throttle{interval: interval}
}

// wait blocks until the throttle interval has elapsed since the previous call,
// or until ctx is cancelled. It returns ctx.Err() if cancelled, allowing the
// caller (e.g. a poller responding to Stop) to bail out promptly instead of
// being delayed by an uncancellable sleep.
func (t *throttle) wait(ctx context.Context) error {
	if t == nil || t.interval <= 0 {
		return ctx.Err()
	}
	for {
		t.mu.Lock()
		wait := time.Until(t.next)
		if wait <= 0 {
			t.next = time.Now().Add(t.interval)
			t.mu.Unlock()
			return nil
		}
		t.mu.Unlock()
		wait = min(wait, t.interval)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
