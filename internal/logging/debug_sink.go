package logging

import (
	"path/filepath"
	"time"
)

// The types in this file are shared by both builds of the SQLite debug sink:
// the real one (sqlite.go, built with -tags sqlitedebug) and the stub
// (sqlite_stub.go, the default) that keeps modernc.org/sqlite out of release
// binaries.

// SQLiteRunInfo captures per-process metadata stored in the runs table.
type SQLiteRunInfo struct {
	Version        string
	ExecutablePath string
	CWD            string
	Args           []string
	Flags          map[string]string
	SocketPath     string
	RootMenu       string
	MenuArgs       string
	ClientID       string
	SessionName    string
}

// RunResult records how the process finished.
type RunResult struct {
	ExitCode   int
	ExitStatus string
	Error      error
}

type eventRecord struct {
	RunID     int64
	Seq       int64
	Time      time.Time
	Level     string
	Component string
	EventName string
	Message   string
	Attrs     map[string]any
}

type spanRecord struct {
	RunID         int64
	Seq           int64
	ParentSpanSeq int64
	StartedAt     time.Time
	EndedAt       time.Time
	Duration      time.Duration
	Component     string
	Operation     string
	Outcome       string
	Target        string
	Attrs         map[string]any
	ErrorText     string
}

// ResolveSQLiteDebugPath places the debug database next to the binary.
func ResolveSQLiteDebugPath(executablePath string) string {
	return filepath.Join(filepath.Dir(executablePath), filepath.Base(executablePath)+".debug.sqlite3")
}
