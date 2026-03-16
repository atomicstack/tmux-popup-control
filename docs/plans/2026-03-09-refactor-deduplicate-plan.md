# Refactor: Deduplicate Common Code & Leverage Upstream APIs

**Date:** 2026-03-09
**Goal:** Reduce code duplication across the codebase and replace hand-rolled logic with upstream library APIs where possible.

---

## Summary of Findings

### Code Duplication (internal patterns)
1. **Rename forms** — `WindowRenameForm`, `PaneRenameForm`, and `SessionForm` share identical struct fields, accessor methods, and Update() logic
2. **Menu loaders with current-item prepend** — 6 loader functions (`loadPaneBreakMenu`, `loadPaneSwapMenu`, `loadPaneKillMenu`, `loadPaneRenameMenu`, `loadWindowSwapMenu`, `loadWindowKillMenu`) are identical: `EntriesToItems()` + `currentItem()` prepend
3. **`currentWindowItem` / `currentPaneItem`** — identical functions in `window.go` and `pane.go`
4. **`windowItemFromEntry` / `paneItemFromEntry`** — identical one-liner converters
5. **`EntriesToItems` functions** — `SessionEntriesToItems`, `WindowEntriesToItems`, `PaneEntriesToItems` all do `Item{ID: entry.ID, Label: entry.Label}`
6. **`splitWindowIDs` / `splitPaneIDs`** — near-identical ID splitting with dedup
7. **`WindowSwitchItems` / `windowRenameItems`** — share ~70% of their row-building logic (entry segregation, sort, table formatting)
8. **State stores** — `SessionStore`, `WindowStore`, `PaneStore` all follow the same `clone + get/set` template
9. **UI form handlers** — `handlePaneForm`, `handleWindowForm`, `handleSessionForm` in `forms.go` follow the same done/cancel/cmd pattern
10. **`viewVertical` / `viewSideBySide`** — share the item-rendering loop (header, viewport, display items, empty-state, tree-level, footer)
11. **Event tracer reason types** — `sessionReason`, `windowReason`, `paneReason` are identical string types with identical constants

### Upstream API Opportunities
1. **lipgloss `Border()`** — `renderPreviewPanel` hand-draws rounded borders character-by-character; lipgloss has `RoundedBorder()` + `Style.Border()` built-in
2. **lipgloss `Style.Width()` / `Style.Padding()`** — manual `strings.Repeat(" ", pad)` padding throughout `buildItemLine`, `viewSideBySide`, `renderPreviewPanel` could use lipgloss width/padding
3. **bubbles `textinput.Validate`** — forms manually call `validate()` after every Update; the textinput model has a built-in `Validate` field

---

## Implementation Plan

### Step 1: Extract `splitIDs` helper
**Files:** `internal/menu/pane.go`, `internal/menu/window.go`
**Change:** Replace `splitPaneIDs` and `splitWindowIDs` with a single `splitIDs(raw string) []string` in `internal/menu/menu.go`. The only difference is that `splitPaneIDs` also splits on space while `splitWindowIDs` doesn't — the new function should accept a variadic separator parameter or use the superset (split on space, comma, newline).
**Tests:** Update `TestSplitPaneIDs` and `TestSplitWindowIDs` (if they exist) to call the unified function. Add a table test covering all edge cases.

### Step 2: Unify `currentItem` and `itemFromEntry` helpers
**Files:** `internal/menu/pane.go`, `internal/menu/window.go`
**Change:**
- Replace `paneItemFromEntry` / `windowItemFromEntry` with a single unexported function `itemFromIDLabel(id, label string) Item` in `menu.go` (or inline the one-liner at call sites).
- Replace `currentPaneItem` / `currentWindowItem` with a single `currentItem(id, label string) (Item, bool)` in `menu.go`.
**Tests:** Update callers. The existing loader tests cover these indirectly.

### Step 3: Extract `loadMenuWithCurrent` helper
**Files:** `internal/menu/pane.go`, `internal/menu/window.go`
**Change:** The 6 identical loaders (`loadPaneBreakMenu`, `loadPaneSwapMenu`, `loadPaneKillMenu`, `loadPaneRenameMenu`, `loadWindowSwapMenu`, `loadWindowKillMenu`) all do:
```go
items := <Type>EntriesToItems(ctx.<Type>s)
if current, ok := current<Type>Item(ctx); ok {
    items = append([]Item{current}, items...)
}
return items, nil
```
Replace with a generic helper:
```go
func loadWithCurrent(items []Item, currentID, currentLabel string) []Item
```
Each loader becomes a one-liner calling `loadWithCurrent`.
**Tests:** Existing loader tests (`TestLoadPaneBreakMenu`, etc.) should continue to pass.

