package logging

import "testing"

func TestResolveSQLiteDebugPath(t *testing.T) {
	got := ResolveSQLiteDebugPath("/tmp/bin/tmux-popup-control")
	want := "/tmp/bin/tmux-popup-control.debug.sqlite3"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
