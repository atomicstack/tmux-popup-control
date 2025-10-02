package logging

import (
	"fmt"
	"log"
	"os"
)

const logFile = "tmux-popup-control.log"

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
