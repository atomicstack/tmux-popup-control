# Save/Restore Sessions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add save and restore functionality for tmux sessions, windows, panes, and layouts — with a progress popup featuring a gradient progress bar and scrolling log.

**Architecture:** New `internal/resurrect/` package owns all save/restore logic (types, storage, orchestration). Progress is reported via a channel consumed by a new `ModeResurrect` Bubble Tea mode. Menu integration adds four items under `session`. CLI subcommands launch the progress popup via `tmux display-popup`.

**Tech Stack:** Go, Bubble Tea, lipgloss (24-bit colour), gotmuxcc (via `internal/tmux`), `archive/tar` + `compress/gzip` for pane contents.

**Spec:** `docs/superpowers/specs/2026-03-20-save-restore-design.md`

---

## File Map

### New files

| File | Responsibility |
|------|----------------|
| `internal/resurrect/types.go` | `SaveFile`, `Session`, `Window`, `Pane`, `ProgressEvent`, `Config`, `SaveEntry` structs |
| `internal/resurrect/storage.go` | `ResolveDir`, `ListSaves`, `LatestSave`, `savePath`, `updateLastSymlink`, JSON read/write |
| `internal/resurrect/storage_test.go` | Storage unit tests (dir resolution, file naming, symlink, listing) |
| `internal/resurrect/pane_contents.go` | `WritePaneArchive`, `ExtractPaneArchive` — tar.gz creation and extraction |
| `internal/resurrect/pane_contents_test.go` | Pane contents archive unit tests |
| `internal/resurrect/save.go` | `Save()` — spawns goroutine, discovery + save orchestration, sends progress events |
| `internal/resurrect/save_test.go` | Save orchestration unit tests (with tmux stubs) |
| `internal/resurrect/restore.go` | `Restore()` — spawns goroutine, parse + restore orchestration, sends progress events |
| `internal/resurrect/restore_test.go` | Restore orchestration unit tests (with tmux stubs) |
| `internal/ui/resurrect.go` | `resurrectState`, `logEntry`, message types, handlers, `readResurrectProgress` |
| `internal/ui/resurrect_view.go` | `resurrectView()` — log area + gradient progress bar rendering |
| `internal/ui/resurrect_test.go` | UI mode tests via `Harness` |
| `internal/menu/save_form.go` | `SaveForm` struct (name input, collision check, confirm-overwrite) |

### Modified files

| File | Changes |
|------|---------|
| `internal/resurrect/types.go` | (new — listed above) |
| `internal/tmux/restore.go` | (new) `CreateSession`, `CreateWindow`, `SplitPane`, `SelectLayoutTarget`, `CapturePaneContents` |
| `internal/tmux/restore_test.go` | (new) tests for the new tmux helpers |
| `internal/menu/menu.go` | Add `save`, `save-as`, `restore`, `restore-from` to `CategoryLoaders`, `ActionLoaders`, `ActionHandlers` |
| `internal/menu/session.go` | Add `loadSessionMenu` items, action handlers, action loader for `restore-from`, `SaveAsPrompt` message type |
| `internal/ui/model.go` | Add `ModeResurrect`, `ModeSessionSaveForm` to `Mode` enum; add `resurrectState *resurrectState` and `saveForm *menu.SaveForm` to `Model`; wire `handleActiveForm` and `registerHandlers` |
| `internal/ui/forms.go` | Add `handleSaveForm`, `startSaveForm` |
| `internal/ui/view.go` | Add `ModeResurrect` and `ModeSessionSaveForm` cases to `View()` |
| `internal/config/config.go` | Add `SessionStorageDir`, `RestorePaneContents` to config; add env var constants and parsing |
| `main.go` | Add `save-sessions` and `restore-sessions` subcommand dispatch |

---

## Task 1: Resurrect types

**Files:**
- Create: `internal/resurrect/types.go`

- [ ] **Step 1: Write types**

