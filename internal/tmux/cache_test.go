package tmux

import "testing"

func TestCacheResetRejectsInFlightStaleResult(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	calls := 0
	fake := &fakeClient{globalOptionFn: func(string) (string, error) {
		calls++
		if calls == 1 {
			close(entered)
			<-release
			return "before", nil
		}
		return "after", nil
	}}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	done := make(chan string, 1)
	go func() { done <- ShowOption("socket", "status-left") }()
	<-entered
	resetCaches()
	close(release)
	<-done
	if got := ShowOption("socket", "status-left"); got != "after" {
		t.Fatalf("in-flight read repopulated stale cache: %q", got)
	}
}