### Step 4: Unify `EntriesToItems` with a generic function
**Files:** `internal/menu/session.go`, `internal/menu/menu.go`, `internal/menu/pane.go`
**Change:** Go 1.24 supports generics. Define an interface:
```go
type itemizer interface {
    ItemID() string
    ItemLabel() string
}
```
Add `ItemID()`/`ItemLabel()` methods to `SessionEntry`, `WindowEntry`, `PaneEntry`. Then write one function:
```go
func entriesToItems[T itemizer](entries []T) []Item
```
Remove `SessionEntriesToItems`, `WindowEntriesToItems`, `PaneEntriesToItems`.
**Tests:** Search and replace all call sites. Existing tests cover behavior.

### Step 5: Unify event tracer reason types
**Files:** `internal/logging/events/session.go`, `window.go`, `pane.go`
**Change:** The three reason types (`sessionReason`, `windowReason`, `paneReason`) and their constants (`*ReasonEscape`, `*ReasonEmpty`) are identical. Extract a single exported `Reason` type with `ReasonEscape` and `ReasonEmpty` constants. Update all three tracers to use it.
**Tests:** Compile-time check only — no runtime behavior change. Grep for all references to the old constants and update.

### Step 6: Extract base rename form
**Files:** `internal/menu/pane.go`, `internal/menu/window.go`
**Change:** `WindowRenameForm` and `PaneRenameForm` are structurally identical:
- Same fields: `input textinput.Model`, `ctx Context`, `target string`, `help string`, `title string`
- Same accessors: `Context()`, `Target()`, `Title()`, `Help()`, `Value()`, `InputView()`
- Same `PendingLabel()` logic
- Same `Update()` key handling (ctrl+u, esc, enter) — only difference is the event tracer calls and the submit action

Extract a `baseRenameForm` struct with shared fields and methods. `WindowRenameForm` and `PaneRenameForm` embed it, overriding only `ActionID()`, `Update()` (for event-specific tracing), and `SyncContext()` (pane only).

**Alternatively**, define a `RenameForm` with callbacks:
```go
type RenameForm struct {
    input    textinput.Model
    ctx      Context
    target   string
    help     string
    title    string
    actionID string
    onEsc    func(target string)
    onEmpty  func(target string)
    onSubmit func(target, value string)
}
```
This eliminates the two separate types entirely. `NewWindowRenameForm` and `NewPaneRenameForm` become factory functions that set different callbacks.

**Tests:** Port existing `TestWindowRenameFormUpdate` and `TestPaneRenameFormUpdate` tests. Ensure ctrl+u, esc, enter-empty, enter-with-value all work as before.

### Step 7: Unify UI form handlers
**Files:** `internal/ui/forms.go`
**Change:** `handlePaneForm`, `handleWindowForm`, `handleSessionForm` follow the same pattern:
```go
cmd, done, cancel := form.Update(msg)
if cancel → clear form, mode = ModeMenu
if done → extract ctx/value/target/actionID/pendingLabel, clear form, set loading, create cmd
```
If Step 6 introduces a common form interface, define:
```go
type formModel interface {
    Update(tea.Msg) (tea.Cmd, bool, bool)
    Context() Context
    Value() string
    Target() string
    ActionID() string
    PendingLabel() string
}
```
Then write one `handleForm(form formModel, fallbackCmd func(Context, string, string) tea.Cmd)` method. The three handle* methods become thin wrappers.

**Note:** `SessionForm` is more complex (it has `Error()`, `SetSessions()`, `IsRename()`, create vs rename modes). It may not fit this interface cleanly. If the unification feels forced, keep `handleSessionForm` separate and only merge pane+window handlers.

**Tests:** UI harness tests for form behavior should be re-run.

### Step 8: Extract shared item-rendering in `viewVertical`/`viewSideBySide`
**Files:** `internal/ui/view.go`
**Change:** Both `viewVertical` and `viewSideBySide` contain a ~30-line block that:
1. Adds header
2. Syncs viewport
3. Computes displayItems from viewport window
4. Renders empty state, tree view, or item lines
5. Adds info and footer

Extract this into a `buildMenuLines(header string, width int) []styledLine` method on Model. Both View functions call it, then diverge for their specific layout logic.

**Tests:** Existing `TestView*` tests and integration tests exercise both paths.

### Step 9: Replace manual preview border with lipgloss
**Files:** `internal/ui/view.go`
**Change:** `renderPreviewPanel` (lines 348-476) manually constructs border characters `╭╮╰╯─│`. Replace the border construction with `lipgloss.NewStyle().Border(lipgloss.RoundedBorder())`. The tricky parts:
- Title text needs to be inset into the top border — lipgloss doesn't natively support this, so we may need to keep the manual top line but use lipgloss for the remaining 3 sides, OR keep the manual approach but simplify inner content padding with `Style.Width()`.
- Scroll indicator in the top-right corner.

**Recommendation:** Because the top border has embedded title + scroll info, keep the manual top-border line but use `lipgloss.NewStyle().Width(innerW)` for inner content padding to eliminate the manual `strings.Repeat(" ", innerW-w)` padding loops. This still saves ~15 lines.

**Tests:** Visual verification via `UPDATE_GOLDEN=1 make test` if golden files cover previews. Otherwise manual verification.