```go
// internal/resurrect/types.go
package resurrect

import "time"

const currentVersion = 1

// Config is passed to Save/Restore by the caller.
type Config struct {
	SocketPath          string
	SaveDir             string // resolved by caller via ResolveDir()
	CapturePaneContents bool
	Name                string // empty for auto-timestamped
}

// SaveFile is the top-level JSON structure written to disk.
type SaveFile struct {
	Version           int       `json:"version"`
	Timestamp         time.Time `json:"timestamp"`
	Name              string    `json:"name"`
	HasPaneContents   bool      `json:"has_pane_contents"`
	ClientSession     string    `json:"client_session"`
	ClientLastSession string    `json:"client_last_session"`
	Sessions          []Session `json:"sessions"`
}

// Session represents one tmux session in the save file.
type Session struct {
	Name     string   `json:"name"`
	Created  int64    `json:"created"`
	Attached bool     `json:"attached"`
	Windows  []Window `json:"windows"`
}

// Window represents one tmux window in the save file.
type Window struct {
	Index           int    `json:"index"`
	Name            string `json:"name"`
	Layout          string `json:"layout"`
	Active          bool   `json:"active"`
	Alternate       bool   `json:"alternate"`
	AutomaticRename bool   `json:"automatic_rename"`
	Panes           []Pane `json:"panes"`
}

// Pane represents one tmux pane in the save file.
type Pane struct {
	Index      int    `json:"index"`
	WorkingDir string `json:"working_dir"`
	Title      string `json:"title"`
	Command    string `json:"command"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Active     bool   `json:"active"`
}

// ProgressEvent is sent on the channel during save/restore.
type ProgressEvent struct {
	Step    int
	Total   int
	Message string
	Kind    string // "session", "window", "pane", "info", "error"
	ID      string // entity name/ID for UI colouring
	Done    bool
	Err     error
}

// SaveEntry represents one save file in the listing.
type SaveEntry struct {
	Path            string
	Name            string // snapshot name or empty for auto
	Timestamp       time.Time
	HasPaneContents bool
	Size            int64
	SessionCount    int
}
```

- [ ] **Step 2: Verify it compiles**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go build ./internal/resurrect/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/resurrect/types.go
git commit -m "feat(resurrect): add save/restore type definitions"
```

---

## Task 2: Storage — directory resolution, file naming, JSON I/O, symlink

**Files:**
- Create: `internal/resurrect/storage.go`
- Create: `internal/resurrect/storage_test.go`

- [ ] **Step 1: Write failing tests for ResolveDir**

Test the four-step lookup chain: env var → tmux option → XDG_DATA_HOME → HOME fallback. Use `t.Setenv` for env var control. Mock the tmux option query via an injectable package-level var.

```go
// internal/resurrect/storage_test.go
package resurrect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDirEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", dir)
	got, err := ResolveDir("")
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
}

func TestResolveDirXDG(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", "")
	defer withTmuxOptionFn(func(string, string) (string, error) {
		return "", nil
	})()
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	got, err := ResolveDir("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "tmux-popup-control-sessions")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveDirHomeFallback(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR", "")
	defer withTmuxOptionFn(func(string, string) (string, error) {
		return "", nil
	})()
	t.Setenv("XDG_DATA_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := ResolveDir("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "tmux-popup-control-sessions")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestResolveDir -v`
Expected: FAIL — `ResolveDir` not defined

- [ ] **Step 3: Implement storage.go**

