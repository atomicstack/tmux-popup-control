# Window Layout Live Preview — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Moving the cursor in the `window:layout` submenu applies the layout in real-time; Enter confirms, Escape reverts to the original layout.

**Architecture:** Piggyback on the existing preview system (`ensurePreviewForLevel`). A new `previewKindLayout` fires `selectLayoutFn` on cursor change instead of rendering preview content. The original layout string is captured from `gotmuxcc.Window.Layout` on the current window, piped through `WindowEntry` → `menu.Context` → the loader. `level.Data` stores the original layout for escape-revert.

**Tech Stack:** Go, Bubble Tea, gotmuxcc control-mode

---

### Task 1: Add layout field to Window/WindowEntry/WindowSnapshot

Thread the `Layout` field from `gotmuxcc.Window.Layout` through the data pipeline so the menu loader can access the current window's layout string.

**Files:**
- Modify: `internal/tmux/types.go:10-19` (Window struct)
- Modify: `internal/tmux/snapshots.go:109-118` (FetchWindows, populate Layout)
- Modify: `internal/menu/menu.go:43-51` (WindowEntry struct)
- Modify: `internal/menu/window.go:356-346` (WindowEntriesFromTmux)

**Step 1: Add Layout to tmux.Window**

In `internal/tmux/types.go`, add `Layout` field to the Window struct:

```go
type Window struct {
	ID         string
	Session    string
	Index      int
	Name       string
	Active     bool
	Label      string
	Current    bool
	InternalID string
	Layout     string
}
```

**Step 2: Populate Layout in FetchWindows**

In `internal/tmux/snapshots.go`, in the `FetchWindows` function, populate `Layout` from the gotmuxcc window object. There are two code paths — the fallback path (line ~88, where `w == nil`) and the main path (line ~109). Only populate in the main path:

```go
entry := Window{
	ID:         displayID,
	Session:    session,
	Index:      w.Index,
	Name:       w.Name,
	Active:     w.Active,
	Label:      line.label,
	Current:    session == currentSession && w.Active,
	InternalID: line.windowID,
	Layout:     w.Layout,
}
```

**Step 3: Add Layout to menu.WindowEntry**

In `internal/menu/menu.go`, add `Layout` field to `WindowEntry`:

```go
type WindowEntry struct {
	ID         string
	Label      string
	Name       string
	Session    string
	Index      int
	InternalID string
	Current    bool
	Layout     string
}
```

**Step 4: Pipe Layout in WindowEntriesFromTmux**

In `internal/menu/window.go`, in `WindowEntriesFromTmux`, add `Layout: w.Layout`:

```go
entries = append(entries, WindowEntry{
	ID:         id,
	Label:      label,
	Name:       w.Name,
	Session:    w.Session,
	Index:      w.Index,
	InternalID: w.InternalID,
	Current:    w.Current,
	Layout:     w.Layout,
})
```

**Step 5: Add CurrentWindowLayout to menu.Context**

In `internal/menu/menu.go`, add to the Context struct:

```go
type Context struct {
	// ... existing fields ...
	CurrentWindowLayout string
}
```

**Step 6: Populate CurrentWindowLayout in menuContext()**

In `internal/ui/commands.go`, in `menuContext()`, derive the layout from the current window entry:

```go
func (m *Model) menuContext() menu.Context {
	ctx := menu.Context{
		// ... existing fields ...
	}
	for _, w := range ctx.Windows {
		if w.Current {
			ctx.CurrentWindowLayout = w.Layout
			break
		}
	}
	return ctx
}
```

**Step 7: Run tests**

Run: `make test`
Expected: All tests pass (no tests reference the new field yet).

**Step 8: Commit**

```
feat: thread window layout string through data pipeline
```

---

### Task 2: Update loadWindowLayoutMenu to include current-layout item

The loader reads `ctx.CurrentWindowLayout` and appends a "current-layout" item at the bottom with the raw layout string as its ID.

**Files:**
- Modify: `internal/menu/window.go:259-270` (loadWindowLayoutMenu)

