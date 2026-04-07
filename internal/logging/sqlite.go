package logging

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	sqliteSchemaVersion = 1
	sqliteQueueSize     = 2048
)

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

type closeRequest struct {
	Result RunResult
	Done   chan struct{}
}

type sqliteSink struct {
	path    string
	db      *sql.DB
	runID   int64
	queue   chan any
	seq     atomic.Int64
	dropped atomic.Int64

	closeOnce sync.Once
	writerWG  sync.WaitGroup
}

// ResolveSQLiteDebugPath places the debug database next to the binary.
func ResolveSQLiteDebugPath(executablePath string) string {
	return filepath.Join(filepath.Dir(executablePath), filepath.Base(executablePath)+".debug.sqlite3")
}

func openSQLiteSink(info SQLiteRunInfo) (*sqliteSink, error) {
	executablePath := strings.TrimSpace(info.ExecutablePath)
	if executablePath == "" {
		resolved, err := os.Executable()
		if err != nil {
			return nil, fmt.Errorf("resolve executable for sqlite debug path: %w", err)
		}
		executablePath = resolved
	}

	dbPath := ResolveSQLiteDebugPath(executablePath)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create sqlite debug directory: %w", err)
	}

	// Pre-create the database file with restricted permissions so it is
	// not world-readable (it stores runtime metadata including socket
	// paths, argv, and command output).
	if f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		f.Close()
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite debug database: %w", err)
	}

	if err := configureSQLite(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := ensureSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	runID, err := insertRun(db, info, executablePath)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	sink := &sqliteSink{
		path:  dbPath,
		db:    db,
		runID: runID,
		queue: make(chan any, sqliteQueueSize),
	}
	sink.writerWG.Add(1)
	go sink.writer()
	return sink, nil
}

func configureSQLite(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("configure sqlite debug database: %w", err)
		}
	}
	return nil
}

func ensureSQLiteSchema(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			pid INTEGER NOT NULL,
			ppid INTEGER NOT NULL,
			version TEXT NOT NULL,
			exe_path TEXT NOT NULL,
			cwd TEXT NOT NULL,
			argv_json TEXT NOT NULL,
			flags_json TEXT NOT NULL,
			socket_path TEXT NOT NULL,
			root_menu TEXT NOT NULL,
			menu_args TEXT NOT NULL,
			client_id TEXT NOT NULL,
			session_name TEXT NOT NULL,
			exit_code INTEGER,
			exit_status TEXT,
			error_text TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			seq INTEGER NOT NULL,
			ts TEXT NOT NULL,
			level TEXT NOT NULL,
			component TEXT NOT NULL,
			event_name TEXT NOT NULL,
			message TEXT NOT NULL,
			attrs_json TEXT NOT NULL,
			FOREIGN KEY (run_id) REFERENCES runs (id)
		)`,
		`CREATE TABLE IF NOT EXISTS spans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			seq INTEGER NOT NULL,
			parent_span_seq INTEGER,
			started_at TEXT NOT NULL,
			ended_at TEXT NOT NULL,
			duration_us INTEGER NOT NULL,
			component TEXT NOT NULL,
			operation TEXT NOT NULL,
			outcome TEXT NOT NULL,
			target TEXT NOT NULL,
			attrs_json TEXT NOT NULL,
			error_text TEXT NOT NULL,
			FOREIGN KEY (run_id) REFERENCES runs (id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_run_seq ON events (run_id, seq)`,
		`CREATE INDEX IF NOT EXISTS idx_events_component_name ON events (component, event_name)`,
		`CREATE INDEX IF NOT EXISTS idx_spans_run_seq ON spans (run_id, seq)`,
		`CREATE INDEX IF NOT EXISTS idx_spans_component_operation ON spans (component, operation)`,
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin sqlite schema setup: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("apply sqlite schema: %w", err)
		}
	}

	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		sqliteSchemaVersion,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("record sqlite schema migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite schema setup: %w", err)
	}
	return nil
}

func insertRun(db *sql.DB, info SQLiteRunInfo, executablePath string) (int64, error) {
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	argvJSON := mustJSON(info.Args)
	flagsJSON := mustJSON(info.Flags)
	version := strings.TrimSpace(info.Version)
	if version == "" {
		version = "dev"
	}
	cwd := info.CWD
	if cwd == "" {
		if resolved, err := os.Getwd(); err == nil {
			cwd = resolved
		}
	}
	res, err := db.Exec(
		`INSERT INTO runs (
			started_at, pid, ppid, version, exe_path, cwd, argv_json, flags_json,
			socket_path, root_menu, menu_args, client_id, session_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		startedAt,
		os.Getpid(),
		os.Getppid(),
		version,
		executablePath,
		cwd,
		argvJSON,
		flagsJSON,
		strings.TrimSpace(info.SocketPath),
		strings.TrimSpace(info.RootMenu),
		strings.TrimSpace(info.MenuArgs),
		strings.TrimSpace(info.ClientID),
		strings.TrimSpace(info.SessionName),
	)
	if err != nil {
		return 0, fmt.Errorf("insert sqlite debug run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("lookup sqlite debug run id: %w", err)
	}
	return runID, nil
}

func (s *sqliteSink) nextSeq() int64 {
	return s.seq.Add(1)
}

func (s *sqliteSink) enqueueEvent(record eventRecord) {
	if s == nil {
		return
	}
	record.RunID = s.runID
	record.Seq = s.nextSeq()
	if record.Time.IsZero() {
		record.Time = time.Now().UTC()
	}
	s.enqueue(record)
}

func (s *sqliteSink) enqueueSpan(record spanRecord) {
	if s == nil {
		return
	}
	record.RunID = s.runID
	s.enqueue(record)
}

func (s *sqliteSink) enqueue(op any) {
	select {
	case s.queue <- op:
	default:
		s.dropped.Add(1)
	}
}

func (s *sqliteSink) writer() {
	defer s.writerWG.Done()

	for {
		op := <-s.queue

		// Recover from panics in writeEvent/writeSpan so the writer
		// goroutine stays alive and Close() cannot deadlock waiting on
		// a closeRequest that will never be processed.
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "sqlite debug writer panic: %v\n", r)
				}
			}()

			if err := s.flushDropped(); err != nil {
				fmt.Fprintf(os.Stderr, "sqlite debug drop flush failed: %v\n", err)
			}

			switch value := op.(type) {
			case eventRecord:
				if err := s.writeEvent(value); err != nil {
					fmt.Fprintf(os.Stderr, "sqlite debug event write failed: %v\n", err)
				}
			case spanRecord:
				if err := s.writeSpan(value); err != nil {
					fmt.Fprintf(os.Stderr, "sqlite debug span write failed: %v\n", err)
				}
			case closeRequest:
				if err := s.flushDropped(); err != nil {
					fmt.Fprintf(os.Stderr, "sqlite debug final drop flush failed: %v\n", err)
				}
				if err := s.finishRun(value.Result); err != nil {
					fmt.Fprintf(os.Stderr, "sqlite debug run finalization failed: %v\n", err)
				}
				close(value.Done)
			}
		}()

		// Check if we just processed a close request.
		if _, ok := op.(closeRequest); ok {
			return
		}
	}
}

