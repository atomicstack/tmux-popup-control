//go:build !sqlitedebug

package logging

import (
	"errors"
	"testing"
)

func TestEnableSQLiteDebugUnavailableWithoutBuildTag(t *testing.T) {
	err := EnableSQLiteDebug(SQLiteRunInfo{ExecutablePath: t.TempDir() + "/tmux-popup-control"})
	if !errors.Is(err, ErrSQLiteDebugUnavailable) {
		t.Fatalf("expected ErrSQLiteDebugUnavailable, got %v", err)
	}
	if DebugSQLiteEnabled() {
		t.Fatalf("expected sqlite debug sink to stay disabled")
	}
	// the rest of the api must stay a no-op without a sink.
	span := StartSpan("test", "noop", SpanOptions{})
	span.AddAttr("k", "v")
	span.End(nil)
	Close(RunResult{ExitStatus: "ok"})
}
