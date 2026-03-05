package trace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	writeMu     sync.Mutex
	snippetSize = 256

	writerOnce sync.Once
	writer     io.Writer
	writerFile *os.File

	rootOnce sync.Once
	rootDir  string
	rootErr  error
)

func shouldLog(component string) bool {
	val := strings.TrimSpace(os.Getenv("GOTMUXCC_TRACE"))
	if val == "" {
		return false
	}

	val = strings.ToLower(val)
	switch val {
	case "0", "false", "off", "no":
		return false
	case "1", "true", "yes", "on", "all", "*":
		return true
	}

	component = strings.ToLower(strings.TrimSpace(component))
	if component == "" {
		return true
	}

	parts := strings.Split(val, ",")
	for _, part := range parts {
		if strings.TrimSpace(part) == component {
			return true
		}
	}
	return false
}

func Enabled() bool {
	return shouldLog("")
}

func Printf(component, format string, args ...interface{}) {
	if !shouldLog(component) {
		return
	}

	msg := fmt.Sprintf(format, args...)
	now := time.Now().Format("15:04:05.000")

	var prefix string
	if component != "" {
		prefix = "[" + component + "] "
	}

	w := getWriter()

	writeMu.Lock()
	defer writeMu.Unlock()
	fmt.Fprintf(w, "[%s] %s%s\n", now, prefix, msg)
	if writerFile != nil {
		_ = writerFile.Sync()
	}
}

func Snippet(value string) string {
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\r")
	if len(value) > snippetSize {
		return value[:snippetSize] + "..."
	}
	return value
}

func getWriter() io.Writer {
	writerOnce.Do(func() {
		path := strings.TrimSpace(os.Getenv("GOTMUXCC_TRACE_FILE"))
		if path == "" {
			path = "gotmuxcc_trace.log"
		}

		resolved, err := resolvePath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[trace] failed to resolve trace path %q: %v\n", path, err)
			writer = os.Stdout
			return
		}

		file, err := os.OpenFile(resolved, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[trace] failed to open trace log %q: %v\n", resolved, err)
			writer = os.Stdout
			return
		}

		writerFile = file
		writer = file
	})
	return writer
}

func resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	root, err := projectRoot()
	if err != nil {
		wd, werr := os.Getwd()
		if werr != nil {
			return "", fmt.Errorf("unable to determine root: %w", err)
		}
		return filepath.Abs(filepath.Join(wd, path))
	}

	return filepath.Join(root, path), nil
}

func projectRoot() (string, error) {
	rootOnce.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			rootErr = fmt.Errorf("trace: unable to determine caller location")
			return
		}
		internalDir := filepath.Dir(file)
		root := filepath.Clean(filepath.Join(internalDir, "..", ".."))
		goMod := filepath.Join(root, "go.mod")
		info, err := os.Stat(goMod)
		if err != nil {
			rootErr = fmt.Errorf("trace: unable to stat go.mod: %w", err)
			return
		}
		if info.IsDir() {
			rootErr = fmt.Errorf("trace: go.mod is a directory at %q", goMod)
			return
		}
		rootDir = root
	})
	return rootDir, rootErr
}
