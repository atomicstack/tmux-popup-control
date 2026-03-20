# Save/Restore Sessions Design

## Overview

Add save and restore functionality to tmux-popup-control, inspired by
tmux-resurrect. Users can save their entire tmux state (sessions, windows,
panes, layouts, and optionally pane contents) to a JSON file and restore it
later. A progress popup with a scrolling log and gradient progress bar provides
visual feedback during both operations.

## Menu integration

Four new items under the existing `session` category:

| Menu ID              | Label            | Behaviour                                                        |
|----------------------|------------------|------------------------------------------------------------------|
| `session:save`       | save             | auto-timestamped save, enters progress UI immediately            |
| `session:save-as`    | save as...       | text form for a snapshot name, then progress UI                  |
| `session:restore`    | restore          | resolves `last` symlink, enters progress UI immediately          |
| `session:restore-from` | restore from... | lists all saves newest-first, user picks one, then progress UI |

### Action flow

- `session:save` handler runs discovery, sends
  `ResurrectStart{Operation: "save"}` → model enters `ModeResurrect`.
- `session:save-as` handler enters `ModeSessionSaveForm` (single text input
  for the name, reusing the existing form pattern) → on submit sends
  `ResurrectStart{Operation: "save", Name: "..."}` → `ModeResurrect`.
- `session:restore` handler resolves the `last` symlink via
  `resurrect.LatestSave()`. If no auto-save exists (symlink missing), the
  handler returns `menu.ActionResult{Err: ...}` with a message like "no saved
  session found" — the standard error display handles this.
- `session:restore-from` action loader calls
  `resurrect.ListSaves(resurrect.ResolveDir(ctx.SocketPath))` to scan the
  save directory and return save entries as menu items. The loader calls
  `ResolveDir` directly using the socket path from `menu.Context`. If the
  save directory does not exist or is empty, the loader returns no items and
  the menu shows the standard empty-list placeholder.

## CLI subcommands

```
tmux-popup-control save-sessions [--name my-snapshot]
tmux-popup-control restore-sessions [--from my-snapshot]
```

Each subcommand launches `tmux display-popup` with the binary itself as the
popup command. Extra parameters are passed via env vars:

- `save-sessions --name foo` → popup launched with
  `TMUX_POPUP_CONTROL_RESURRECT_NAME=foo`.
- `save-sessions` (no flag) → auto-timestamped, no extra env var.
- `restore-sessions --from foo` → popup launched with
  `TMUX_POPUP_CONTROL_RESURRECT_FROM=foo`.
- `restore-sessions` (no flag) → restores from `last` symlink, no extra env
  var.

**Popup detection:** The outer CLI invocation (run by the user or a tmux
keybinding) launches `tmux display-popup` with the binary as the popup
command, passing a `--resurrect-popup` flag to signal that the binary is
running inside the popup and should enter `ModeResurrect` directly. The
operation type is conveyed by `os.Args[1]` (`save-sessions` or
`restore-sessions`), and optional parameters via env vars. Without
`--resurrect-popup`, the subcommand launches the popup; with it, the
subcommand enters the progress UI. This prevents the popup from re-launching
another popup.

`--name` and `--from` accept either a bare name (resolved in the save
directory) or a full path.

## File format

### JSON save file

```json
{
  "version": 1,
  "timestamp": "2026-03-20T14:30:22Z",
  "name": "",
  "has_pane_contents": true,
  "client_session": "dev",
  "client_last_session": "shells",
  "sessions": [
    {
      "name": "dev",
      "created": 1710000000,
      "attached": true,
      "windows": [
        {
          "index": 0,
          "name": "editor",
          "layout": "bb02,185x62,0,0,5",
          "active": true,
          "alternate": false,
          "automatic_rename": false,
          "panes": [
            {
              "index": 0,
              "working_dir": "/Users/matt/git_tree/project",
              "title": "~",
              "command": "nvim",
              "width": 185,
              "height": 62,
              "active": true
            }
          ]
        }
      ]
    }
  ]
}
```

- `version` — format version for future evolution.
- `name` — empty for auto-timestamped saves, populated for named snapshots.
- `has_pane_contents` — whether a companion pane contents archive exists.

### Pane contents archive

