package main

import (
	"errors"
	"io"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/app"
	"github.com/atomicstack/tmux-popup-control/internal/config"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

func stubRunDeps(t *testing.T) *logging.RunResult {
	t.Helper()

	oldEnsure := ensureZeroExitOnHangupFn
	oldLoad := loadConfigFn
	oldValidate := validateConfigFn
	oldConfigure := configureLoggingFn
	oldTraceEnabled := setTraceEnabledFn
	oldSQLite := enableSQLiteDebugFn
	oldClose := closeLoggingFn
	oldTraceStartup := traceStartupFn
	oldAppRun := appRunFn
	oldLogError := logErrorFn

	var closed logging.RunResult
	ensureZeroExitOnHangupFn = func() {}
	configureLoggingFn = func(string) {}
	setTraceEnabledFn = func(bool) {}
	traceStartupFn = func(config.Config) {}
	enableSQLiteDebugFn = func(logging.SQLiteRunInfo) error { return nil }
	closeLoggingFn = func(result logging.RunResult) { closed = result }
	appRunFn = func(app.Config) error { return nil }
	logErrorFn = func(error) {}

	t.Cleanup(func() {
		ensureZeroExitOnHangupFn = oldEnsure
		loadConfigFn = oldLoad
		validateConfigFn = oldValidate
		configureLoggingFn = oldConfigure
		setTraceEnabledFn = oldTraceEnabled
		enableSQLiteDebugFn = oldSQLite
		closeLoggingFn = oldClose
		traceStartupFn = oldTraceStartup
		appRunFn = oldAppRun
		logErrorFn = oldLogError
	})

	return &closed
}

func TestRunWithDepsReturnsZeroForVersionRequest(t *testing.T) {
	closed := stubRunDeps(t)
	loadConfigFn = func() (config.Config, error) {
		return config.Config{}, config.ErrVersionRequested
	}

	if got := runWithDeps(MainDeps{}); got != 0 {
		t.Fatalf("runWithDeps() = %d, want 0", got)
	}
	if closed.ExitCode != 0 || closed.ExitStatus != "ok" || closed.Error != nil {
		t.Fatalf("unexpected close result: %+v", *closed)
	}
}

func TestRunWithDepsReturnsConfigErrorOnLoadFailure(t *testing.T) {
	closed := stubRunDeps(t)
	wantErr := errors.New("bad config")
	loadConfigFn = func() (config.Config, error) {
		return config.Config{}, wantErr
	}

	if got := runWithDeps(MainDeps{}); got != 2 {
		t.Fatalf("runWithDeps() = %d, want 2", got)
	}
	if closed.ExitStatus != "config_error" || !errors.Is(closed.Error, wantErr) {
		t.Fatalf("unexpected close result: %+v", *closed)
	}
}

func TestRunWithDepsReturnsConfigErrorOnValidationFailure(t *testing.T) {
	closed := stubRunDeps(t)
	wantErr := errors.New("invalid")
	loadConfigFn = func() (config.Config, error) {
		return config.Config{}, nil
	}
	validateConfigFn = func(config.Config) error { return wantErr }

	if got := runWithDeps(MainDeps{}); got != 2 {
		t.Fatalf("runWithDeps() = %d, want 2", got)
	}
	if closed.ExitStatus != "config_error" || !errors.Is(closed.Error, wantErr) {
		t.Fatalf("unexpected close result: %+v", *closed)
	}
}

func TestRunWithDepsReturnsSQLiteDebugError(t *testing.T) {
	closed := stubRunDeps(t)
	wantErr := errors.New("sqlite boom")
	loadConfigFn = func() (config.Config, error) {
		return config.Config{
			Logging: config.Logging{DebugToSQLite: true},
		}, nil
	}
	validateConfigFn = func(config.Config) error { return nil }
	enableSQLiteDebugFn = func(logging.SQLiteRunInfo) error { return wantErr }

	if got := runWithDeps(MainDeps{}); got != 2 {
		t.Fatalf("runWithDeps() = %d, want 2", got)
	}
	if closed.ExitStatus != "sqlite_debug_error" || !errors.Is(closed.Error, wantErr) {
		t.Fatalf("unexpected close result: %+v", *closed)
	}
}

func TestRunWithDepsReturnsSubcommandError(t *testing.T) {
	closed := stubRunDeps(t)
	wantErr := errors.New("autosave failed")
	loadConfigFn = func() (config.Config, error) {
		return config.Config{
			App:     app.Config{SocketPath: "sock"},
			Command: []string{"autosave"},
		}, nil
	}
	validateConfigFn = func(config.Config) error { return nil }

	deps := MainDeps{
		ResolveSocketPath:              func(string) (string, error) { return "sock", nil },
		ResolveSaveDir:                 func(string) (string, error) { return "/tmp/saves", nil },
		ResolvePaneContents:            func(string) bool { return false },
		ResolveAutosaveIntervalMinutes: func(string) int { return 5 },
		ResolveAutosaveMax:             func(string) int { return 10 },
		ResolveAutosaveIcon:            func(string) string { return "*" },
		ResolveAutosaveIconSeconds:     func(string) int { return 1 },
		RunAutoSaveCommand:             func(resurrect.StatusConfig, io.Writer) error { return wantErr },
	}

	if got := runWithDeps(deps); got != 1 {
		t.Fatalf("runWithDeps() = %d, want 1", got)
	}
	if closed.ExitStatus != "error" || !errors.Is(closed.Error, wantErr) {
		t.Fatalf("unexpected close result: %+v", *closed)
	}
}

func TestRunWithDepsReturnsAppError(t *testing.T) {
	closed := stubRunDeps(t)
	wantErr := errors.New("app failed")
	loadConfigFn = func() (config.Config, error) { return config.Config{}, nil }
	validateConfigFn = func(config.Config) error { return nil }
	var gotLogged error
	appRunFn = func(app.Config) error { return wantErr }
	logErrorFn = func(err error) { gotLogged = err }

	if got := runWithDeps(MainDeps{}); got != 1 {
		t.Fatalf("runWithDeps() = %d, want 1", got)
	}
	if !errors.Is(gotLogged, wantErr) {
		t.Fatalf("expected logging.Error to receive %v, got %v", wantErr, gotLogged)
	}
	if closed.ExitStatus != "error" || !errors.Is(closed.Error, wantErr) {
		t.Fatalf("unexpected close result: %+v", *closed)
	}
}
