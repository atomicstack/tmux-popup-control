package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

const logFile = "tmux-popup-control.log"

var (
	traceMu       sync.Mutex
	traceEnabled  bool
)

// Error writes errors to the shared log file, mirroring the previous behaviour.
func Error(err error) {
	if err == nil {
		return
	}

	f, ferr := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