When pane content capture is enabled, a companion file is written alongside the
JSON save file:

- Auto-save: `save_20260320T143022.panes.tar.gz`
- Named snapshot: `my-snapshot.panes.tar.gz`

The archive contains one plain text file per pane, named
`session:window_index.pane_index` (e.g. `dev:0.1`).

The restore logic derives the archive path from the JSON filename by replacing
`.json` with `.panes.tar.gz`.

### File naming and rotation

- Auto-saves: `save_20260320T143022.json`
- Named snapshots: `my-snapshot.json`
- `last` — symlink pointing to the most recent auto-save's JSON file. Named
  snapshots (`session:save-as`) do **not** update the `last` symlink — it
  always points to the latest auto-timestamped save only.
- No automatic pruning for now; users manage their own saves.

## Directory resolution

Lookup chain (first match wins):

1. `TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR` env var
2. `@tmux-popup-control-session-storage-dir` tmux option
3. `$XDG_DATA_HOME/tmux-popup-control-sessions/` (falls back to
   `$HOME/.local/share/tmux-popup-control-sessions/` if `XDG_DATA_HOME` is
   unset)
4. `$HOME/tmux-popup-control-sessions/`

The directory is created automatically if it does not exist. This chain
intentionally avoids any overlap with tmux-resurrect's `~/.tmux/resurrect/`
directory.

## Data captured

### Per session

- name, creation time, attached status

### Per window

- index, name, layout string, active flag, alternate flag, automatic-rename
  setting

### Per pane

- index, working directory, active flag, title
- current command (short form, e.g. "vim")
- dimensions (width × height)

### Client state

- currently attached session, alternate session

### Pane contents (opt-in)

- captured via `CapturePane` through the gotmuxcc control-mode connection
- stored in a separate `.panes.tar.gz` archive

### Not saved (for now)

- full command line / args (no process resume feature yet)
- process restoration strategies

## Save orchestration

### Phase 1: Discovery

Fetch all sessions, windows, and panes upfront using existing
`FetchSessions`, `FetchWindows`, `FetchPanes` snapshot functions in
`internal/tmux/`. Compute exact total work units before any save work begins.

### Phase 2: Save (fixed total, progress bar runs)

1. For each session: save metadata → step++
2. For each window: save layout → step++
3. (If pane contents enabled) For each pane: capture contents → step++
4. Write JSON file → step++
5. (If pane contents) Write tar.gz → step++
6. Update `last` symlink → step++

## Restore orchestration

### Phase 1: Discovery

- Read and parse the JSON save file.
- Count sessions, windows, panes to create.
- Check if companion tar.gz exists; if so, extract to a temp directory.
- Compute exact total work units.

### Phase 2: Restore (fixed total, progress bar runs)

1. For each session: create session → step++
2. For each window in session: create window → step++
3. For each pane in window: create pane (via `split-window`) → step++
4. For each window: apply layout (`select-layout`) → step++
5. (If pane contents) For each pane: send contents to pane → step++
6. For each window: set active pane → step++
7. For each session: set active/alternate window → step++
8. Restore client session attachment → step++
9. Clean up temp directory → step++

### Conflict handling

If a session with the same name already exists, skip it and all its windows
and panes, logging a warning. Skipped sessions still consume their budgeted
step count as no-ops so the progress bar total remains accurate. The log area
will show clearly what was skipped. This behaviour may change in the future.

## Progress UI

### Mode

A new `ModeResurrect` UI mode, reusable for both save and restore.

### State

```go
type resurrectState struct {
    operation string                       // "save" or "restore"
    progress  <-chan resurrect.ProgressEvent
    log       []logEntry                   // accumulated log lines
    step      int
    total     int
    done      bool
    err       error
}

type logEntry struct {
    message string  // raw text
    kind    string  // "session", "window", "pane", "info", "error"
    id      string  // entity name/ID for colouring
}
```

### Message types

- `ResurrectStart{Operation, Name, SaveFile}` — triggers mode transition,
  starts the goroutine, returns first read command.
- `resurrectProgressMsg{ProgressEvent}` — received for each channel event;
  updates state, returns next read command.
- `resurrectTickMsg{}` — fired 1s after done, triggers `tea.Quit`.

### Handler chain