```go
// internal/resurrect/storage.go
package resurrect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// tmuxOptionFn queries a tmux option. Injectable for testing.
var tmuxOptionFn = defaultTmuxOptionFn

func withTmuxOptionFn(fn func(string, string) (string, error)) func() {
	orig := tmuxOptionFn
	tmuxOptionFn = fn
	return func() { tmuxOptionFn = orig }
}

func defaultTmuxOptionFn(socketPath, option string) (string, error) {
	// Calls tmux show-option -gqv to read the option.
	// Uses internal/tmux.DisplayMessage or client.Command.
	// Implementation deferred to wiring step — for now returns empty.
	return "", nil
}

// ResolveDir returns the save directory using the lookup chain:
// 1. TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR env var
// 2. @tmux-popup-control-session-storage-dir tmux option
// 3. $XDG_DATA_HOME/tmux-popup-control-sessions/
// 4. $HOME/tmux-popup-control-sessions/
func ResolveDir(socketPath string) (string, error) {
	if dir := os.Getenv("TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR"); dir != "" {
		return ensureDir(dir)
	}
	if val, err := tmuxOptionFn(socketPath, "@tmux-popup-control-session-storage-dir"); err == nil && val != "" {
		return ensureDir(val)
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return ensureDir(filepath.Join(xdg, "tmux-popup-control-sessions"))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve save dir: %w", err)
	}
	return ensureDir(filepath.Join(home, "tmux-popup-control-sessions"))
}

func ensureDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create save dir %s: %w", dir, err)
	}
	return dir, nil
}

// savePath returns the full path for a save file.
func savePath(dir, name string) string {
	if name != "" {
		return filepath.Join(dir, name+".json")
	}
	ts := time.Now().UTC().Format("20060102T150405")
	return filepath.Join(dir, "save_"+ts+".json")
}

// paneArchivePath derives the archive path from the JSON path.
func paneArchivePath(jsonPath string) string {
	return strings.TrimSuffix(jsonPath, ".json") + ".panes.tar.gz"
}

// updateLastSymlink points the "last" symlink to the given save file.
// Only called for auto-timestamped saves, not named snapshots.
func updateLastSymlink(dir, target string) error {
	link := filepath.Join(dir, "last")
	_ = os.Remove(link)
	return os.Symlink(target, link)
}

// WriteSaveFile writes the SaveFile as JSON to the given path.
func WriteSaveFile(path string, sf *SaveFile) error {
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal save file: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// ReadSaveFile reads and parses a JSON save file.
func ReadSaveFile(path string) (*SaveFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read save file: %w", err)
	}
	var sf SaveFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, fmt.Errorf("parse save file: %w", err)
	}
	return &sf, nil
}

// LatestSave resolves the "last" symlink and returns the path it points to.
func LatestSave(dir string) (string, error) {
	link := filepath.Join(dir, "last")
	target, err := os.Readlink(link)
	if err != nil {
		return "", fmt.Errorf("no saved session found")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	if _, err := os.Stat(target); err != nil {
		return "", fmt.Errorf("saved session file missing: %s", target)
	}
	return target, nil
}

// ListSaves returns all save files in the directory, sorted newest-first.
func ListSaves(dir string) ([]SaveEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var saves []SaveEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		sf, err := ReadSaveFile(path)
		if err != nil {
			continue // skip unparseable files
		}
		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		saves = append(saves, SaveEntry{
			Path:            path,
			Name:            sf.Name,
			Timestamp:       sf.Timestamp,
			HasPaneContents: sf.HasPaneContents,
			Size:            size,
			SessionCount:    len(sf.Sessions),
		})
	}
	sort.Slice(saves, func(i, j int) bool {
		return saves[i].Timestamp.After(saves[j].Timestamp)
	})
	return saves, nil
}

// SaveFileExists checks whether a named snapshot already exists.
func SaveFileExists(dir, name string) bool {
	_, err := os.Stat(savePath(dir, name))
	return err == nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestResolveDir -v`
Expected: PASS

- [ ] **Step 5: Write additional storage tests**

Add tests for `WriteSaveFile`/`ReadSaveFile` round-trip, `LatestSave` (with and without symlink), `ListSaves` ordering, `SaveFileExists`, `savePath` naming, and `paneArchivePath`.

- [ ] **Step 6: Run all storage tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/resurrect/storage.go internal/resurrect/storage_test.go
git commit -m "feat(resurrect): add storage layer — dir resolution, file I/O, symlinks"
```

---

## Task 3: Pane contents — tar.gz archive creation and extraction

**Files:**
- Create: `internal/resurrect/pane_contents.go`
- Create: `internal/resurrect/pane_contents_test.go`

- [ ] **Step 1: Write failing test for round-trip**

Test: create archive from a map of pane ID → content, extract to temp dir, verify files match.

- [ ] **Step 2: Run test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestPaneArchive -v`
Expected: FAIL

- [ ] **Step 3: Implement pane_contents.go**

`WritePaneArchive(path string, contents map[string]string) error` — creates a `.tar.gz` with one entry per pane (key is filename like `dev:0.1`, value is content).

`ExtractPaneArchive(archivePath, destDir string) error` — extracts to a directory, one file per pane.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestPaneArchive -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/resurrect/pane_contents.go internal/resurrect/pane_contents_test.go
git commit -m "feat(resurrect): add tar.gz pane contents archive support"
```

---

## Task 4: New tmux helpers for restore

**Files:**
- Create: `internal/tmux/restore.go`
- Create: `internal/tmux/restore_test.go`

- [ ] **Step 1: Write failing tests**

Test each new helper with `withStubTmux` and `fakeClient`. Verify the correct `Command()` args are passed. Helpers: `CreateSession`, `CreateWindow`, `RenameWindow`, `SplitPane`, `SelectLayoutTarget`, `SelectPane`, `SelectWindow`, `SendPaneContents`, `CapturePaneContents`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/tmux/ -run TestRestore -v`
Expected: FAIL