func (s *sqliteSink) flushDropped() error {
	dropped := s.dropped.Swap(0)
	if dropped == 0 {
		return nil
	}
	return s.writeEvent(eventRecord{
		RunID:     s.runID,
		Seq:       s.nextSeq(),
		Time:      time.Now().UTC(),
		Level:     "warn",
		Component: "logging",
		EventName: "debug.drop",
		Message:   "sqlite debug queue overflow",
		Attrs: map[string]any{
			"dropped_records": dropped,
		},
	})
}

func (s *sqliteSink) writeEvent(record eventRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO events (run_id, seq, ts, level, component, event_name, message, attrs_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.RunID,
		record.Seq,
		record.Time.UTC().Format(time.RFC3339Nano),
		emptyFallback(record.Level, "trace"),
		emptyFallback(record.Component, "app"),
		emptyFallback(record.EventName, "event"),
		record.Message,
		mustJSON(record.Attrs),
	)
	return err
}

func (s *sqliteSink) writeSpan(record spanRecord) error {
	var parent any
	if record.ParentSpanSeq > 0 {
		parent = record.ParentSpanSeq
	}

	_, err := s.db.Exec(
		`INSERT INTO spans (
			run_id, seq, parent_span_seq, started_at, ended_at, duration_us,
			component, operation, outcome, target, attrs_json, error_text
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.RunID,
		record.Seq,
		parent,
		record.StartedAt.UTC().Format(time.RFC3339Nano),
		record.EndedAt.UTC().Format(time.RFC3339Nano),
		record.Duration.Microseconds(),
		emptyFallback(record.Component, "app"),
		emptyFallback(record.Operation, "operation"),
		emptyFallback(record.Outcome, "ok"),
		record.Target,
		mustJSON(record.Attrs),
		record.ErrorText,
	)
	return err
}

func (s *sqliteSink) finishRun(result RunResult) error {
	exitStatus := strings.TrimSpace(result.ExitStatus)
	if exitStatus == "" {
		exitStatus = "ok"
	}
	var errorText any
	if result.Error != nil {
		errorText = result.Error.Error()
	}
	_, err := s.db.Exec(
		`UPDATE runs
		 SET ended_at = ?, exit_code = ?, exit_status = ?, error_text = ?
		 WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano),
		result.ExitCode,
		exitStatus,
		errorText,
		s.runID,
	)
	return err
}

func (s *sqliteSink) Close(result RunResult) {
	if s == nil {
		return
	}

	s.closeOnce.Do(func() {
		done := make(chan struct{})
		s.queue <- closeRequest{Result: result, Done: done}
		<-done
		s.writerWG.Wait()
		_ = s.db.Close()
	})
}

func mustJSON(value any) string {
	if value == nil {
		return "{}"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		fallback, _ := json.Marshal(map[string]any{
			"marshalError": err.Error(),
			"value":        fmt.Sprintf("%v", value),
		})
		return string(fallback)
	}
	if string(encoded) == "null" {
		return "{}"
	}
	return string(encoded)
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