1. `handleResurrectStartMsg` → initialises `resurrectState`, calls
   `resurrect.Save()` or `resurrect.Restore()` (which return the channel
   immediately with no blocking — all discovery and I/O runs in the spawned
   goroutine), returns `readResurrectProgress` command.
2. `readResurrectProgress` → reads one event from channel, wraps as
   `resurrectProgressMsg`.
3. `handleResurrectProgressMsg` → appends to log, updates step/total; if done,
   starts 1s tick; otherwise returns another `readResurrectProgress`.
4. On error: sets `err`, stops reading, waits for keypress.

### Layout

```
┌──────────────────────────────────────┐
│ saving session 'dev'...              │
│ saving window 'dev':0 'editor'...    │
│ saving layout for 'dev':0...         │
│ capturing pane contents for %5...    │
│ capturing pane contents for %6...    │
│                                      │
│                                      │
│                                      │
│                                      │
│ ████████████░░░░░░░░░░░░░  12/25     │
└──────────────────────────────────────┘
```

- **Log area**: fills all available height minus the progress bar row. New
  lines append at the bottom, older lines scroll up. Auto-scrolls to show most
  recent lines.
- **Progress bar**: always the last line. Fixed-total bar with step counter.

### Styling

**Progress bar gradient (24-bit colour):** Each character in the filled
portion is rendered with an interpolated RGB value.

- Save: white (`#ffffff`) → purple (`#7c4dff`) — accumulating state.
- Restore: purple (`#7c4dff`) → white (`#ffffff`) — pouring state back.

Empty portion uses a dim background (existing `styles.ProgressEmpty`).

**Progress counter:** Current step in `#7c4dff`, separator dim, total in
`#777777`.

**Log line ID colouring:** Hierarchical purple shading applied to entity
identifiers:

- Session names: `#b388ff`
- Window names/IDs: `#ce93d8`
- Pane IDs: `#e1bee7`

Error lines use the existing `styles.Error` (red).

### Completion behaviour