**Step 1: Write the failing test**

In `internal/menu/window_test.go`, add:

```go
func TestLoadWindowLayoutMenuIncludesCurrentLayout(t *testing.T) {
	ctx := Context{
		CurrentWindowLayout: "bb62,159x48,0,0{79x48,0,0,79x48,80,0}",
	}
	items, err := loadWindowLayoutMenu(ctx)
	if err != nil {
		t.Fatalf("loadWindowLayoutMenu returned error: %v", err)
	}
	if len(items) != 8 {
		t.Fatalf("expected 8 items (7 named + current-layout), got %d", len(items))
	}
	last := items[len(items)-1]
	if last.ID != "bb62,159x48,0,0{79x48,0,0,79x48,80,0}" {
		t.Fatalf("expected current layout ID, got %q", last.ID)
	}
	if last.Label != "current layout" {
		t.Fatalf("expected label 'current layout', got %q", last.Label)
	}
}

func TestLoadWindowLayoutMenuOmitsCurrentWhenEmpty(t *testing.T) {
	ctx := Context{}
	items, err := loadWindowLayoutMenu(ctx)
	if err != nil {
		t.Fatalf("loadWindowLayoutMenu returned error: %v", err)
	}
	if len(items) != 7 {
		t.Fatalf("expected 7 items, got %d", len(items))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run TestLoadWindowLayoutMenu -v`
Expected: FAIL — `loadWindowLayoutMenu` doesn't accept Context or return current-layout.

**Step 3: Update loadWindowLayoutMenu**

Change the signature to accept Context and add current-layout item:

```go
func loadWindowLayoutMenu(ctx Context) ([]Item, error) {
	layouts := []string{
		"even-horizontal",
		"even-vertical",
		"main-horizontal",
		"main-vertical",
		"tiled",
		"main-horizontal-mirrored",
		"main-vertical-mirrored",
	}
	items := menuItemsFromIDs(layouts)
	if layout := strings.TrimSpace(ctx.CurrentWindowLayout); layout != "" {
		items = append(items, Item{ID: layout, Label: "current layout"})
	}
	return items, nil
}
```

Note: `loadWindowLayoutMenu` already has the `Loader` signature `func(Context) ([]Item, error)` since all loaders accept Context. We just need to use the `ctx` parameter.

**Step 4: Run test to verify it passes**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run TestLoadWindowLayoutMenu -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `make test`
Expected: All pass.

**Step 6: Commit**

```
feat: add current-layout item to window layout submenu
```

---

### Task 3: Add previewKindLayout and live apply on cursor change

Add the new preview kind that applies layouts via `selectLayoutFn` when the cursor moves, and saves the original layout to `level.Data` on first invocation.

**Files:**
- Modify: `internal/ui/preview.go:14-19,51-52,96-138,277-290` (add previewKindLayout, update switch)
- New message type: `layoutAppliedMsg` in `internal/ui/preview.go`

**Step 1: Write the failing test**

In `internal/ui/preview_test.go` (new file if needed, or add to existing test file):

```go
func TestLayoutPreviewAppliesOnCursorMove(t *testing.T) {
	var applied []string
	selectLayoutFn = func(socket, layout string) error {
		applied = append(applied, layout)
		return nil
	}
	defer func() { selectLayoutFn = tmux.SelectLayout }()

	model := NewModel("test.sock", 80, 24, false, false, nil, "window:layout", "", "", "")
	h := NewHarness(model)

	// Simulate the layout menu being loaded with items
	items := []menu.Item{
		{ID: "even-horizontal", Label: "Even Horizontal"},
		{ID: "even-vertical", Label: "Even Vertical"},
		{ID: "tiled", Label: "Tiled"},
		{ID: "bb62,159x48,0,0", Label: "current layout"},
	}
	lvl := newLevel("window:layout", "Layout", items, nil)
	model.stack = append(model.stack, lvl)

	// Move cursor down — should apply "even-vertical"
	h.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	if len(applied) == 0 {
		t.Fatal("expected layout to be applied on cursor move")
	}
	if applied[len(applied)-1] != "even-vertical" {
		t.Fatalf("expected even-vertical, got %q", applied[len(applied)-1])
	}
}
```