### Step 10: Use lipgloss `Style.Width()` for padding in `buildItemLine`
**Files:** `internal/ui/view.go`
**Change:** In `buildItemLine` (line 334):
```go
if pad := width - len([]rune(fullText)); pad > 0 {
    fullText += strings.Repeat(" ", pad)
}
```
Replace with storing the width on the styledLine and letting the rendering step handle it, or use lipgloss:
```go
fullText = lipgloss.NewStyle().Width(width).Render(fullText)
```

**Caveat:** This interacts with the `styledLine` rendering pipeline — `renderLines` applies styles separately from width. Evaluate whether this change fits cleanly or would require reworking the pipeline. If it doesn't fit cleanly, skip this step.

**Tests:** Existing view tests. Manual verification of alignment in side-by-side mode.

### Step 11: Deduplicate `WindowSwitchItems` / `windowRenameItems`
**Files:** `internal/menu/window.go`
**Change:** These two functions share ~80% of their code (segregate current, sort, build table rows). Extract a shared helper:
```go
func buildWindowTable(entries []WindowEntry, includeCurrent bool, currentFirst bool) []Item
```
- `WindowSwitchItems` calls it with `includeCurrent=ctx.WindowIncludeCurrent, currentFirst=true`
- `windowRenameItems` calls it with `includeCurrent=true, currentFirst=true`

**Tests:** Existing `TestWindowSwitchItems` and window rename tests.

### Step 12: State store generics (evaluate only)
**Files:** `internal/state/session.go`, `window.go`, `pane.go`
**Change:** The three stores follow an identical pattern but have different `SetCurrent` signatures (session: single string; window: three strings; pane: two strings). This asymmetry means a generic base type would need a flexible "current" representation. This may not be worth the abstraction cost.

**Recommendation:** Mark as **evaluated and deferred**. The stores are small (55-70 lines each), the duplication is mechanical, and a generic version would sacrifice readability for negligible savings. The `clone*Entries` functions could be replaced with a single `cloneSlice[T any](s []T) []T` generic — do that much and leave the rest.

**Tests:** Existing store tests.

---

## Execution Order & Dependencies

```
Step 1  (splitIDs)             — independent
Step 2  (currentItem helpers)  — independent
Step 3  (loadMenuWithCurrent)  — depends on Step 2 and Step 4
Step 4  (generic EntriesToItems) — independent
Step 5  (event reason types)   — independent
Step 6  (base rename form)     — independent
Step 7  (UI form handlers)     — depends on Step 6
Step 8  (view rendering)       — independent
Step 9  (preview borders)      — independent
Step 10 (buildItemLine lipgloss) — evaluate feasibility after Step 9
Step 11 (window table builder) — independent
Step 12 (state store generics) — independent, evaluate-only
```

**Parallelizable groups:**
- Group A: Steps 1, 2, 4, 5 (small, independent extractions)
- Group B: Steps 6, 7 (form unification chain)
- Group C: Steps 8, 9, 10 (view/rendering)
- Group D: Steps 3, 11 (menu helpers, depend on Group A)
- Group E: Step 12 (evaluation only)

## Testing Strategy

Each step must:
1. Run `make test` — all existing tests must pass
2. Run `make build` — compilation check
3. Verify no new test gaps:
   - Steps that introduce new helper functions need unit tests for those helpers
   - Steps that remove functions need call-site verification (grep for old function names)
4. After all steps: `make cover` to verify coverage hasn't dropped

## Estimated Impact

| Step | Lines Removed | Lines Added | Net |
|------|--------------|-------------|-----|
| 1 | ~25 | ~15 | -10 |
| 2 | ~20 | ~10 | -10 |
| 3 | ~40 | ~10 | -30 |
| 4 | ~20 | ~15 | -5 |
| 5 | ~10 | ~5 | -5 |
| 6 | ~80 | ~50 | -30 |
| 7 | ~30 | ~20 | -10 |
| 8 | ~30 | ~15 | -15 |
| 9 | ~15 | ~10 | -5 |
| 10 | ~5 | ~3 | -2 |
| 11 | ~40 | ~25 | -15 |
| 12 | ~10 | ~5 | -5 |
| **Total** | **~325** | **~183** | **~-142** |

## What's NOT included (and why)

- **Tmux client lifecycle (`withClient` helper):** The codebase uses a cached client pattern (`newTmux` caches). Adding a `withClient(socket, fn)` wrapper that calls `Close()` would conflict with the caching design. The cached client is closed once via `tmux.Shutdown()`. No leak exists with the current architecture.
- **Event tracer consolidation beyond reason types:** Each tracer has operation-specific methods with different signatures. Making them generic would lose type safety and readability for marginal savings.
- **`textinput.Validate` adoption:** Evaluated but deferred. The session form's validation is tightly coupled to live session list updates (`SetSessions()`). The textinput `Validate` field only receives the current string value, not the form's `existing` map. Shoehorning this into `Validate` would require closures over mutable state, which is fragile.
- **`lipgloss.RoundedBorder()` for full preview panel:** The custom title-in-border and scroll-indicator-in-border features aren't supported by lipgloss's border API. Keeping the manual top/bottom border lines.
