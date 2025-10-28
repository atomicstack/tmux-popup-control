package backend

import (
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

func (t *throttle) wait() {
	if t == nil || t.interval <= 0 {
		return
	}
	for {
		t.mu.Lock()
		wait := time.Until(t.next)
		if wait <= 0 {
			t.next = time.Now().Add(t.interval)
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()
		if wait > t.interval {
			wait = t.interval
		}
		time.Sleep(wait)
	}
}