- [ ] **Step 3: Implement restore.go**

Each function follows the pattern: `newTmux(socketPath)` → `client.Command(...)`. `CapturePaneContents` uses `client.CapturePane(target, &gotmux.CaptureOptions{PreserveTrailingSpace: true})` — no escape sequences, no start-line limit.

```go
// internal/tmux/restore.go
package tmux

import (
	"fmt"
	"os"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc"
)

func CreateSession(socketPath, name, dir string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.NewSession(&gotmux.SessionOptions{
		Name:           name,
		StartDirectory: dir,
	})
	return err
}

func CreateWindow(socketPath, session string, index int, name, dir string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	target := fmt.Sprintf("%s:%d", session, index)
	_, err = client.Command("new-window", "-t", target, "-n", name, "-c", dir, "-d")
	return err
}

func RenameWindow(socketPath, target, name string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("rename-window", "-t", strings.TrimSpace(target), name)
	return err
}

func SplitPane(socketPath, target, dir string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("split-window", "-t", strings.TrimSpace(target), "-c", dir, "-d")
	return err
}

func SelectLayoutTarget(socketPath, target, layout string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("select-layout", "-t", strings.TrimSpace(target), layout)
	return err
}

func SelectPane(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("select-pane", "-t", strings.TrimSpace(target))
	return err
}

func SelectWindow(socketPath, target string) error {
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	_, err = client.Command("select-window", "-t", strings.TrimSpace(target))
	return err
}

func SendPaneContents(socketPath, target, contents string) error {
	// Write contents to a temp file, then load-buffer + paste-buffer.
	// send-keys would interpret special characters; load-buffer is literal.
	// Cannot pipe via stdin through control-mode Command(), so use a file.
	f, err := os.CreateTemp("", "tmux-pane-contents-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(contents); err != nil {
		f.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	f.Close()

	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	if _, err := client.Command("load-buffer", f.Name()); err != nil {
		return fmt.Errorf("load-buffer: %w", err)
	}
	_, err = client.Command("paste-buffer", "-t", strings.TrimSpace(target), "-d")
	return err
}

func CapturePaneContents(socketPath, target string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	return client.CapturePane(strings.TrimSpace(target), &gotmux.CaptureOptions{
		PreserveTrailingSpace: true,
	})
}
```

**Note on SendPaneContents:** Uses `load-buffer` + `paste-buffer` rather than `send-keys` because `send-keys` interprets special characters (Enter, Tab, etc.) and would corrupt the content. Content is written to a temp file first because gotmuxcc's `Command()` sends arguments as a command string over control-mode and cannot pipe data to stdin. `paste-buffer -d` pastes then deletes the buffer. The temp file is cleaned up immediately after use.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/tmux/ -run TestRestore -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tmux/restore.go internal/tmux/restore_test.go
git commit -m "feat(tmux): add restore helpers — CreateSession, CreateWindow, SplitPane, SelectLayoutTarget, CapturePaneContents"
```

---

## Task 5: Save orchestration

**Files:**
- Create: `internal/resurrect/save.go`
- Create: `internal/resurrect/save_test.go`

- [ ] **Step 1: Write failing test for Save**

Test that `Save()` returns a channel and emits the correct sequence of progress events: discovery event (step 0, total N), then one event per session, per window, per pane (if contents enabled), write JSON, write archive (if contents), update symlink, and finally done. Use injectable tmux functions — add package-level vars to `save.go` for `fetchSessionsFn`, `fetchWindowsFn`, `fetchPanesFn`, `capturePaneContentsFn` that default to the real `tmux.*` functions.

- [ ] **Step 2: Run test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestSave -v`
Expected: FAIL

- [ ] **Step 3: Implement save.go**

`Save(cfg Config) <-chan ProgressEvent` — creates channel, spawns goroutine:
1. Discovery: fetch sessions/windows/panes, compute total.
2. Send discovery event `{Step: 0, Total: N, Message: "discovering sessions...", Kind: "info"}`.
3. For each session: build `Session` struct, send progress.
4. For each window: query layout via `DisplayMessage`, build `Window` struct, send progress.
5. If pane contents: for each pane, call `CapturePaneContents`, collect into map, send progress.
6. Assemble `SaveFile`, call `WriteSaveFile`, send progress.
7. If pane contents: call `WritePaneArchive`, send progress.
8. If not a named snapshot: call `updateLastSymlink`, send progress.
9. Send final done event with summary message.

