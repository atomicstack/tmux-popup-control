# pane capture-to-file

## summary

Add a `pane:capture` menu item that captures the full scrollback of the current
pane and writes it to a file. The user is presented with a form containing a
text input (prefilled with a tmux-format template), a live preview of the
expanded filename, and a checkbox to optionally include ANSI escape sequences.

This replaces the existing tmux keybinding:

```
bind-key -T prefix H command-prompt -I "/Users/matt/tmux-#D.%F-%H-%M-%S.log" \
  -p capture-pane-to-file: "capture-pane -S - ; save-buffer %1 ; delete-buffer"
```

## menu registration

- ID: `pane:capture`
- Added to `loadPaneMenu()` item list (after `break`, before `switch`)
- Registered in `ActionHandlers()` — no entry in `ActionLoaders()` (no submenu;
  always targets the current pane)
- Direct invocation via `TMUX_POPUP_CONTROL_ROOT_MENU=pane:capture` works
  because `applyRootMenuOverride` already handles action-only nodes

## action flow

1. `PaneCaptureAction` returns a `PaneCapturePrompt` message containing the
   menu `Context` and the default template `~/tmux-#{pane_id}.%F-%H-%M-%S.log`
2. The UI handler receives `PaneCapturePrompt`, creates a `PaneCaptureForm`,
   and switches to `ModePaneCaptureForm`
3. The form immediately fires a command to expand the template via
   `ExpandFormat` (which wraps `DisplayMessage`) and returns a
   `PaneCapturePreviewMsg` with the resolved path
4. On each input change, a new expand command is dispatched (debounced ~200ms)
5. On enter, the form submits; `PaneCaptureCommand` expands `~` to `$HOME`,
   resolves the template, captures the pane, and writes the file

## form type: `PaneCaptureForm`

```go
type PaneCapturePrompt struct {
    Context  Context
    Template string // default: ~/tmux-#{pane_id}.%F-%H-%M-%S.log
}

type PaneCaptureForm struct {
    input      textinput.Model
    ctx        Context
    escSeqs    bool   // checkbox: include escape sequences (default false)
    preview    string // expanded template from DisplayMessage
    previewErr string // error from expansion, if any
}
```

### keybindings

| key | action |
|---|---|
| tab | toggle escape sequences checkbox |
| enter | submit (capture pane and write to file) |
| esc | cancel, return to menu |
| ctrl+u | clear input |
| backspace | delete character |

### rendering

```
pane→capture

~/tmux-#{pane_id}.%F-%H-%M-%S.log|     ← text input with cursor

□ capture escape sequences               ← checkbox, tab to toggle

~/tmux-%3.2026-03-22-14-30-00.log        ← faint preview of expanded path

tab: toggle escape sequences · enter: save · esc: cancel
```

The checkbox uses `■` (checked) / `□` (unchecked), styled with
`styles.CheckboxChecked` / `styles.Checkbox` for consistency with the existing
multi-select UI.

The preview line is rendered in faint style. If expansion fails, the error is
shown in `styles.Error` instead.

## tmux layer

New file `internal/tmux/capture.go`:

```go
// CapturePaneToFile captures the full scrollback of a pane and writes it to a
// file. escSeqs controls whether ANSI escape sequences are included.
var capturePaneToFileFn = CapturePaneToFile

func CapturePaneToFile(socketPath, paneTarget, filePath string, escSeqs bool) error

// ExpandFormat resolves a tmux format string against a target via
// DisplayMessage. Used for the live preview.
var expandFormatFn = ExpandFormat

func ExpandFormat(socketPath, target, format string) (string, error)
```

`CapturePaneToFile` uses gotmuxcc `CapturePane` with:
- `StartLine: "-"` (full scrollback)
- `EscTxtNBgAttr: escSeqs`

The captured string is written to `filePath` via `os.WriteFile` with mode
`0644`. The `~` → `$HOME` expansion and template resolution happen in
`PaneCaptureCommand` before calling this function.

`ExpandFormat` is a thin wrapper around `DisplayMessage(target, format)`.

Both are package-level function vars for test injection.

## UI wiring

### model.go

- New constant `ModePaneCaptureForm` added to the `Mode` enum
- New field `paneCaptureForm *menu.PaneCaptureForm` on `Model`

### forms.go

- `handlePaneCaptureForm(msg) (bool, tea.Cmd)` — delegates to form's `Update`;
  on done, calls `PaneCaptureCommand`
- `startPaneCaptureForm(prompt)` — creates form, sets mode
- `viewPaneCaptureForm(header) string` — renders header, input, checkbox,
  preview, help

### prompt.go

- `handlePaneCapturePromptMsg(msg) tea.Cmd` — receives `PaneCapturePrompt`,
  calls `startPaneCaptureForm`

### view.go

- `ModePaneCaptureForm` case in `View()` switch — renders via
  `viewPaneCaptureForm`

### model.go (handleActiveForm)

- `ModePaneCaptureForm` case delegates to `handlePaneCaptureForm`

### registerHandlers

- `PaneCapturePrompt` → `handlePaneCapturePromptMsg`
- `PaneCapturePreviewMsg` → handler that updates `paneCaptureForm.preview`

## file changes

| file | change |
|---|---|
| `internal/menu/pane.go` | `PaneCapturePrompt`, `PaneCaptureForm`, `PaneCaptureAction`, `PaneCaptureCommand`; add `"capture"` to `loadPaneMenu` |
| `internal/menu/menu.go` | register `"pane:capture": PaneCaptureAction` in `ActionHandlers()` |
| `internal/tmux/capture.go` | new file: `CapturePaneToFile`, `ExpandFormat` (+ injectable vars) |
| `internal/ui/model.go` | `ModePaneCaptureForm` constant, `paneCaptureForm` field |
| `internal/ui/forms.go` | `handlePaneCaptureForm`, `startPaneCaptureForm`, `viewPaneCaptureForm` |
| `internal/ui/prompt.go` | `handlePaneCapturePromptMsg` |
| `internal/ui/view.go` | `ModePaneCaptureForm` case in `View()` switch |
| `internal/logging/events/` | trace events for capture |
| tests | unit tests for form, tmux functions, UI handler |

## tilde expansion

The form accepts `~` as a substitute for `$HOME` in the path template. On
submit, `PaneCaptureCommand` replaces a leading `~/` with `os.UserHomeDir() +
"/"`. The preview also applies this expansion so the user sees the resolved
absolute path.

## testing

- **Form unit tests:** verify key handling (tab toggle, enter submit, esc
  cancel, ctrl+u clear), initial template prefill, checkbox state
- **tmux unit tests:** `CapturePaneToFile` with `withStubTmux` verifying
  `CaptureOptions` flags and file write; `ExpandFormat` with stub
  `DisplayMessage`
- **UI handler tests:** `PaneCapturePrompt` triggers form mode;
  `PaneCapturePreviewMsg` updates preview; form submit dispatches command
