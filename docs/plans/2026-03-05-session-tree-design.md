# Session Tree Feature Design

## Overview

Add a `session:tree` menu item to the session submenu that displays an interactive, navigable tree of sessions, windows, and panes. The tree uses Unicode characters (▶/▼ collapse indicators, ├──/└── connectors) and supports fuzzy filtering, keyboard navigation, and pane capture previews.

## Approach

Use lipgloss's `tree` sub-package (available in v0.13.1) for rendering, while keeping the existing `Level` infrastructure for interactive state (cursor, viewport, filter, preview). The tree is represented as a flat `[]menu.Item` list with expand/collapse state tracked separately. Lipgloss builds the visual tree string from this flat list at render time.

## Menu Registration & Data Model

- New item `session:tree` in the session category loader (`internal/menu/session.go`), labeled "tree".
- Registered in `ActionLoaders()` with loader `loadSessionTreeMenu`.
- Item ID encoding:
  - Sessions: `tree:s:session-name`
  - Windows: `tree:w:session-name:window-id`
  - Panes: `tree:p:session-name:window-id:pane-id`
- Expand/collapse state: `map[string]bool` keyed by item ID, stored on the level's `Data` field.

## CLI Parameters

- New `--menu-args` flag (env var `TMUX_POPUP_CONTROL_MENU_ARGS`), added to `Config` and `app.Config` as string field `MenuArgs`.
- `--root-menu session:tree` opens the tree directly.
- `--root-menu session:tree --menu-args expanded` opens with all nodes expanded.
- Other menus ignore `--menu-args`. The tree loader parses it; the only recognized value for now is `expanded`.

## Tree Rendering

- When the active level is `session:tree`, `view.go` uses `renderTreeView` instead of per-item `buildItemLine`.
- `renderTreeView` builds a `tree.Tree` per session from the flat item list:
  - Session nodes: `▶ name` (collapsed) or `▼ name` (expanded).
  - Window nodes: children of their session, with ▶/▼ if they have panes.
  - Pane nodes: leaf children of their window.
- Cursor highlight via `ItemStyleFunc`/`EnumeratorStyleFunc`/`IndenterStyleFunc` — conditionally applies selected vs normal style based on cursor position.
- Enumerator: `tree.DefaultEnumerator` (├──/└──) for children.
- `tree.Width(availableWidth)` for full-row cursor highlight background.
- Integrates with existing `viewSideBySide`/`viewVertical` layout; tree string replaces item list portion.

## Navigation

- **Up/Down:** Standard flat-list cursor movement (wrapping, viewport scroll, page up/down, Home/End).
- **Right on collapsed node:** Expand, stay on row.
- **Right on expanded node or leaf:** Move cursor down one item.
- **Left on expanded node:** Collapse, stay on row.
- **Left on collapsed node or leaf:** Move cursor up one item.
- **Enter:** Parse item ID prefix:
  - `tree:s:*` → `SwitchClient` to session, quit.
  - `tree:w:*` → `SwitchClient` to session + `SelectWindow`, quit.
  - `tree:p:*` → `SwitchClient` to session + `SelectWindow` + select pane, quit.
- **Escape:** Pop level (return to session submenu or exit if `--root-menu session:tree`).

## Filtering

- Standard fuzzy text input box, using existing `fuzzysearch`.
- Matches against display labels at all depths (session names, window names, pane titles).
- Ancestor preservation: matched children keep their parent chain visible.
- Auto-expand: collapsed nodes with matching children are expanded while filter is active.
- On filter clear: expand/collapse state reverts to pre-filter state (saved copy).

## Preview

- Preview kind: `previewKindPane` — always shows `capture-pane` output.
- Resolution:
  - Pane selected → capture that pane.
  - Window selected → capture window's active pane.
  - Session selected → capture session's active window's active pane.
- Reuses `activePaneIDForSession`, `activePaneIDForWindow` helpers.
- Side-by-side when terminal is wide enough, otherwise inline.
- Async with sequence numbers to discard stale responses.

## Dependencies

- Bump lipgloss from v0.10.0 → v0.13.1 (same v0.x line; compatible with bubbles v0.16.0, bubbletea v0.25.0).
- No new external dependencies beyond the lipgloss upgrade.