Note: The exact test structure may need adjustment based on how `selectLayoutFn` is accessed from the `ui` package (it's in `menu` package). We may need an injectable `layoutPreviewFn` on the Model or use the existing `selectLayoutFn` from menu package. Adjust imports as needed.

**Step 2: Add previewKindLayout constant**

In `internal/ui/preview.go`, add the constant:

```go
const (
	previewKindNone previewKind = iota
	previewKindSession
	previewKindWindow
	previewKindPane
)

const previewKindTree previewKind = 10
const previewKindLayout previewKind = 11
```

**Step 3: Update previewKindForLevel**

```go
func previewKindForLevel(id string) previewKind {
	switch id {
	case "session:switch":
		return previewKindSession
	case "window:switch":
		return previewKindWindow
	case "pane:switch", "pane:join":
		return previewKindPane
	case "session:tree":
		return previewKindTree
	case "window:layout":
		return previewKindLayout
	default:
		return previewKindNone
	}
}
```

**Step 4: Add layoutAppliedMsg and injectable fn**

Add a new message type and an injectable function var to `internal/ui/preview.go`:

```go
type layoutAppliedMsg struct {
	levelID string
	seq     int
	err     error
}

var layoutPreviewFn = tmux.SelectLayout
```

**Step 5: Add layout branch to ensurePreviewForLevel**

In the switch on `kind`, add:

```go
case previewKindLayout:
	// Save original layout on first visit.
	if level.Data == nil {
		// Find the current-layout item — its ID is the raw layout string.
		for _, it := range level.Items {
			if it.Label == "current layout" {
				level.Data = it.ID
				break
			}
		}
		// Fallback: store empty string so we don't re-scan.
		if level.Data == nil {
			level.Data = ""
		}
	}
	return func() tea.Msg {
		err := layoutPreviewFn(socket, target)
		return layoutAppliedMsg{levelID: levelID, seq: seq, err: err}
	}
```

**Step 6: Register handler for layoutAppliedMsg**

In `internal/ui/model.go`, in `registerHandlers()`, add:

```go
reflect.TypeOf(layoutAppliedMsg{}): m.handleLayoutAppliedMsg,
```

And add the handler in `internal/ui/preview.go`:

```go
func (m *Model) handleLayoutAppliedMsg(msg tea.Msg) tea.Cmd {
	update, ok := msg.(layoutAppliedMsg)
	if !ok {
		return nil
	}
	if m.preview == nil {
		return nil
	}
	data, ok := m.preview[update.levelID]
	if !ok {
		return nil
	}
	if data.seq != update.seq {
		return nil
	}
	data.loading = false
	if update.err != nil {
		data.err = update.err.Error()
	}
	return nil
}
```

**Step 7: Suppress preview panel rendering for layout kind**

In `internal/ui/view.go`, wherever the preview panel is rendered, skip rendering when `kind == previewKindLayout`. The preview data exists (for dedup tracking) but no visual panel should appear. Check `activePreview()` usage in `view.go` — if the preview has no lines and no error, it likely already renders nothing, but verify.

**Step 8: Run tests**

Run: `make test`
Expected: All pass.

**Step 9: Commit**

```
feat: apply window layout in real-time on cursor move
```

---

### Task 4: Revert layout on Escape

When escaping from `window:layout`, revert to the original layout stored in `level.Data`.

**Files:**
- Modify: `internal/ui/navigation.go:14-47` (handleEscapeKey)

**Step 1: Write the failing test**

```go
func TestLayoutPreviewRevertsOnEscape(t *testing.T) {
	var applied []string
	layoutPreviewFn = func(socket, layout string) error {
		applied = append(applied, layout)
		return nil
	}
	defer func() { layoutPreviewFn = tmux.SelectLayout }()

	model := NewModel("test.sock", 80, 24, false, false, nil, "", "", "", "")
	h := NewHarness(model)

	// Simulate being in window:layout with original layout saved
	items := []menu.Item{
		{ID: "even-horizontal", Label: "Even Horizontal"},
		{ID: "tiled", Label: "Tiled"},
		{ID: "original-layout-string", Label: "current layout"},
	}
	root := model.stack[0]
	lvl := newLevel("window:layout", "Layout", items, nil)
	lvl.Data = "original-layout-string"
	model.stack = append(model.stack, lvl)
	_ = root

	applied = nil // reset
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	if len(applied) == 0 {
		t.Fatal("expected revert on escape")
	}
	if applied[0] != "original-layout-string" {
		t.Fatalf("expected revert to original layout, got %q", applied[0])
	}
}
```

**Step 2: Update handleEscapeKey**

In `internal/ui/navigation.go`, add revert logic before clearing preview:

```go
func (m *Model) handleEscapeKey() tea.Cmd {
	current := m.currentLevel()
	if current == nil {
		return tea.Quit
	}
	if len(m.stack) <= 1 {
		return tea.Quit
	}
	if current.ID == "window:swap-target" {
		m.pendingWindowSwap = nil
	}
	if current.ID == "pane:swap-target" {
		m.pendingPaneSwap = nil
	}

	// Revert layout preview on escape.
	var revertCmd tea.Cmd
	if current.ID == "window:layout" {
		if original, ok := current.Data.(string); ok && original != "" {
			socket := m.socketPath
			revertCmd = func() tea.Msg {
				err := layoutPreviewFn(socket, original)
				return layoutAppliedMsg{levelID: "window:layout", err: err}
			}
		}
	}

	if current != nil {
		m.clearPreview(current.ID)
	}
	parent := m.stack[len(m.stack)-2]
	m.stack = m.stack[:len(m.stack)-1]
	// ... rest unchanged ...

	parentPreviewCmd := m.ensurePreviewForLevel(parent)
	if revertCmd != nil && parentPreviewCmd != nil {
		return tea.Batch(revertCmd, parentPreviewCmd)
	}
	if revertCmd != nil {
		return revertCmd
	}
	return parentPreviewCmd
}
```

**Step 3: Run tests**

Run: `make test`
Expected: All pass.

**Step 4: Commit**

```
feat: revert window layout on escape from layout menu
```

---

### Task 5: Skip revert when Enter confirms layout

When the user presses Enter on a layout item, the layout is already applied. The existing `WindowLayoutAction` fires and returns `ActionResult`. No revert should happen. Verify the current flow handles this correctly — `handleEnterKey` does NOT call `handleEscapeKey`, so the revert code won't fire.

**Files:**
- (Verification only — no changes expected)

**Step 1: Write verification test**

```go
func TestLayoutPreviewNoRevertOnEnter(t *testing.T) {
	var applied []string
	layoutPreviewFn = func(socket, layout string) error {
		applied = append(applied, layout)
		return nil
	}
	selectLayoutFn = func(socket, layout string) error {
		applied = append(applied, "action:"+layout)
		return nil
	}
	defer func() {
		layoutPreviewFn = tmux.SelectLayout
		selectLayoutFn = tmux.SelectLayout
	}()

	// Build model with window:layout level loaded
	// Move cursor to "tiled"
	// Press Enter
	// Verify "original-layout-string" was NOT re-applied
}
```

**Step 2: Run test, verify it passes without changes**

Run: `make test`
Expected: All pass.

**Step 3: Commit (if any changes were needed)**

---

### Task 6: Add WindowLayout event tracer

**Files:**
- Already done: `internal/logging/events/window.go` has `Layout(layout string)` from the first changeset.

Verify it's called in `WindowLayoutAction`. Already done. Skip this task.

---

### Task 7: Final integration check

**Step 1: Run full test suite**

Run: `make test`
Expected: All pass.

**Step 2: Build binary**

Run: `make build`
Expected: Clean build.

**Step 3: Commit any remaining changes**

```
feat: window layout live preview with escape-revert
```
