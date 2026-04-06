package resurrect

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const defaultAutosaveStatusIcon = "💾"

var ErrAutoSaveLocked = errors.New("autosave lock busy")

type StatusConfig struct {
	SocketPath          string
	SaveDir             string
	CapturePaneContents bool
	IntervalMinutes     int
	Max                 int
	IconSeconds         int
	Icon                string
}

type autoSaveState struct {
	LastSuccess time.Time `json:"last_success"`
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

// RunAutoSave performs one autosave cycle: save, prune excess autosaves, then
// persist the success timestamp for future schedule/icon checks.
func RunAutoSave(cfg Config, max int) error {
	if max < 1 {
		max = 1
	}

	saveTime := autosaveNowFn()
	cfg.Kind = SaveKindAuto
	if cfg.Name == "" {
		cfg.Name = AutoSaveName(saveTime)
	}

	var saveErr error
	for event := range Save(cfg) {
		if event.Done {
			saveErr = event.Err
			break
		}
	}
	if saveErr != nil {
		return saveErr
	}

	if err := PruneAutoSaves(cfg.SaveDir, max); err != nil {
		return err
	}
	if err := WriteAutoSaveState(cfg.SaveDir, saveTime); err != nil {
		return err
	}
	return nil
}

// RunAutoSaveCommand is intended for tmux status-right #() usage. It acquires
// a singleton lifetime lock, waits until the next required event, writes
// status-line output over time, and exits after clearing the icon.
func RunAutoSaveCommand(cfg StatusConfig, output io.Writer) error {
	if cfg.IntervalMinutes <= 0 {
		return nil
	}

	err := withAutosaveLockFn(cfg.SaveDir, func() error {
		return runAutoSaveCommandLocked(cfg, output)
	})
	if errors.Is(err, ErrAutoSaveLocked) {
		return nil
	}
	return err
}

func runAutoSaveCommandLocked(cfg StatusConfig, output io.Writer) error {
	now := autosaveNowFn()
	lastSuccess, err := LastAutoSaveSuccess(cfg.SaveDir)
	if err != nil {
		return err
	}

	currentIcon := currentAutoSaveIcon(lastSuccess, now, cfg.IconSeconds, cfg.Icon)
	if err := writeAutoSaveLine(output, currentIcon); err != nil {
		return err
	}

	if currentIcon != "" {
		autosaveSleepFn(iconRemaining(lastSuccess, now, cfg.IconSeconds))
		return writeAutoSaveLine(output, "")
	}

	if sleepFor := timeUntilNextAutoSave(lastSuccess, now, cfg.IntervalMinutes); sleepFor > 0 {
		autosaveSleepFn(sleepFor)
	}

	if err := RunAutoSave(Config{
		SocketPath:          cfg.SocketPath,
		SaveDir:             cfg.SaveDir,
		CapturePaneContents: cfg.CapturePaneContents,
		Kind:                SaveKindAuto,
	}, cfg.Max); err != nil {
		return err
	}

	lastSuccess, err = LastAutoSaveSuccess(cfg.SaveDir)
	if err != nil {
		return err
	}
	now = autosaveNowFn()
	iconAfterSave := currentAutoSaveIcon(lastSuccess, now, cfg.IconSeconds, cfg.Icon)
	if iconAfterSave == "" {
		return nil
	}
	if err := writeAutoSaveLine(output, iconAfterSave); err != nil {
		return err
	}
	autosaveSleepFn(iconRemaining(lastSuccess, now, cfg.IconSeconds))
	return writeAutoSaveLine(output, "")
}

func resolveAutoSaveStatusIcon(icon string) string {
	if icon == "" {
		return defaultAutosaveStatusIcon
	}
	return icon
}

func LastAutoSaveSuccess(dir string) (time.Time, error) {
	state, err := ReadAutoSaveState(dir)
	switch {
	case err == nil && !state.LastSuccess.IsZero():
		return state.LastSuccess, nil
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return time.Time{}, err
	}

	entries, err := ListSaves(dir)
	if err != nil {
		return time.Time{}, err
	}
	for _, entry := range entries {
		if entry.Kind == SaveKindAuto {
			return entry.Timestamp, nil
		}
	}
	return time.Time{}, nil
}

func WriteAutoSaveState(dir string, ts time.Time) error {
	data, err := json.MarshalIndent(autoSaveState{LastSuccess: ts}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal autosave state: %w", err)
	}
	if err := os.WriteFile(autosaveStatePath(dir), data, 0o600); err != nil {
		return fmt.Errorf("write autosave state: %w", err)
	}
	return nil
}

func ReadAutoSaveState(dir string) (autoSaveState, error) {
	data, err := os.ReadFile(autosaveStatePath(dir))
	if err != nil {
		return autoSaveState{}, err
	}
	var state autoSaveState
	if err := json.Unmarshal(data, &state); err != nil {
		return autoSaveState{}, fmt.Errorf("parse autosave state: %w", err)
	}
	return state, nil
}

func autosaveStatePath(dir string) string {
	return filepath.Join(dir, ".autosave-state")
}

func autosaveLockPath(dir string) string {
	return filepath.Join(dir, ".autosave.lock")
}

func autoSaveDue(lastSuccess, now time.Time, intervalMinutes int) bool {
	if intervalMinutes <= 0 {
		return false
	}
	if lastSuccess.IsZero() {
		return true
	}
	return !now.Before(lastSuccess.Add(time.Duration(intervalMinutes) * time.Minute))
}

func currentAutoSaveIcon(lastSuccess, now time.Time, iconSeconds int, icon string) string {
	if iconSeconds <= 0 || lastSuccess.IsZero() {
		return ""
	}
	if now.Sub(lastSuccess) <= time.Duration(iconSeconds)*time.Second {
		return resolveAutoSaveStatusIcon(icon)
	}
	return ""
}

func timeUntilNextAutoSave(lastSuccess, now time.Time, intervalMinutes int) time.Duration {
	if autoSaveDue(lastSuccess, now, intervalMinutes) {
		return 0
	}
	return lastSuccess.Add(time.Duration(intervalMinutes) * time.Minute).Sub(now)
}

func iconRemaining(lastSuccess, now time.Time, iconSeconds int) time.Duration {
	if iconSeconds <= 0 || lastSuccess.IsZero() {
		return 0
	}
	remaining := lastSuccess.Add(time.Duration(iconSeconds) * time.Second).Sub(now)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func writeAutoSaveLine(output io.Writer, line string) error {
	_, err := fmt.Fprintln(output, line)
	return err
}
