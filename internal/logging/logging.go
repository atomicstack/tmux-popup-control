package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultLogFile = "tmux-popup-control.log"

var (
	traceMu      sync.RWMutex
	traceEnabled bool
	logPath      = defaultLogFile
	sqliteDebug  *sqliteSink
)

// SpanOptions configures a traced span.
type SpanOptions struct {
	Target string
	Attrs  map[string]interface{}
	Parent *Span
}

// Span records a timed operation in the SQLite debug sink when enabled.
type Span struct {
	enabled   bool
	sink      *sqliteSink
	seq       int64
	parentSeq int64
	started   time.Time
	component string
	operation string
	target    string
	attrs     map[string]interface{}
}

// Error writes errors to the shared log file, mirroring the previous behaviour.
func Error(err error) {
	if err == nil {
		return
	}

	traceMu.RLock()
	path := logPath
	sink := sqliteDebug
	traceMu.RUnlock()

	f, ferr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "logging failed: %v\n", ferr)
	} else {
		defer f.Close()
		logger := log.New(f, "", log.LstdFlags)
		logger.Println(err)
	}

	if sink != nil {
		sink.enqueueEvent(eventRecord{
			Time:      time.Now().UTC(),
			Level:     "error",
			Component: "app",
			EventName: "error",
			Message:   err.Error(),
			Attrs: map[string]interface{}{
				"error": err.Error(),
			},
		})
	}
}

// SetTraceEnabled toggles emission of structured trace entries.
func SetTraceEnabled(enabled bool) {
	traceMu.Lock()
	traceEnabled = enabled
	traceMu.Unlock()
}

// DebugSQLiteEnabled reports whether the SQLite debug sink is active.
func DebugSQLiteEnabled() bool {
	traceMu.RLock()
	defer traceMu.RUnlock()
	return sqliteDebug != nil
}

// EnableSQLiteDebug configures the SQLite debug sink for the current process.
func EnableSQLiteDebug(info SQLiteRunInfo) error {
	sink, err := openSQLiteSink(info)
	if err != nil {
		return err
	}

	traceMu.Lock()
	prev := sqliteDebug
	sqliteDebug = sink
	traceMu.Unlock()

	if prev != nil {
		prev.Close(RunResult{
			ExitStatus: "reconfigured",
		})
	}
	return nil
}

// Close flushes any active SQLite debug sink and marks the current run as ended.
func Close(result RunResult) {
	traceMu.Lock()
	sink := sqliteDebug
	sqliteDebug = nil
	traceMu.Unlock()

	if sink != nil {
		sink.Close(result)
	}
}

// Trace appends a structured JSON entry to the shared log when tracing is enabled
// and mirrors it into the SQLite debug sink when that is active.
func Trace(event string, payload interface{}) {
	traceMu.RLock()
	enabled := traceEnabled
	path := logPath
	sink := sqliteDebug
	traceMu.RUnlock()
	if !enabled && sink == nil {
		return
	}

	entry := struct {
		Time    time.Time   `json:"time"`
		Event   string      `json:"event"`
		Payload interface{} `json:"payload,omitempty"`
	}{
		Time:    time.Now().UTC(),
		Event:   event,
		Payload: payload,
	}

	if enabled {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "trace logging failed: %v\n", err)
		} else {
			defer f.Close()
			enc := json.NewEncoder(f)
			if err := enc.Encode(entry); err != nil {
				fmt.Fprintf(os.Stderr, "trace encoding failed: %v\n", err)
			}
		}
	}

	if sink != nil {
		sink.enqueueEvent(eventRecord{
			Time:      entry.Time,
			Level:     "trace",
			Component: eventComponent(event),
			EventName: event,
			Attrs:     payloadToAttrs(payload),
		})
	}
}

// StartSpan begins a timed operation for the SQLite debug sink.
func StartSpan(component, operation string, opts SpanOptions) *Span {
	traceMu.RLock()
	sink := sqliteDebug
	traceMu.RUnlock()
	if sink == nil {
		return &Span{}
	}

	span := &Span{
		enabled:   true,
		sink:      sink,
		seq:       sink.nextSeq(),
		started:   time.Now().UTC(),
		component: strings.TrimSpace(component),
		operation: strings.TrimSpace(operation),
		target:    strings.TrimSpace(opts.Target),
		attrs:     cloneAttrs(opts.Attrs),
	}
	if opts.Parent != nil {
		span.parentSeq = opts.Parent.seq
	}
	return span
}

// AddAttr appends a key/value pair to the span payload.
func (s *Span) AddAttr(key string, value interface{}) {
	if s == nil || !s.enabled || strings.TrimSpace(key) == "" {
		return
	}
	if s.attrs == nil {
		s.attrs = map[string]interface{}{}
	}
	s.attrs[key] = value
}

// AddAttrs appends multiple key/value pairs to the span payload.
func (s *Span) AddAttrs(attrs map[string]interface{}) {
	if s == nil || !s.enabled {
		return
	}
	for key, value := range attrs {
		s.AddAttr(key, value)
	}
}

// End records the span with an automatic ok/error outcome.
func (s *Span) End(err error) {
	if err != nil {
		s.EndWithOutcome("error", err)
		return
	}
	s.EndWithOutcome("ok", nil)
}

// EndWithOutcome records the span with an explicit outcome.
func (s *Span) EndWithOutcome(outcome string, err error) {
	if s == nil || !s.enabled || s.sink == nil {
		return
	}
	if strings.TrimSpace(outcome) == "" {
		outcome = "ok"
	}

	var errorText string
	if err != nil {
		errorText = err.Error()
	}

	endedAt := time.Now().UTC()
	s.sink.enqueueSpan(spanRecord{
		Seq:           s.seq,
		ParentSpanSeq: s.parentSeq,
		StartedAt:     s.started,
		EndedAt:       endedAt,
		Duration:      endedAt.Sub(s.started),
		Component:     s.component,
		Operation:     s.operation,
		Outcome:       outcome,
		Target:        s.target,
		Attrs:         cloneAttrs(s.attrs),
		ErrorText:     errorText,
	})
}

// Configure sets the log destination. Empty values fall back to the default
// path. Directories are created automatically when missing.
func Configure(path string) {
	traceMu.Lock()
	defer traceMu.Unlock()
	if strings.TrimSpace(path) == "" {
		logPath = defaultLogFile
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "unable to create log directory: %v\n", err)
		logPath = defaultLogFile
		return
	}
	logPath = path
}

func cloneAttrs(attrs map[string]interface{}) map[string]interface{} {
	if len(attrs) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(attrs))
	for key, value := range attrs {
		cloned[key] = value
	}
	return cloned
}

func eventComponent(event string) string {
	event = strings.TrimSpace(event)
	if event == "" {
		return "app"
	}
	if idx := strings.IndexByte(event, '.'); idx > 0 {
		return event[:idx]
	}
	return event
}

func payloadToAttrs(payload interface{}) map[string]interface{} {
	if payload == nil {
		return nil
	}

	switch value := payload.(type) {
	case map[string]interface{}:
		return cloneAttrs(value)
	case map[string]string:
		attrs := make(map[string]interface{}, len(value))
		for key, inner := range value {
			attrs[key] = inner
		}
		return attrs
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return map[string]interface{}{
			"value":        fmt.Sprintf("%v", payload),
			"marshalError": err.Error(),
		}
	}

	var decoded interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return map[string]interface{}{
			"value":          string(encoded),
			"unmarshalError": err.Error(),
		}
	}

	if attrs, ok := decoded.(map[string]interface{}); ok {
		return attrs
	}
	return map[string]interface{}{
		"value": decoded,
	}
}