On any error: send `ProgressEvent{Err: err, Done: true}` and return.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestSave -v`
Expected: PASS

- [ ] **Step 5: Write edge case tests**

- Save with no sessions (empty tmux server)
- Save with pane contents disabled (no archive written)
- Named snapshot (no symlink update)
- Error during capture (verify error event and early stop)

- [ ] **Step 6: Run all tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/resurrect/save.go internal/resurrect/save_test.go
git commit -m "feat(resurrect): add save orchestration with progress events"
```

---

## Task 6: Restore orchestration

**Files:**
- Create: `internal/resurrect/restore.go`
- Create: `internal/resurrect/restore_test.go`

- [ ] **Step 1: Write failing test for Restore**

Test that `Restore()` reads a JSON save file, emits correct progress events (create session, create window, create pane, apply layout, restore contents, set active pane, set active window, restore client, cleanup), and sends done. Use injectable tmux functions — add package-level vars for `createSessionFn`, `createWindowFn`, `renameWindowFn`, `splitPaneFn`, `selectLayoutTargetFn`, `selectPaneFn`, `selectWindowFn`, `sendPaneContentsFn`, `switchClientFn`, `existingSessionsFn`.

- [ ] **Step 2: Run test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestRestore -v`
Expected: FAIL

- [ ] **Step 3: Implement restore.go**

`Restore(cfg Config, file string) <-chan ProgressEvent` — creates channel, spawns goroutine:
1. Read and parse JSON save file.
2. Check for companion tar.gz; if present, extract to temp dir.
3. Fetch existing sessions to detect conflicts.
4. Compute total (accounting for all steps in spec).
5. Send discovery event.
6. For each session: check conflict (skip + warn if exists, still increment steps); else call `createSessionFn` (wraps `tmux.CreateSession`). The first window and pane are auto-created by tmux when creating a session.
7. For each window: if index 0, call `renameWindowFn` (wraps `tmux.RenameWindow`) to rename the auto-created window. Otherwise call `createWindowFn` (wraps `tmux.CreateWindow`) with `-d` flag.
8. For each pane: if index 0, it's the auto-created pane — skip creation. Otherwise call `splitPaneFn` (wraps `tmux.SplitPane`) targeting the window.
9. For each window: call `selectLayoutTargetFn` (wraps `tmux.SelectLayoutTarget`) to apply the saved layout string.
10. If pane contents: for each pane, read content from temp dir file, call `sendPaneContentsFn` (wraps `tmux.SendPaneContents` which uses `load-buffer` + `paste-buffer`).
11. For each window: call `selectPaneFn` (wraps `tmux.SelectPane`) targeting the saved active pane.
12. For each session: call `selectWindowFn` (wraps `tmux.SelectWindow`) targeting the saved active window.
13. Restore client session via `switchClientFn` (wraps the existing `tmux.SwitchClient`).
14. Clean up temp dir.
15. Send done event.

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -run TestRestore -v`
Expected: PASS

- [ ] **Step 5: Write edge case tests**

- Restore with session conflict (skip + warn, steps still consumed)
- Restore with no pane contents archive
- Restore from named snapshot
- Error during session creation (verify error event)

- [ ] **Step 6: Run all tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/resurrect/restore.go internal/resurrect/restore_test.go
git commit -m "feat(resurrect): add restore orchestration with conflict handling"
```

---

## Task 7: Config — env vars and tmux options

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Read current config.go**

Read `internal/config/config.go` to identify where to add the new constants and parsing.

- [ ] **Step 2: Add env var constants**

Add to the `const` block:

```go
envSessionStorageDir    = "TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR"
envRestorePaneContents  = "TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS"
```

- [ ] **Step 3: Add fields to both Config structs**

Add `SessionStorageDir string` and `RestorePaneContents bool` to `app.Config` in `internal/app/app.go` (this is the struct that flows to `ui.NewModel` and menu handlers). In `internal/config/config.go`, read the new env vars in `LoadArgs` and populate `cfg.App.SessionStorageDir` and `cfg.App.RestorePaneContents`. The tmux option fallback is handled by `resurrect.ResolveDir` at runtime, not at config load time.

- [ ] **Step 4: Verify it compiles**

Run: `make build`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/app/app.go
git commit -m "feat(config): add session storage dir and pane contents env vars"
```

