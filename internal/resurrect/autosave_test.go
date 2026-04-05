package resurrect

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func TestResolveAutosaveIntervalEnvVarOverridesTmuxOption(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_AUTOSAVE_INTERVAL_MINUTES", "7")
	restore := withTmuxOptionFn(func(_, opt string) string {
		if opt == "@tmux-popup-control-autosave-interval-minutes" {
			return "3"
		}
		return ""
	})
	defer restore()

	if got := ResolveAutosaveIntervalMinutes("dummy"); got != 7 {
		t.Fatalf("expected env override interval 7, got %d", got)
	}
}

func TestResolveAutosaveIntervalDisablesOnUnsetOrInvalid(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_AUTOSAVE_INTERVAL_MINUTES", "")
	restore := withTmuxOptionFn(func(_, opt string) string {
		if opt == "@tmux-popup-control-autosave-interval-minutes" {
			return "nope"
		}
		return ""
	})
	defer restore()

	if got := ResolveAutosaveIntervalMinutes("dummy"); got != 0 {
		t.Fatalf("expected disabled interval 0, got %d", got)
	}
}

func TestResolveAutosaveMaxUsesDefaultAndClampsMinimum(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_AUTOSAVE_MAX", "")
	restore := withTmuxOptionFn(func(_, opt string) string {
		switch opt {
		case "@tmux-popup-control-autosave-max":
			return "0"
		default:
			return ""
		}
	})
	defer restore()

	if got := ResolveAutosaveMax("dummy"); got != 1 {
		t.Fatalf("expected clamped autosave max 1, got %d", got)
	}
}

func TestResolveAutosaveIconSecondsUsesEnvAndDisablesOnUnsetOrInvalid(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_AUTOSAVE_ICON_SECONDS", "4")
	restore := withTmuxOptionFn(func(_, opt string) string {
		if opt == "@tmux-popup-control-autosave-icon-seconds" {
			return "9"
		}
		return ""
	})
	defer restore()

	if got := ResolveAutosaveIconSeconds("dummy"); got != 4 {
		t.Fatalf("expected env override icon seconds 4, got %d", got)
	}

	t.Setenv("TMUX_POPUP_CONTROL_AUTOSAVE_ICON_SECONDS", "")
	restore = withTmuxOptionFn(func(_, opt string) string {
		if opt == "@tmux-popup-control-autosave-icon-seconds" {
			return "bad"
		}
		return ""
	})
	defer restore()

	if got := ResolveAutosaveIconSeconds("dummy"); got != 0 {
		t.Fatalf("expected disabled icon seconds 0, got %d", got)
	}
}

func TestResolveAutosaveIconUsesEnvThenTmuxOptionThenDefault(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_AUTOSAVE_ICON", "X ")
	restore := withTmuxOptionFn(func(_, opt string) string {
		if opt == "@tmux-popup-control-autosave-icon" {
			return "Y "
		}
		return ""
	})
	defer restore()

	if got := ResolveAutosaveIcon("dummy"); got != "X " {
		t.Fatalf("expected env override icon %q, got %q", "X ", got)
	}

	t.Setenv("TMUX_POPUP_CONTROL_AUTOSAVE_ICON", "")
	restore = withTmuxOptionFn(func(_, opt string) string {
		if opt == "@tmux-popup-control-autosave-icon" {
			return "Y "
		}
		return ""
	})
	defer restore()

	if got := ResolveAutosaveIcon("dummy"); got != "Y " {
		t.Fatalf("expected tmux option icon %q, got %q", "Y ", got)
	}

	restore = withTmuxOptionFn(func(_, opt string) string { return "" })
	defer restore()
	if got := ResolveAutosaveIcon("dummy"); got != "💾" {
		t.Fatalf("expected default icon %q, got %q", "💾", got)
	}
}

func TestAutoSaveNameUsesISODateTime(t *testing.T) {
	ts := time.Date(2026, 4, 5, 16, 7, 8, 0, time.UTC)
	if got := AutoSaveName(ts); got != "auto-2026-04-05T16-07-08" {
		t.Fatalf("unexpected autosave name %q", got)
	}
}

