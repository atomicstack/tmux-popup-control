# Window Layout Live Preview

## Summary

When browsing the `window:layout` submenu, moving the cursor applies the
highlighted layout to the current tmux window in real-time. Pressing Enter
confirms (layout is already applied). Pressing Escape reverts to the original
layout.

A "current-layout" item at the bottom of the list lets the user explicitly
return to the original layout while still inside the menu.

## Data flow

1. **Enter submenu** -- loader reads `Context.CurrentWindowLayout` (the raw
   layout string from `#{window_layout}`) and appends a "current-layout" item
   whose ID is that string. On the first cursor-triggered preview call, the
   original layout is saved into `level.Data`.

2. **Cursor movement** -- the preview system recognises `window:layout` as
   `previewKindLayout`. Instead of rendering preview panel content, it fires an
   async command that calls `selectLayoutFn(socket, itemID)`. The tmux window
   itself serves as the preview. The `previewData.target` field prevents
   re-applying when the cursor hasn't changed items.

3. **Enter** -- `WindowLayoutAction` returns a success `ActionResult`. The
   layout is already applied; no extra work needed.

4. **Escape** -- `handleEscapeKey()` checks `current.ID == "window:layout"` and
   `current.Data`. If an original layout was saved, it fires
   `selectLayoutFn(socket, originalLayout)` to revert before popping the level.

## Changes by layer

### tmux layer (`internal/tmux/`)

Add `WindowLayout(socketPath string) (string, error)` -- runs
`display-message -p '#{window_layout}'` via control-mode and returns the raw
layout string.

### Menu context (`internal/menu/`)

- Add `CurrentWindowLayout string` to `menu.Context` and `menu.WindowEntry`.
- `loadWindowLayoutMenu` reads `ctx.CurrentWindowLayout` and appends a
  "current-layout" item at the end with the raw layout string as its ID.
- Add injectable `windowLayoutFn` package-level var (for testing).

### Backend / dispatcher

Populate `CurrentWindowLayout` from the window data fetched by the backend.

### Preview system (`internal/ui/preview.go`)

- New `previewKindLayout` constant.
- `previewKindForLevel` maps `"window:layout"` to it.
- Layout branch in `ensurePreviewForLevel`: save original layout to
  `level.Data` on first call; fire async `selectLayoutFn`; no preview panel
  content rendered.

### Escape handling (`internal/ui/navigation.go`)

In `handleEscapeKey()`, before popping the level: if leaving `window:layout`
and `level.Data` holds a string, fire `selectLayoutFn` with that string.

### Testing

- Unit tests with stub commander: layout applied on cursor change, reverted on
  escape, "current-layout" item present with correct ID, no re-apply on same
  item.