- **Success:** log shows a summary line (e.g. "saved 3 sessions, 7 windows,
  12 panes"), progress bar shows full, auto-dismiss after ~1 second via
  `tea.Tick`.
- **Error:** log shows error in red, progress stops, any keypress dismisses.

### Key handling in ModeResurrect

`ModeResurrect` is added as a case in `handleActiveForm` to consume all key
events (mirroring `ModePluginInstall`). This prevents key presses from falling
through to `handleKeyMsg` which would otherwise navigate the menu stack.

- While running: all keys consumed and ignored.
- On error: any key dismisses.
- On done: auto-dismiss via tick; keys also dismiss early.

### Save-as form

`ModeSessionSaveForm` is added to the `Mode` iota in `model.go` (alongside
`ModePaneForm`, `ModeWindowForm`, `ModeSessionForm`, etc.) and wired into
`handleActiveForm` for key routing. It reuses the existing form rendering
pattern from `forms.go` — single text input for the snapshot name, enter to
submit, escape to cancel. Validation checks that the name does not collide
with an existing save file in the save directory (distinct from the existing
`ModeSessionForm` which validates against tmux session names).

**Named snapshot collision:** The collision check runs only on form submission
(not per-keystroke) to avoid filesystem I/O on every character. If a save file
with the chosen name already exists, the form shows an inline info notice
(e.g. "snapshot 'foo' already exists — enter to overwrite") and stays open,
requiring a second enter to confirm. This allows intentional updates to named
snapshots while making accidental overwrites unlikely.

## Configuration

### Env vars

| Env var                                     | Purpose                     | Default         |
|---------------------------------------------|-----------------------------|-----------------|
| `TMUX_POPUP_CONTROL_SESSION_STORAGE_DIR`    | override save directory     | (lookup chain)  |
| `TMUX_POPUP_CONTROL_RESTORE_PANE_CONTENTS`  | enable pane content capture | `off`           |

### Tmux options

| Option                                        | Purpose                     | Default         |
|-----------------------------------------------|-----------------------------|-----------------|
| `@tmux-popup-control-session-storage-dir`     | override save directory     | (lookup chain)  |
| `@tmux-popup-control-restore-pane-contents`   | enable pane content capture | `off`           |

Env vars take precedence over tmux options. These are folded into the existing
`Config` struct in `internal/config/`.

## Package structure

```
internal/resurrect/
  resurrect.go      — public API: Save(), Restore(), ListSaves(), LatestSave()
  save.go           — save orchestration (discovery, iteration, progress events)
  restore.go        — restore orchestration (parse, create hierarchy, progress)
  storage.go        — directory resolution, file naming, JSON I/O, symlink mgmt
  pane_contents.go  — tar.gz archive creation and extraction
  types.go          — SaveFile struct, ProgressEvent, Config, session/window/pane types
```

### Public API

```go
type Config struct {
    SocketPath          string
    SaveDir             string // resolved by caller via ResolveDir()
    CapturePaneContents bool
    Name                string // empty for auto-timestamped
}

type ProgressEvent struct {
    Step    int
    Total   int
    Message string
    Done    bool
    Err     error
}

func ResolveDir(socketPath string) (string, error)
func Save(cfg Config) <-chan ProgressEvent
func Restore(cfg Config, file string) <-chan ProgressEvent
func ListSaves(dir string) ([]SaveEntry, error)
func LatestSave(dir string) (string, error)
```

`Save()` and `Restore()` return a read-only channel **immediately** with no
blocking work on the call path. All discovery (fetching sessions/windows/panes)
and I/O runs inside the spawned goroutine. The first `ProgressEvent` sent on the channel is a discovery event
(`Step: 0, Total: N, Message: "discovering sessions..."`) that communicates
the computed total. Until this first event arrives, the UI renders the log
area with a "discovering..." message and a dimmed empty progress bar (no
fraction displayed). The UI consumes events via Bubble Tea commands — one
command per event, each returning a `tea.Cmd` that reads the next event.

**Backend poller interaction:** The `backend.Watcher` continues polling during
save/restore. The gotmuxcc router serialises concurrent commands on the shared
control-mode connection, so there is no contention. Save/restore operations
are sequential within their goroutine and do not interfere with background
polling.

### Tmux interaction

The package imports `internal/tmux` and uses existing client functions:
`FetchSessions`, `FetchWindows`, `FetchPanes` for discovery;
`ListPanesFormat`/`DisplayMessage` for layout data; `CapturePane` for pane
contents.

**New tmux helpers required for restore:**

The restore path needs operations that do not yet exist on the `tmuxClient`
interface or as `internal/tmux` public functions:

- `CreateSession(socketPath, name, dir string) error` — wraps
  `client.NewSession()` or `client.Command("new-session", ...)`.
- `CreateWindow(socketPath, session string, index int, name, dir string) error`
  — wraps `client.Command("new-window", "-t", target, "-n", name, "-c", dir, "-d")`.
  The `-d` flag prevents auto-selection of the new window.
- `SplitPane(socketPath, target, dir string) error` — wraps
  `client.Command("split-window", "-t", target, "-c", dir, "-d")`.
- `SelectLayoutTarget(socketPath, target, layout string) error` — wraps
  `client.Command("select-layout", "-t", target, layout)`. The existing
  `SelectLayout` function has no target parameter and applies to the
  control-mode session's active window, which is unsuitable for batch restore.
- `CapturePaneContents(socketPath, target string) (string, error)` — wraps
  `client.CapturePane(target, opts)` for bulk capture with an explicit target.
  Uses `CaptureOptions{PreserveTrailingSpace: true}` with no escape sequences
  and no start-line limit — captures the full scrollback as plain text. The
  existing preview-oriented capture functions use styled output with
  `EscTxtNBgAttr: true` and limited history, which is unsuitable for
  save/restore.

All of these use `client.Command()` (the generic gotmuxcc command interface)
rather than introducing new gotmuxcc API surface. They follow the existing
pattern of thin wrappers in `internal/tmux/` that acquire the cached client
via `newTmux(socketPath)`.

Handle abstractions are not needed for these operations — they are
fire-and-forget commands that do not return gotmuxcc objects.

### Testing

- Unit tests: `withStubTmux` to inject fakes for tmux operations.
- UI tests: `Harness` to drive model updates through the progress event
  sequence.
- Integration tests: `StartTmuxServer` + `launchBinary` for live save/restore
  round-trips.
- Storage tests: temp directories for file I/O, symlink, and tar.gz
  operations.