func TestReadSaveFileDefaultsMissingKindToManual(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.json")
	data := []byte(`{
  "version": 1,
  "timestamp": "2026-04-05T16:07:08Z",
  "name": "legacy",
  "has_pane_contents": false,
  "client_session": "",
  "client_last_session": "",
  "sessions": []
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy save: %v", err)
	}

	sf, err := ReadSaveFile(path)
	if err != nil {
		t.Fatalf("ReadSaveFile: %v", err)
	}
	if sf.Kind != SaveKindManual {
		t.Fatalf("expected missing kind to default to manual, got %q", sf.Kind)
	}
}

func TestPruneAutoSavesRemovesOnlyOldAutoSaves(t *testing.T) {
	dir := t.TempDir()

	autoOldest := writeSaveFixture(t, dir, "auto-1", SaveKindAuto, time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC), true)
	autoMiddle := writeSaveFixture(t, dir, "auto-2", SaveKindAuto, time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC), true)
	autoNewest := writeSaveFixture(t, dir, "auto-3", SaveKindAuto, time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC), true)
	manual := writeSaveFixture(t, dir, "manual-1", SaveKindManual, time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC), true)

	if err := PruneAutoSaves(dir, 2); err != nil {
		t.Fatalf("PruneAutoSaves: %v", err)
	}

	if _, err := os.Stat(autoOldest); !os.IsNotExist(err) {
		t.Fatalf("expected oldest auto save to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(paneArchivePath(autoOldest)); !os.IsNotExist(err) {
		t.Fatalf("expected oldest auto archive to be removed, stat err=%v", err)
	}
	for _, path := range []string{autoMiddle, autoNewest, manual} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to remain, stat err=%v", filepath.Base(path), err)
		}
	}
}

func TestRunAutoSaveWritesAutoSaveUpdatesLastAndState(t *testing.T) {
	dir := t.TempDir()

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return makeSessions("main"), nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return makeWindows("main", 0), nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return makePanes("main", 0), nil })
	defer restoreFetchPanes()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (string, string) {
		return "", ""
	})
	defer restoreClientInfo()
	restoreNow := withAutosaveNowFn(func() time.Time {
		return time.Date(2026, 4, 5, 16, 7, 8, 0, time.UTC)
	})
	defer restoreNow()

	if err := RunAutoSave(Config{
		SocketPath: "/tmp/tmux.sock",
		SaveDir:    dir,
	}, 5); err != nil {
		t.Fatalf("RunAutoSave: %v", err)
	}

	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 autosave entry, got %d", len(entries))
	}
	entry := entries[0]
	if entry.Kind != SaveKindAuto {
		t.Fatalf("expected autosave kind %q, got %q", SaveKindAuto, entry.Kind)
	}
	if entry.Name != "auto-2026-04-05T16-07-08" {
		t.Fatalf("unexpected autosave name %q", entry.Name)
	}

	lastPath, err := LatestSave(dir)
	if err != nil {
		t.Fatalf("LatestSave: %v", err)
	}
	if lastPath != entry.Path {
		t.Fatalf("expected last symlink to point at %q, got %q", entry.Path, lastPath)
	}

	lastSuccess, err := LastAutoSaveSuccess(dir)
	if err != nil {
		t.Fatalf("LastAutoSaveSuccess: %v", err)
	}
	wantTime := time.Date(2026, 4, 5, 16, 7, 8, 0, time.UTC)
	if !lastSuccess.Equal(wantTime) {
		t.Fatalf("expected autosave state time %s, got %s", wantTime, lastSuccess)
	}
}

func TestRunAutoSavePrunesOlderAutoSavesButKeepsManualSaves(t *testing.T) {
	dir := t.TempDir()

	writeSaveFixture(t, dir, "auto-1", SaveKindAuto, time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC), true)
	writeSaveFixture(t, dir, "auto-2", SaveKindAuto, time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC), true)
	writeSaveFixture(t, dir, "manual-1", SaveKindManual, time.Date(2026, 4, 5, 9, 0, 0, 0, time.UTC), true)

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return makeSessions("main"), nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return makeWindows("main", 0), nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return makePanes("main", 0), nil })
	defer restoreFetchPanes()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	})
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (string, string) {
		return "", ""
	})
	defer restoreClientInfo()
	restoreNow := withAutosaveNowFn(func() time.Time {
		return time.Date(2026, 4, 5, 12, 30, 0, 0, time.UTC)
	})
	defer restoreNow()

	if err := RunAutoSave(Config{SaveDir: dir}, 2); err != nil {
		t.Fatalf("RunAutoSave: %v", err)
	}

	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}

	var autoCount, manualCount int
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name)
		switch entry.Kind {
		case SaveKindAuto:
			autoCount++
		case SaveKindManual:
			manualCount++
		}
	}
	if autoCount != 2 {
		t.Fatalf("expected 2 autosaves after pruning, got %d (%v)", autoCount, names)
	}
	if manualCount != 1 {
		t.Fatalf("expected manual save to remain, got %d (%v)", manualCount, names)
	}
}

func TestAutoSaveStatusRunsDueSaveAndReturnsIcon(t *testing.T) {
	dir := t.TempDir()

	restoreFetchSessions := withFetchSessionsFn(func(string) (tmux.SessionSnapshot, error) { return makeSessions("main"), nil })
	defer restoreFetchSessions()
	restoreFetchWindows := withFetchWindowsFn(func(string) (tmux.WindowSnapshot, error) { return makeWindows("main", 0), nil })
	defer restoreFetchWindows()
	restoreFetchPanes := withFetchPanesFn(func(string) (tmux.PaneSnapshot, error) { return makePanes("main", 0), nil })
	defer restoreFetchPanes()
	restoreWindowOpts := withQueryWindowOptionsFn(func(string) (map[string]bool, error) { return map[string]bool{}, nil })
	defer restoreWindowOpts()
	restoreClientInfo := withClientInfoFn(func(string, string) (string, string) { return "", "" })
	defer restoreClientInfo()
	restoreNow := withAutosaveNowFn(func() time.Time {
		return time.Date(2026, 4, 5, 16, 7, 8, 0, time.UTC)
	})
	defer restoreNow()
	restoreLock := withWithAutosaveLockFn(func(_ string, critical func() error) error {
		return critical()
	})
	defer restoreLock()

	status, err := AutoSaveStatus(StatusConfig{
		SocketPath:      "/tmp/tmux.sock",
		SaveDir:         dir,
		IntervalMinutes: 5,
		Max:             5,
		IconSeconds:     10,
		Icon:            "X ",
	})
	if err != nil {
		t.Fatalf("AutoSaveStatus: %v", err)
	}
	if status != "X " {
		t.Fatalf("expected autosave icon output, got %q", status)
	}
}

func TestAutoSaveStatusFallsBackToDefaultIcon(t *testing.T) {
	dir := t.TempDir()

	if err := WriteAutoSaveState(dir, time.Date(2026, 4, 5, 16, 7, 8, 0, time.UTC)); err != nil {
		t.Fatalf("WriteAutoSaveState: %v", err)
	}
	restoreNow := withAutosaveNowFn(func() time.Time {
		return time.Date(2026, 4, 5, 16, 7, 9, 0, time.UTC)
	})
	defer restoreNow()

	status, err := AutoSaveStatus(StatusConfig{
		SaveDir:         dir,
		IntervalMinutes: 5,
		Max:             5,
		IconSeconds:     10,
	})
	if err != nil {
		t.Fatalf("AutoSaveStatus: %v", err)
	}
	if status != "💾" {
		t.Fatalf("expected default autosave icon output, got %q", status)
	}
}

func TestAutoSaveStatusReturnsBlankWhenIconDisabled(t *testing.T) {
	dir := t.TempDir()

	if err := WriteAutoSaveState(dir, time.Date(2026, 4, 5, 16, 7, 8, 0, time.UTC)); err != nil {
		t.Fatalf("WriteAutoSaveState: %v", err)
	}
	restoreNow := withAutosaveNowFn(func() time.Time {
		return time.Date(2026, 4, 5, 16, 7, 9, 0, time.UTC)
	})
	defer restoreNow()

	status, err := AutoSaveStatus(StatusConfig{
		SaveDir:         dir,
		IntervalMinutes: 5,
		Max:             5,
		IconSeconds:     0,
	})
	if err != nil {
		t.Fatalf("AutoSaveStatus: %v", err)
	}
	if status != "" {
		t.Fatalf("expected blank autosave status, got %q", status)
	}
}

func TestAutoSaveStatusSkipsDueSaveWhenLockBusy(t *testing.T) {
	dir := t.TempDir()

	restoreNow := withAutosaveNowFn(func() time.Time {
		return time.Date(2026, 4, 5, 16, 7, 8, 0, time.UTC)
	})
	defer restoreNow()
	restoreLock := withWithAutosaveLockFn(func(string, func() error) error {
		return ErrAutoSaveLocked
	})
	defer restoreLock()

	status, err := AutoSaveStatus(StatusConfig{
		SaveDir:         dir,
		IntervalMinutes: 5,
		Max:             5,
		IconSeconds:     10,
	})
	if err != nil {
		t.Fatalf("AutoSaveStatus: %v", err)
	}
	if status != "" {
		t.Fatalf("expected blank autosave status when lock busy, got %q", status)
	}

	entries, err := ListSaves(dir)
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no autosaves when lock is busy, got %d", len(entries))
	}
}

func writeSaveFixture(t *testing.T, dir, name string, kind SaveKind, ts time.Time, withArchive bool) string {
	t.Helper()

	path := filepath.Join(dir, name+"_"+ts.Format("20060102T150405")+".json")
	sf := &SaveFile{
		Version:   currentVersion,
		Timestamp: ts,
		Name:      name,
		Kind:      kind,
	}
	if err := WriteSaveFile(path, sf); err != nil {
		t.Fatalf("WriteSaveFile(%s): %v", name, err)
	}
	if withArchive {
		archive := paneArchivePath(path)
		if err := os.WriteFile(archive, []byte("archive"), 0o600); err != nil {
			t.Fatalf("write archive for %s: %v", name, err)
		}
	}
	return path
}