---

## Task 8: Progress UI mode — state, messages, handlers

**Files:**
- Create: `internal/ui/resurrect.go`
- Modify: `internal/ui/model.go`

- [ ] **Step 1: Add mode constants to model.go**

Add `ModeResurrect` and `ModeSessionSaveForm` to the `Mode` iota in `internal/ui/model.go` (after `ModePluginInstall`).

- [ ] **Step 2: Write resurrect.go — state and message types**

```go
// internal/ui/resurrect.go
package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

type resurrectState struct {
	operation string // "save" or "restore"
	progress  <-chan resurrect.ProgressEvent
	log       []logEntry
	step      int
	total     int
	done      bool
	err       error
}

type logEntry struct {
	message string
	kind    string // "session", "window", "pane", "info", "error"
	id      string
}

// ResurrectStart triggers mode transition from a menu action.
type ResurrectStart struct {
	Operation string // "save" or "restore"
	Name      string // snapshot name (save-as only)
	SaveFile  string // path to restore from
	Config    resurrect.Config
}

type resurrectProgressMsg struct {
	event resurrect.ProgressEvent
}

type resurrectTickMsg struct{}
```

- [ ] **Step 3: Write handlers**

In `resurrect.go`, add:

- `handleResurrectStartMsg` — initialises `resurrectState`, calls `resurrect.Save()` or `resurrect.Restore()`, returns `readResurrectProgress` command.
- `readResurrectProgress` — reads one event from channel, wraps as `resurrectProgressMsg`. Returns nil if channel is closed.
- `handleResurrectProgressMsg` — appends to log, updates step/total. If done, starts 1s tick. If error, sets err. Otherwise returns another `readResurrectProgress`.
- `handleResurrectKey` — for `handleActiveForm`: while running, consume all keys. On error/done, any key quits.

- [ ] **Step 4: Wire into model.go**

Add `resurrectState *resurrectState` field to `Model` struct.
Add `ModeResurrect` case to `handleActiveForm`.
Add `ModeSessionSaveForm` case to `handleActiveForm`.
Register `ResurrectStart` and `resurrectProgressMsg` in `registerHandlers`.

- [ ] **Step 5: Verify it compiles**

Run: `make build`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add internal/ui/resurrect.go internal/ui/model.go
git commit -m "feat(ui): add ModeResurrect state, messages, and handlers"
```

---

## Task 9: Progress UI view — gradient progress bar and log rendering

**Files:**
- Create: `internal/ui/resurrect_view.go`
- Modify: `internal/ui/view.go`

- [ ] **Step 1: Implement resurrect_view.go**

`resurrectView()` renders:
- Log area: last N lines that fit in `m.height - 2` rows. Each line applies hierarchical purple colouring based on `kind` and `id`:
  - Session names: `#b388ff`
  - Window names/IDs: `#ce93d8`
  - Pane IDs: `#e1bee7`
  - Error: `styles.Error`
- Progress bar (last line): character-by-character 24-bit gradient.
  - Save: white `#ffffff` → purple `#7c4dff`
  - Restore: purple `#7c4dff` → white `#ffffff`
  - Empty portion: `styles.ProgressEmpty`
  - Counter: step in `#7c4dff`, `/` dim, total in `#777777`
- Before first event (`total == 0`): show "discovering..." in log, dimmed empty bar.

Use `lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b)))` for per-character gradient rendering. Interpolate RGB linearly across the filled width.

- [ ] **Step 2: Wire into view.go**

Add `ModeResurrect` case to the `View()` method's switch statement. Add `ModeSessionSaveForm` case (delegates to form view — done in task 11).

- [ ] **Step 3: Verify it compiles**

Run: `make build`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/ui/resurrect_view.go internal/ui/view.go
git commit -m "feat(ui): add gradient progress bar and log rendering for resurrect mode"
```

---

## Task 10: UI tests for resurrect mode

**Files:**
- Create: `internal/ui/resurrect_test.go`

- [ ] **Step 1: Write test for progress event flow**

Use `Harness` to drive the model. Send a `ResurrectStart` message, then simulate progress events by directly calling the handler (or using a pre-loaded channel). Verify:
- Mode transitions to `ModeResurrect`
- Log entries accumulate
- Step/total update correctly
- Done state triggers tick

- [ ] **Step 2: Write test for key handling during progress**

Send key messages while in `ModeResurrect` running state — verify they are consumed. Send key after error — verify quit.

- [ ] **Step 3: Write test for view rendering**

Verify `View()` output contains progress bar characters, log lines, gradient colouring (check for ANSI escape sequences with expected RGB values).

- [ ] **Step 4: Run tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/ -run TestResurrect -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ui/resurrect_test.go
git commit -m "test(ui): add resurrect mode unit tests"
```

