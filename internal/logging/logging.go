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
	traceMu      sync.Mutex
	traceEnabled bool
	logPath      = defaultLogFile
)

// Error writes errors to the shared log file, mirroring the previous behaviour.
func Error(err error) {
	if err == nil {
		return
	}

	f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "logging failed: %v\n", ferr)
		return
	}
	defer f.Close()

	log.SetOutput(f)
	log.Println(err)
}

// SetTraceEnabled toggles emission of structured trace entries.
func SetTraceEnabled(enabled bool) {
	traceMu.Lock()
	traceEnabled = enabled
	traceMu.Unlock()
}

// Trace appends a structured JSON entry to the shared log when tracing is enabled.
func Trace(event string, payload interface{}) {
	traceMu.Lock()
	enabled := traceEnabled
	traceMu.Unlock()
	if !enabled {
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

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "trace logging failed: %v\n", err)
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	if err := enc.Encode(entry); err != nil {
		fmt.Fprintf(os.Stderr, "trace encoding failed: %v\n", err)
	}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "unable to create log directory: %v\n", err)
		logPath = defaultLogFile
		return
	}
	logPath = path
}
