# direct rename shortcut

## problem

tmux has built-in keybindings for quick rename:
- `$` — rename current session (`command-prompt -I "#S" { rename-session "%%" }`)
- `,` — rename current window (`command-prompt -I "#W" { rename-window "%%" }`)

tmux-popup-control's `--root-menu session:rename` and `--root-menu window:rename` open a picker list first, requiring the user to select which session/window to rename before seeing the form. for the common case of renaming the *current* session or window, this extra step is unnecessary.

## solution

use `--menu-args` to pass the target directly, skipping the picker and opening the rename form immediately once backend data is available.

### invocation

```
tmux-popup-control --root-menu session:rename --menu-args "mysession"
tmux-popup-control --root-menu window:rename --menu-args "mysession:1"
```

### changes

#### 1. `internal/ui/navigation.go` — `applyRootMenuOverride`

when the resolved node is `session:rename` or `window:rename` and `m.menuArgs` is non-empty, treat it as a **deferred rename form** instead of loading the picker list. this check must come **before** the existing `node.Loader != nil` branch, since both rename nodes have loaders that would otherwise trigger the standard picker path:

- set `m.loading = true`
- store the node in a new field `m.deferredRename` (reusing the same pattern as `m.deferredAction` but for form-opening actions)
- set `m.rootMenuID` and `m.rootTitle` as usual for root menu overrides
- do NOT call the node's loader

#### 2. `internal/ui/backend.go` — `applyBackendEvent`

extend the deferred handling to check `m.deferredRename` after backend data arrives:

**session:rename** (fires on `SessionsUpdated`):
- look up `m.menuArgs` as the target session name
- build a `SessionPrompt{Context: ctx, Action: "session:rename", Target: target, Initial: target}`
- call `m.startSessionForm(prompt)` to open the form directly

**window:rename** (fires on `WindowsUpdated`):
- look up `m.menuArgs` as the target window ID in `session:index` format (e.g. `mysession:1`), matching against `entry.ID`
- resolve the window name from the matched entry for the `Initial` field
- build a `WindowPrompt{Context: ctx, Target: target, Initial: windowName}`
- call `m.startWindowForm(prompt)` to open the form directly

both paths clear `m.deferredRename` and set `m.loading = false`. `m.loading` must be explicitly cleared when opening the form since `startSessionForm`/`startWindowForm` do not clear it themselves — without this the spinner would render over the form.

#### 3. `main.tmux` — keybindings

add two new configurable keybindings following the existing pattern:

```bash
# session rename — default '$'
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RENAME="$(opt key-session-rename)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RENAME='$'
tmux bind-key -T prefix -N "Renames session via $BINARY_NAME" \
  "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" \
  run-shell -b "$LAUNCH_SCRIPT --root-menu session:rename --menu-args #{session_name}"

# window rename — default ','
[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME="$(opt key-window-rename)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME=','
tmux bind-key -T prefix -N "Renames window via $BINARY_NAME" \
  "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" \
  run-shell -b "$LAUNCH_SCRIPT --root-menu window:rename --menu-args #{session_name}:#{window_index}"
```

#### 4. tests

unit tests for both paths:
- set `menuArgs` to a session/window name, call `applyRootMenuOverride`, verify `deferredRename` is set
- simulate backend event arrival, verify the correct form is opened with the right target and initial value

### edge cases

- **target doesn't exist**: the form opens normally; the rename command will fail with an `ActionResult{Err}` which is displayed and the popup stays open for the user to cancel.
- **empty `menuArgs`**: falls through to the existing picker behavior (no change).
- **`menuArgs` used without `session:rename`/`window:rename`**: no change to existing behavior; `menuArgs` is only checked for these specific node IDs.

### existing code reuse

the form creation (`NewSessionForm`, `NewWindowRenameForm`), form update loop (`handleSessionForm`, `handleWindowForm`), rename commands (`SessionRenameCommand`, `WindowRenameCommand`), and result handling (`handleActionResultMsg`) are all reused unchanged. the only new code is the deferred-form trigger in the root menu override and backend event paths.