---

## Task 11: Save-as form

**Files:**
- Create: `internal/menu/save_form.go`
- Modify: `internal/ui/forms.go`
- Modify: `internal/menu/session.go` (add `SaveAsPrompt` type)

- [ ] **Step 1: Write SaveForm**

Model after the existing `SessionForm` in `internal/menu/session.go`. The `SaveForm` struct has:
- `input textinput.Model` — text input for snapshot name
- `saveDir string` — resolved save directory
- `ctx Context`
- `err string`
- `confirmOverwrite bool` — set true when collision detected on first enter

`Update` handles:
- Esc → cancel
- Enter → if name empty, show error. If file exists and `!confirmOverwrite`, show info notice "snapshot 'X' already exists — enter to overwrite", set `confirmOverwrite = true`. If `confirmOverwrite` or no collision, return done.
- Character input → reset `confirmOverwrite`, forward to textinput

- [ ] **Step 2: Add SaveAsPrompt message type in session.go**

```go
type SaveAsPrompt struct {
	Context Context
	SaveDir string
}
```

- [ ] **Step 3: Wire into forms.go**

Add `handleSaveForm` (like `handleSessionForm`), `startSaveForm` (like `startSessionForm`). Add `saveForm *menu.SaveForm` field to `Model` in `model.go`. Add `ModeSessionSaveForm` case in `handleActiveForm`. Register `SaveAsPrompt` in `registerHandlers`.

- [ ] **Step 4: Write tests for SaveForm**

Test: empty name rejected, collision shows info notice, second enter confirms, escape cancels.

