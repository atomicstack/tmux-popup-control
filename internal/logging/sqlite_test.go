package logging

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSQLiteDebugPath(t *testing.T) {
	got := ResolveSQLiteDebugPath("/tmp/bin/tmux-popup-control")
	want := "/tmp/bin/tmux-popup-control.debug.sqlite3"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSQLiteDebugCapturesRunEventsAndSpans(t *testing.T) {
	Configure("")
	SetTraceEnabled(false)

	tempDir := t.TempDir()
	executablePath := filepath.Join(tempDir, "tmux-popup-control")
	runErr := errors.New("boom")

	if err := EnableSQLiteDebug(SQLiteRunInfo{
		Version:        "test-version",
		ExecutablePath: executablePath,
		CWD:            tempDir,
		Args:           []string{"--debug-to-sqlite"},
		Flags: map[string]string{
			"debugToSQLite": "true",
		},
		SocketPath:  "/tmp/test.sock",
		RootMenu:    "session",
		MenuArgs:    "expanded",
		ClientID:    "client-1",
		SessionName: "work",
	}); err != nil {
		t.Fatalf("EnableSQLiteDebug failed: %v", err)
	}

	Trace("menu.enter", map[string]interface{}{"level": "root", "item": "pane"})
	Error(runErr)
	span := StartSpan("tmux.control", "command", SpanOptions{
		Target: "list-sessions",
		Attrs: map[string]interface{}{
			"socket_path": "/tmp/test.sock",
		},
	})
	span.AddAttr("item_count", 2)
	span.End(nil)
	Close(RunResult{ExitCode: 1, ExitStatus: "error", Error: runErr})

	db, err := sql.Open("sqlite", ResolveSQLiteDebugPath(executablePath))
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	defer db.Close()

	var runCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&runCount); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if runCount != 1 {
		t.Fatalf("expected 1 run, got %d", runCount)
	}

	var exitStatus string
	var exitCode int
	var endedAt string
	if err := db.QueryRow(`SELECT exit_status, exit_code, ended_at FROM runs LIMIT 1`).Scan(&exitStatus, &exitCode, &endedAt); err != nil {
		t.Fatalf("query run status: %v", err)
	}
	if exitStatus != "error" {
		t.Fatalf("expected exit_status=error, got %q", exitStatus)
	}
	if exitCode != 1 {
		t.Fatalf("expected exit_code=1, got %d", exitCode)
	}
	if endedAt == "" {
		t.Fatalf("expected ended_at to be populated")
	}

	var menuEnterEvents int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE event_name = 'menu.enter'`).Scan(&menuEnterEvents); err != nil {
		t.Fatalf("count menu.enter events: %v", err)
	}
	if menuEnterEvents != 1 {
		t.Fatalf("expected 1 menu.enter event, got %d", menuEnterEvents)
	}

	var errorEvents int
	if err := db.QueryRow(`SELECT COUNT(*) FROM events WHERE event_name = 'error' AND level = 'error'`).Scan(&errorEvents); err != nil {
		t.Fatalf("count error events: %v", err)
	}
	if errorEvents != 1 {
		t.Fatalf("expected 1 error event, got %d", errorEvents)
	}

	var spanCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM spans WHERE component = 'tmux.control' AND operation = 'command'`).Scan(&spanCount); err != nil {
		t.Fatalf("count spans: %v", err)
	}
	if spanCount != 1 {
		t.Fatalf("expected 1 span, got %d", spanCount)
	}

	info, err := os.Stat(ResolveSQLiteDebugPath(executablePath))
	if err != nil {
		t.Fatalf("stat sqlite database: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected sqlite database mode 0600, got %03o", got)
	}
}
