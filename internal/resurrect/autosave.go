package resurrect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

const defaultAutosaveStatusIcon = "💾"

// postSaveIconLinger is how long the autosave indicator stays on the status
// line after a save finishes, so a brief save still shows a visible flash.
const postSaveIconLinger = time.Second

var ErrAutoSaveLocked = errors.New("autosave lock busy")

// NowConfig configures one scheduled autosave.
type NowConfig struct {
	SocketPath          string
	SaveDir             string
	CapturePaneContents bool
	Max                 int
	Icon                string
}

// AutoSaveTmux is the tmux side of a scheduled autosave: advancing the
// monitor hook's schedule and toggling the status-line indicator.
type AutoSaveTmux struct {
	MarkStarted  func(socketPath string) error
	SetIndicator func(socketPath, icon string) error
}

var autoSaveTmux = AutoSaveTmux{
	MarkStarted:  tmux.MarkAutoSaveStarted,
	SetIndicator: tmux.SetAutoSaveIndicator,
}

var autosaveNowFn = time.Now

var autosaveSleepFn = time.Sleep

var withAutosaveLockFn = func(dir string, critical func() error) error {
	lockPath := autosaveLockPath(dir)
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("opening autosave lock: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return ErrAutoSaveLocked
		}
		return fmt.Errorf("locking autosave state: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	return critical()
}

func withAutosaveNowFn(fn func() time.Time) func() {
	orig := autosaveNowFn
	autosaveNowFn = fn
	return func() { autosaveNowFn = orig }
}

func withAutosaveSleepFn(fn func(time.Duration)) func() {
	orig := autosaveSleepFn
	autosaveSleepFn = fn
	return func() { autosaveSleepFn = orig }
}

func withWithAutosaveLockFn(fn func(string, func() error) error) func() {
	orig := withAutosaveLockFn
	withAutosaveLockFn = fn
	return func() { withAutosaveLockFn = orig }
}

// RunAutoSave performs one autosave: save, then prune excess autosaves.
func RunAutoSave(cfg Config, maxSaves int) error {
	maxSaves = max(maxSaves, 1)

	cfg.Kind = SaveKindAuto
	if cfg.Name == "" {
		cfg.Name = AutoSaveName(autosaveNowFn())
	}

	var saveErr error
	for event := range Save(context.Background(), cfg) {
		if event.Done {
			saveErr = event.Err
			break
		}
	}
	if saveErr != nil {
		return saveErr
	}
	return PruneAutoSaves(cfg.SaveDir, maxSaves)
}

// RunAutoSaveNow is run by the autosave monitor hook main.tmux installs, when
// the server has been up for an interval past the last save. It advances the
// schedule first, so a failed save waits one interval instead of leaving the
// monitor stuck true (it only fires on a change to true), then saves with the
// status indicator up. A save already in progress makes it a no-op.
func RunAutoSaveNow(cfg NowConfig) error {
	if err := autoSaveTmux.MarkStarted(cfg.SocketPath); err != nil {
		return fmt.Errorf("advancing autosave schedule: %w", err)
	}
	err := withAutosaveLockFn(cfg.SaveDir, func() error {
		icon := cfg.Icon
		if icon == "" {
			icon = defaultAutosaveStatusIcon
		}
		_ = autoSaveTmux.SetIndicator(cfg.SocketPath, icon)
		defer func() { _ = autoSaveTmux.SetIndicator(cfg.SocketPath, "") }()

		err := RunAutoSave(Config{
			SocketPath:          cfg.SocketPath,
			SaveDir:             cfg.SaveDir,
			CapturePaneContents: cfg.CapturePaneContents,
		}, cfg.Max)
		if err == nil {
			autosaveSleepFn(postSaveIconLinger)
		}
		return err
	})
	if errors.Is(err, ErrAutoSaveLocked) {
		return nil
	}
	return err
}

func autosaveLockPath(dir string) string {
	return filepath.Join(dir, ".autosave.lock")
}