- [ ] **Step 5: Run tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/ -run TestSaveForm -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/menu/save_form.go internal/ui/forms.go internal/ui/model.go internal/menu/session.go
git commit -m "feat(menu): add save-as form with collision detection"
```

---

## Task 12: Menu integration — register save/restore items

**Files:**
- Modify: `internal/menu/menu.go`
- Modify: `internal/menu/session.go`

- [ ] **Step 1: Add menu items to loadSessionMenu**

In `internal/menu/session.go`, add `"save"`, `"save-as"`, `"restore"`, `"restore-from"` to the items slice in `loadSessionMenu`.

- [ ] **Step 2: Add action handlers**

In `internal/menu/session.go`, add:

- `SessionSaveAction(ctx, item)` — resolves save dir, returns `ResurrectStart{Operation: "save", Config: ...}`.
- `SessionSaveAsAction(ctx, item)` — resolves save dir, returns `SaveAsPrompt{Context: ctx, SaveDir: dir}`.
- `SessionRestoreAction(ctx, item)` — resolves save dir, calls `resurrect.LatestSave(dir)`. If error, returns `ActionResult{Err: ...}`. If ok, returns `ResurrectStart{Operation: "restore", SaveFile: path, Config: ...}`.
- `SessionRestoreFromAction(ctx, item)` — returns `ResurrectStart{Operation: "restore", SaveFile: item.ID, Config: ...}`.

- [ ] **Step 3: Add action loader for restore-from**

`loadSessionRestoreFromMenu(ctx)` — calls `resurrect.ResolveDir(ctx.SocketPath)`, then `resurrect.ListSaves(dir)`. Converts `SaveEntry` list to `[]Item` (ID = path, Label = formatted timestamp + name + session count).

- [ ] **Step 4: Register in menu.go**

Add entries to `ActionHandlers()`:
- `"session:save"` → `SessionSaveAction`
- `"session:save-as"` → `SessionSaveAsAction`
- `"session:restore"` → `SessionRestoreAction`
- `"session:restore-from"` → `SessionRestoreFromAction`

Add entry to `ActionLoaders()`:
- `"session:restore-from"` → `loadSessionRestoreFromMenu`

- [ ] **Step 5: Verify it compiles**

Run: `make build`
Expected: success

- [ ] **Step 6: Run full test suite**

Run: `make test`
Expected: PASS (or pre-existing failures only)

- [ ] **Step 7: Commit**

```bash
git add internal/menu/menu.go internal/menu/session.go
git commit -m "feat(menu): register save/restore/save-as/restore-from menu items"
```

---

## Task 13: CLI subcommands — save-sessions and restore-sessions

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Read main.go**

Read current subcommand dispatch pattern.

- [ ] **Step 2: Add save-sessions subcommand**

```go
if len(os.Args) > 1 && os.Args[1] == "save-sessions" {
	if err := runSaveSessions(runtimeCfg); err != nil {
		fmt.Fprintf(os.Stderr, "save-sessions: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
```

`runSaveSessions(cfg)`:
- Parse `--name` flag from `os.Args[2:]`.
- Check for `--resurrect-popup` flag. If present, this is the popup instance — enter the progress UI directly by setting `RootMenu` to a special value or using env vars to signal the operation.
- If not popup: resolve socket, find terminal client, launch `tmux display-popup` with `binary save-sessions --resurrect-popup` plus env vars for name.

- [ ] **Step 3: Add restore-sessions subcommand**

Same pattern. Parse `--from` flag. With `--resurrect-popup`, enter progress UI directly. Without, launch popup.

- [ ] **Step 4: Wire popup mode into app.Run**

The popup instance needs to start the tea.Program in resurrect mode. Add support in `app.Run` (or `ui.NewModel`) for an env var or config field that signals "enter ModeResurrect immediately on init". The model's `Init()` should check this and return a `ResurrectStart` command.

- [ ] **Step 5: Verify it compiles**

Run: `make build`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add main.go internal/app/app.go internal/ui/model.go
git commit -m "feat(cli): add save-sessions and restore-sessions subcommands"
```

---

## Task 14: Wire tmux option query into storage

**Files:**
- Modify: `internal/resurrect/storage.go`

- [ ] **Step 1: Implement defaultTmuxOptionFn**

Replace the placeholder `defaultTmuxOptionFn` with a real implementation that calls `tmux.ShowOption(socketPath, option)` or equivalent. This may need a new `ShowOption` helper in `internal/tmux/` that wraps `client.Command("show-option", "-gqv", option)`.

- [ ] **Step 2: Wire pane contents config lookup**

Add a similar lookup for `@tmux-popup-control-restore-pane-contents` — env var takes precedence over tmux option. This should be resolved in `internal/config/` or at the call site in the menu handlers.

- [ ] **Step 3: Test the tmux option path**

Add a test with `withTmuxOptionFn` returning a valid directory.

- [ ] **Step 4: Run tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/resurrect/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/resurrect/storage.go internal/tmux/restore.go
git commit -m "feat(resurrect): wire tmux option query into directory resolution"
```

---

## Task 15: Full test suite and cleanup

**Files:**
- All modified files

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: PASS (or only pre-existing `TestRootMenuRendering` failure)

- [ ] **Step 2: Run fmt**

Run: `make fmt`

- [ ] **Step 3: Fix any test failures**

Address any failures introduced by the new code.

- [ ] **Step 4: Commit any fixes**

Stage only the specific files that were fixed (use explicit paths, never `git add -u` or `git add .`), then commit:

```bash
git commit -m "fix: address test failures from save/restore integration"
```

---

## Task 16: Update context docs

**Files:**
- Modify: `context/todo.md`
- Modify: `context/done.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update done.md**

Add entry for save/restore feature completion.

- [ ] **Step 2: Update todo.md**

Remove any items completed; add any new follow-up items discovered during implementation (e.g. integration tests, auto-pruning, process resume).

- [ ] **Step 3: Update CLAUDE.md**

Add `internal/resurrect/` to the architecture layer overview. Add `session:save`, `session:save-as`, `session:restore`, `session:restore-from` to the menu system docs. Add `save-sessions` and `restore-sessions` to the CLI subcommands section. Add new env vars to the configuration table.

- [ ] **Step 4: Commit**

```bash
git add context/todo.md context/done.md CLAUDE.md
git commit -m "docs: update context and CLAUDE.md for save/restore feature"
```

---

## Task 17: Final verification

- [ ] **Step 1: Run full test suite one more time**

Run: `make test`
Expected: PASS

- [ ] **Step 2: Build and verify binary**

Run: `make build`
Expected: `./tmux-popup-control` binary produced

- [ ] **Step 3: Smoke test subcommands**

Run: `./tmux-popup-control save-sessions --help` (or similar) to verify the subcommand is recognized.
