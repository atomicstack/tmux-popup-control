package tmux

import (
	"sync/atomic"
	"testing"
)

func TestCacheResetRejectsInFlightStaleResult(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	withStubCommander(t, func(string, ...string) commander {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
			return stubCommander{output: []byte("before")}
		}
		return stubCommander{output: []byte("after")}
	})
	done := make(chan string, 1)
	go func() { done <- ShowOption("socket", "@test") }()
	<-entered
	resetCaches()
	close(release)
	<-done
	if got := ShowOption("socket", "@test"); got != "after" {
		t.Fatalf("in-flight read repopulated stale cache: %q", got)
	}
}
