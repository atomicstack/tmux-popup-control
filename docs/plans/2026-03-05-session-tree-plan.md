# Session Tree Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `session:tree` menu item that displays an interactive, filterable tree of sessions/windows/panes with pane-capture previews.

**Architecture:** A new tree package (`internal/menu/tree/`) owns the data model (expand state, flat-item building, tree-aware filtering). The menu system registers `session:tree` as an action loader. The UI layer intercepts Left/Right keys on tree levels and delegates rendering to a `renderTreeView` function that uses lipgloss's `tree` sub-package. Preview reuses the existing `previewKindPane` path.

**Tech Stack:** lipgloss v0.13.1 `tree` sub-package for rendering, existing bubbletea/bubbles/lipgloss for everything else.

---

### Task 1: Upgrade lipgloss to v0.13.1

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Bump the lipgloss dependency**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw go get github.com/charmbracelet/lipgloss@v0.13.1
```

**Step 2: Tidy modules**

Run: `make tidy`

**Step 3: Build and test**

Run: `make build && make test`
Expected: All tests pass, binary builds.

**Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: bump lipgloss to v0.13.1 for tree sub-package"
```

---

### Task 2: Add `--menu-args` CLI flag

**Files:**
- Modify: `internal/config/config.go:31-42` (env const), `config.go:50-108` (LoadArgs)
- Modify: `internal/app/app.go:15-25` (Config struct), `app.go:39` (NewModel call)
- Modify: `internal/ui/model.go:91` (NewModel signature), `model.go:46-86` (Model struct)
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestMenuArgsFlag(t *testing.T) {
	cfg, err := LoadArgs([]string{"--menu-args", "expanded"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.MenuArgs != "expanded" {
		t.Fatalf("expected MenuArgs=expanded, got %q", cfg.App.MenuArgs)
	}
}

func TestMenuArgsEnvVar(t *testing.T) {
	cfg, err := LoadArgs(nil, []string{"TMUX_POPUP_CONTROL_MENU_ARGS=expanded"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.MenuArgs != "expanded" {
		t.Fatalf("expected MenuArgs=expanded, got %q", cfg.App.MenuArgs)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/config/... -run TestMenuArgs -v
```
Expected: FAIL — `cfg.App.MenuArgs` field doesn't exist.

**Step 3: Implement**

In `internal/app/app.go`, add `MenuArgs` to Config struct (after `RootMenu` on line 22):
```go
type Config struct {
	SocketPath  string
	Width       int
	Height      int
	ShowFooter  bool
	Verbose     bool
	RootMenu    string
	MenuArgs    string  // <-- add
	ClientID    string
	SessionName string
}
```

In `internal/config/config.go`, add env const (after `envRootMenu` on line 39):
```go
envMenuArgs   = "TMUX_POPUP_CONTROL_MENU_ARGS"
```

In `LoadArgs()` (after `rootMenu` flag on line 62), add:
```go
menuArgs := fs.String("menu-args", envOrDefault(env, envMenuArgs, ""), "arguments for the target menu (e.g. 'expanded' for session:tree)")
```

In the Config construction (after `RootMenu` on line 83), add:
```go
MenuArgs:    strings.TrimSpace(*menuArgs),
```

Also add to the `Flags` map:
```go
"menuArgs": strings.TrimSpace(*menuArgs),
```

In `internal/ui/model.go`, update `NewModel` signature (line 91) to accept `menuArgs string`, and store it on the Model struct. Add field `menuArgs string` to the Model struct (after `rootMenuID` on line 75).

In `internal/app/app.go` `Run()` (line 39), pass `cfg.MenuArgs` to `NewModel`.

**Step 4: Run tests**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/config/... -run TestMenuArgs -v
```
Expected: PASS

**Step 5: Run full test suite**

Run: `make test`
Expected: All tests pass. Some existing tests that call `NewModel` will need the new parameter added — fix any compilation errors by adding `""` for the `menuArgs` param.

**Step 6: Commit**

```bash
git add internal/config/ internal/app/ internal/ui/
git commit -m "config: add --menu-args flag for per-menu options"
```

---

### Task 3: Create tree data model package

**Files:**
- Create: `internal/menu/tree/tree.go`
- Create: `internal/menu/tree/tree_test.go`

This package owns the expand/collapse state, flat-item building, and tree-aware filtering. It has no UI or lipgloss dependencies — pure data logic.

**Step 1: Write the failing test for `BuildItems`**

Create `internal/menu/tree/tree_test.go`:

```go
package tree

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestBuildItemsCollapsed(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 2, Current: true},
		{Name: "work", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", SessionName: "main"},
		{ID: "@2", Label: "vim", SessionName: "main"},
		{ID: "@3", Label: "htop", SessionName: "work"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", WindowID: "@1", SessionName: "main"},
		{ID: "%2", Label: "pane-2", WindowID: "@2", SessionName: "main"},
		{ID: "%3", Label: "pane-3", WindowID: "@3", SessionName: "work"},
	}

	state := NewState(false) // all collapsed
	items := state.BuildItems(sessions, windows, panes)

	// Collapsed: only session rows visible
	if len(items) != 2 {
		t.Fatalf("expected 2 items (sessions only), got %d", len(items))
	}
	if items[0].ID != "tree:s:main" {
		t.Errorf("expected tree:s:main, got %s", items[0].ID)
	}
	if items[1].ID != "tree:s:work" {
		t.Errorf("expected tree:s:work, got %s", items[1].ID)
	}
}

func TestBuildItemsExpanded(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 2},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", SessionName: "main"},
		{ID: "@2", Label: "vim", SessionName: "main"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", WindowID: "@1", SessionName: "main"},
	}

	state := NewState(false)
	state.SetExpanded("tree:s:main", true)
	items := state.BuildItems(sessions, windows, panes)

	// Session expanded: session + 2 windows visible (windows collapsed, so no panes)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != "tree:s:main" {
		t.Errorf("expected tree:s:main, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:main:@1" {
		t.Errorf("expected tree:w:main:@1, got %s", items[1].ID)
	}
	if items[2].ID != "tree:w:main:@2" {
		t.Errorf("expected tree:w:main:@2, got %s", items[2].ID)
	}
}

func TestBuildItemsFullyExpanded(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", SessionName: "main"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", WindowID: "@1", SessionName: "main"},
		{ID: "%2", Label: "pane-2", WindowID: "@1", SessionName: "main"},
	}

	state := NewState(true) // all expanded
	items := state.BuildItems(sessions, windows, panes)

	// Fully expanded: session + window + 2 panes
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	if items[0].ID != "tree:s:main" {
		t.Errorf("[0] expected tree:s:main, got %s", items[0].ID)
	}
	if items[1].ID != "tree:w:main:@1" {
		t.Errorf("[1] expected tree:w:main:@1, got %s", items[1].ID)
	}
	if items[2].ID != "tree:p:main:@1:%1" {
		t.Errorf("[2] expected tree:p:main:@1:%%1, got %s", items[2].ID)
	}
	if items[3].ID != "tree:p:main:@1:%2" {
		t.Errorf("[3] expected tree:p:main:@1:%%2, got %s", items[3].ID)
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/tree/... -v
```
Expected: FAIL — package doesn't exist.

**Step 3: Implement `internal/menu/tree/tree.go`**

```go
package tree

import (
	"fmt"
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// Item ID prefixes.
const (
	PrefixSession = "tree:s:"
	PrefixWindow  = "tree:w:"
	PrefixPane    = "tree:p:"
)

// State tracks expand/collapse for each tree node.
type State struct {
	expanded   map[string]bool
	allDefault bool // default expand state for nodes not in the map
}

// NewState creates a new tree state. If allExpanded is true, all nodes
// start expanded; otherwise all start collapsed.
func NewState(allExpanded bool) *State {
	return &State{
		expanded:   make(map[string]bool),
		allDefault: allExpanded,
	}
}

// IsExpanded returns whether the node with the given item ID is expanded.
func (s *State) IsExpanded(id string) bool {
	if v, ok := s.expanded[id]; ok {
		return v
	}
	return s.allDefault
}

// SetExpanded sets the expand state for the given item ID.
func (s *State) SetExpanded(id string, expanded bool) {
	s.expanded[id] = expanded
}

// Toggle flips the expand state for the given item ID.
func (s *State) Toggle(id string) {
	s.expanded[id] = !s.IsExpanded(id)
}

// IsExpandable returns true if the item ID represents a session or window
// (nodes that can have children).
func IsExpandable(id string) bool {
	return strings.HasPrefix(id, PrefixSession) || strings.HasPrefix(id, PrefixWindow)
}

// ItemKind returns "session", "window", or "pane" for an item ID.
func ItemKind(id string) string {
	switch {
	case strings.HasPrefix(id, PrefixSession):
		return "session"
	case strings.HasPrefix(id, PrefixWindow):
		return "window"
	case strings.HasPrefix(id, PrefixPane):
		return "pane"
	default:
		return ""
	}
}

// SessionID formats a session item ID.
func SessionID(name string) string {
	return PrefixSession + name
}

// WindowID formats a window item ID.
func WindowID(sessionName, windowID string) string {
	return fmt.Sprintf("%s%s:%s", PrefixWindow, sessionName, windowID)
}

// PaneID formats a pane item ID.
func PaneID(sessionName, windowID, paneID string) string {
	return fmt.Sprintf("%s%s:%s:%s", PrefixPane, sessionName, windowID, paneID)
}

// BuildItems produces the flat item list based on current expand state.
// Sessions are listed in the order provided. Windows and panes appear
// under their parent only when the parent is expanded.
func (s *State) BuildItems(sessions []menu.SessionEntry, windows []menu.WindowEntry, panes []menu.PaneEntry) []menu.Item {
	// Index windows by session, panes by window.
	winBySession := make(map[string][]menu.WindowEntry)
	for _, w := range windows {
		winBySession[w.SessionName] = append(winBySession[w.SessionName], w)
	}
	paneByWindow := make(map[string][]menu.PaneEntry)
	for _, p := range panes {
		paneByWindow[p.WindowID] = append(paneByWindow[p.WindowID], p)
	}

	var items []menu.Item
	for _, sess := range sessions {
		sid := SessionID(sess.Name)
		items = append(items, menu.Item{ID: sid, Label: sess.Name})

		if !s.IsExpanded(sid) {
			continue
		}
		for _, win := range winBySession[sess.Name] {
			wid := WindowID(sess.Name, win.ID)
			items = append(items, menu.Item{ID: wid, Label: win.Label})

			if !s.IsExpanded(wid) {
				continue
			}
			for _, pane := range paneByWindow[win.ID] {
				pid := PaneID(sess.Name, win.ID, pane.ID)
				items = append(items, menu.Item{ID: pid, Label: pane.Label})
			}
		}
	}
	return items
}
```

**Step 4: Run tests**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/tree/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/menu/tree/
git commit -m "feat: add tree data model package for session tree"
```

---

### Task 4: Add tree-aware filtering

**Files:**
- Modify: `internal/menu/tree/tree.go`
- Modify: `internal/menu/tree/tree_test.go`

**Step 1: Write the failing test**

Add to `internal/menu/tree/tree_test.go`:

```go
func TestFilterItemsPreservesAncestors(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 2},
		{Name: "work", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", SessionName: "main"},
		{ID: "@2", Label: "vim", SessionName: "main"},
		{ID: "@3", Label: "htop", SessionName: "work"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "pane-1", WindowID: "@1", SessionName: "main"},
		{ID: "%2", Label: "vim-pane", WindowID: "@2", SessionName: "main"},
		{ID: "%3", Label: "htop-pane", WindowID: "@3", SessionName: "work"},
	}

	state := NewState(false)
	items := state.FilterItems(sessions, windows, panes, "vim")

	// "vim" matches window "vim" in session "main" and pane "vim-pane".
	// Ancestors (session "main", window "@2") must be preserved.
	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID)
	}

	// Must contain session:main (ancestor), window vim (match), and its pane (child of match)
	if len(items) < 2 {
		t.Fatalf("expected at least 2 items, got %d: %v", len(items), ids)
	}

	// Session "main" must be present as ancestor of matched window "vim"
	found := false
	for _, item := range items {
		if item.ID == "tree:s:main" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ancestor session main to be preserved, got %v", ids)
	}
}

func TestFilterItemsNoMatchReturnsEmpty(t *testing.T) {
	sessions := []menu.SessionEntry{{Name: "main", Windows: 1}}
	windows := []menu.WindowEntry{{ID: "@1", Label: "bash", SessionName: "main"}}
	panes := []menu.PaneEntry{{ID: "%1", Label: "pane-1", WindowID: "@1", SessionName: "main"}}

	state := NewState(false)
	items := state.FilterItems(sessions, windows, panes, "zzzznotfound")
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestFilterItemsEmptyQueryReturnsNormal(t *testing.T) {
	sessions := []menu.SessionEntry{{Name: "main", Windows: 1}}
	windows := []menu.WindowEntry{{ID: "@1", Label: "bash", SessionName: "main"}}
	panes := []menu.PaneEntry{}

	state := NewState(false)
	items := state.FilterItems(sessions, windows, panes, "")
	// Empty query: same as BuildItems (collapsed = sessions only)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/tree/... -run TestFilter -v
```
Expected: FAIL — `FilterItems` doesn't exist.

**Step 3: Implement `FilterItems`**

Add to `internal/menu/tree/tree.go`:

```go
// FilterItems produces a flat item list filtered by query.
// Matching is case-insensitive substring on labels and IDs.
// Matched items keep their ancestor chain visible. All children of a
// matched session or window are included. When query is empty, falls
// back to BuildItems with current expand state.
func (s *State) FilterItems(sessions []menu.SessionEntry, windows []menu.WindowEntry, panes []menu.PaneEntry, query string) []menu.Item {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return s.BuildItems(sessions, windows, panes)
	}

	lower := strings.ToLower(trimmed)

	// Index windows by session, panes by window.
	winBySession := make(map[string][]menu.WindowEntry)
	for _, w := range windows {
		winBySession[w.SessionName] = append(winBySession[w.SessionName], w)
	}
	paneByWindow := make(map[string][]menu.PaneEntry)
	for _, p := range panes {
		paneByWindow[p.WindowID] = append(paneByWindow[p.WindowID], p)
	}

	var items []menu.Item
	for _, sess := range sessions {
		sid := SessionID(sess.Name)
		sessionMatches := containsFold(sess.Name, lower)

		var sessionChildren []menu.Item
		for _, win := range winBySession[sess.Name] {
			wid := WindowID(sess.Name, win.ID)
			windowMatches := containsFold(win.Label, lower) || containsFold(win.ID, lower)

			var windowChildren []menu.Item
			for _, pane := range paneByWindow[win.ID] {
				pid := PaneID(sess.Name, win.ID, pane.ID)
				paneMatches := containsFold(pane.Label, lower) || containsFold(pane.ID, lower)
				if paneMatches || windowMatches || sessionMatches {
					windowChildren = append(windowChildren, menu.Item{ID: pid, Label: pane.Label})
				}
			}

			if sessionMatches || windowMatches || len(windowChildren) > 0 {
				sessionChildren = append(sessionChildren, menu.Item{ID: wid, Label: win.Label})
				sessionChildren = append(sessionChildren, windowChildren...)
			}
		}

		if sessionMatches || len(sessionChildren) > 0 {
			items = append(items, menu.Item{ID: sid, Label: sess.Name})
			items = append(items, sessionChildren...)
		}
	}
	return items
}

func containsFold(s, lowerSubstr string) bool {
	return strings.Contains(strings.ToLower(s), lowerSubstr)
}
```

**Step 4: Run tests**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/tree/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/menu/tree/
git commit -m "feat: add tree-aware filtering with ancestor preservation"
```

---

### Task 5: Register `session:tree` in the menu system

**Files:**
- Modify: `internal/menu/session.go:15-26` (loadSessionMenu)
- Modify: `internal/menu/menu.go:149-175` (ActionLoaders)
- Modify: `internal/menu/session.go` (add loadSessionTreeMenu loader)
- Test: `internal/menu/session_test.go`

**Step 1: Write the failing test**

Add to `internal/menu/session_test.go` (or create it if needed):

```go
func TestSessionMenuIncludesTree(t *testing.T) {
	items, err := loadSessionMenu(menu.Context{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.ID == "tree" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected session menu to include 'tree' item")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run TestSessionMenuIncludesTree -v
```
Expected: FAIL

**Step 3: Implement**

In `internal/menu/session.go` `loadSessionMenu()` (line 15-26), add `"tree"` to the items list:
```go
func loadSessionMenu(Context) ([]Item, error) {
	items := []string{
		// vvv do NOT reorder these! vvv
		"kill",
		"detach",
		"rename",
		"new",
		"switch",
		"tree",
		// ^^^ do NOT reorder these! ^^^
	}
	return menuItemsFromIDs(items), nil
}
```

Add the tree loader function in `internal/menu/session.go`:
```go
func loadSessionTreeMenu(ctx Context) ([]Item, error) {
	allExpanded := strings.TrimSpace(ctx.MenuArgs) == "expanded"
	treeState := tree.NewState(allExpanded)

	items := treeState.BuildItems(ctx.Sessions, ctx.Windows, ctx.Panes)
	return items, nil
}
```

In `internal/menu/menu.go` `ActionLoaders()`, add:
```go
"session:tree": loadSessionTreeMenu,
```

The `Context` struct needs a `MenuArgs` field and access to `Windows` and `Panes` — check that these already exist (they do per the design doc exploration: `Context` has `Sessions`, `Windows`, `Panes` fields). Add `MenuArgs string` to `Context` if not present.

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/menu/
git commit -m "feat: register session:tree menu item with loader"
```

---

### Task 6: Wire `MenuArgs` through to menu Context

**Files:**
- Modify: `internal/menu/menu.go:24-39` (Context struct)
- Modify: `internal/ui/commands.go` (where menuContext is built)
- Test: Covered by existing test suite compilation

**Step 1: Check if `MenuArgs` is already on Context**

Read `internal/menu/menu.go` lines 24-39 to see the Context struct. If `MenuArgs` is not present, add it.

**Step 2: Add `MenuArgs` to Context**

In `internal/menu/menu.go`, add `MenuArgs string` to the `Context` struct.

**Step 3: Wire it in the UI**

In `internal/ui/commands.go` (or wherever `menuContext()` builds a `menu.Context`), set `MenuArgs: m.menuArgs`.

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/menu/ internal/ui/
git commit -m "feat: wire MenuArgs through to menu Context"
```

---

### Task 7: Tree rendering with lipgloss

**Files:**
- Create: `internal/ui/tree.go`
- Create: `internal/ui/tree_test.go`

**Step 1: Write the failing test**

Create `internal/ui/tree_test.go`:

```go
func TestRenderTreeViewCollapsed(t *testing.T) {
	h := NewHarness("", 80, 24, false, false, nil, "session:tree", "", "")
	items := []menu.Item{
		{ID: "tree:s:main", Label: "main"},
		{ID: "tree:s:work", Label: "work"},
	}
	treeState := tree.NewState(false)
	h.Model.currentLevel().UpdateItems(items)
	h.Model.currentLevel().Data = treeState

	output := h.Model.renderTreeLines(items, treeState, 0, 80)
	if len(output) == 0 {
		t.Fatal("expected non-empty tree output")
	}
	// Should contain collapse indicators
	combined := strings.Join(output, "\n")
	if !strings.Contains(combined, "▶") {
		t.Error("expected ▶ indicator for collapsed sessions")
	}
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestRenderTreeView -v
```
Expected: FAIL

**Step 3: Implement `internal/ui/tree.go`**

```go
package ui

import (
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	menutree "github.com/atomicstack/tmux-popup-control/internal/menu/tree"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/tree"
)

// renderTreeLines builds the visual tree from the flat item list using
// lipgloss/tree for rendering. Returns one string per display line.
// cursorIdx is the index into items of the currently highlighted item.
func (m *Model) renderTreeLines(items []menu.Item, state *menutree.State, cursorIdx int, width int) []string {
	if len(items) == 0 {
		return nil
	}

	// Build a tree per session, then concatenate their rendered output.
	var allLines []string
	flatIdx := 0

	for flatIdx < len(items) {
		item := items[flatIdx]
		if menutree.ItemKind(item.ID) != "session" {
			// Orphan non-session item; render as plain line.
			style := m.treeItemStyle(flatIdx, cursorIdx)
			allLines = append(allLines, style.Render(padRight(item.Label, width)))
			flatIdx++
			continue
		}

		// Session node — build a lipgloss tree rooted at this session.
		expanded := state.IsExpanded(item.ID)
		indicator := "▶"
		if expanded {
			indicator = "▼"
		}
		sessionLabel := indicator + " " + item.Label
		t := tree.Root(m.treeStyledNode(sessionLabel, flatIdx, cursorIdx, width))
		sessionIdx := flatIdx
		flatIdx++

		// Collect child windows.
		for flatIdx < len(items) && menutree.ItemKind(items[flatIdx].ID) != "session" {
			winItem := items[flatIdx]
			if menutree.ItemKind(winItem.ID) == "window" {
				winExpanded := state.IsExpanded(winItem.ID)
				winIndicator := "▶"
				if winExpanded {
					winIndicator = "▼"
				}
				winLabel := winIndicator + " " + winItem.Label
				winTree := tree.Root(m.treeStyledNode(winLabel, flatIdx, cursorIdx, width))
				winIdx := flatIdx
				flatIdx++

				// Collect child panes.
				for flatIdx < len(items) && menutree.ItemKind(items[flatIdx].ID) == "pane" {
					paneItem := items[flatIdx]
					winTree.Child(m.treeStyledNode(paneItem.Label, flatIdx, cursorIdx, width))
					flatIdx++
				}
				_ = winIdx
				t.Child(winTree)
			} else {
				// Pane directly under session (shouldn't happen, but handle gracefully)
				t.Child(m.treeStyledNode(winItem.Label, flatIdx, cursorIdx, width))
				flatIdx++
			}
		}
		_ = sessionIdx

		// Render this session's tree and split into lines.
		rendered := t.String()
		if rendered != "" {
			allLines = append(allLines, strings.Split(rendered, "\n")...)
		}
	}

	return allLines
}

// treeStyledNode returns the styled label string for a tree node.
func (m *Model) treeStyledNode(label string, idx, cursorIdx, width int) string {
	style := m.treeItemStyle(idx, cursorIdx)
	return style.Render(label)
}

// treeItemStyle returns the lipgloss style for an item at the given index.
func (m *Model) treeItemStyle(idx, cursorIdx int) lipgloss.Style {
	if idx == cursorIdx {
		if styles.SelectedItem != nil {
			return styles.SelectedItem.Copy()
		}
		return lipgloss.NewStyle().Bold(true).Reverse(true)
	}
	if styles.Item != nil {
		return styles.Item.Copy()
	}
	return lipgloss.NewStyle()
}

func padRight(s string, width int) string {
	if pad := width - len([]rune(s)); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// isTreeLevel returns true if the given level ID is the session tree.
func isTreeLevel(id string) bool {
	return id == "session:tree"
}
```

**Step 4: Run tests**

Run:
```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestRenderTreeView -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ui/tree.go internal/ui/tree_test.go
git commit -m "feat: add tree rendering using lipgloss tree package"
```

---

### Task 8: Integrate tree rendering into view.go

**Files:**
- Modify: `internal/ui/view.go:90-179` (viewVertical), `view.go:182-287` (viewSideBySide)
- Test: `internal/ui/tree_test.go` (add view integration tests)

**Step 1: Write the failing test**

Add to `internal/ui/tree_test.go`:

```go
func TestTreeLevelRendersTreeInsteadOfItems(t *testing.T) {
	h := NewHarness("", 80, 24, false, false, nil, "", "", "")
	treeState := tree.NewState(false)
	items := []menu.Item{
		{ID: "tree:s:main", Label: "main"},
		{ID: "tree:s:work", Label: "work"},
	}
	lvl := newLevel("session:tree", "tree", items, nil)
	lvl.Data = treeState
	h.Model.stack = append(h.Model.stack, lvl)

	view := h.Model.View()
	if !strings.Contains(view, "▶") {
		t.Error("expected tree view to contain ▶ indicators")
	}
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — tree level renders with `buildItemLine` (no ▶ indicators).

**Step 3: Implement**

In `viewVertical()` and `viewSideBySide()`, add a branch before the item rendering loop:

```go
if isTreeLevel(current.ID) {
	treeState, _ := current.Data.(*menutree.State)
	treeLines := m.renderTreeLines(current.Items, treeState, current.Cursor, width)
	// Apply viewport offset and height limit, same as for regular items.
	start, end := m.viewportRange(current, len(treeLines), availableHeight)
	for i := start; i < end; i++ {
		lines = append(lines, styledLine{text: treeLines[i]})
	}
} else {
	// existing buildItemLine loop
}
```

The exact integration depends on how viewport slicing works in each function. The pattern is: replace the `for i, item := range displayItems` loop with the tree path when `isTreeLevel` is true.

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ui/view.go internal/ui/tree.go internal/ui/tree_test.go
git commit -m "feat: integrate tree rendering into view layout"
```

---

### Task 9: Left/Right key navigation for tree levels

**Files:**
- Modify: `internal/ui/input.go:123-138` (KeyLeft/KeyRight handlers)
- Modify: `internal/ui/navigation.go` (add tree expand/collapse helpers)
- Create: `internal/ui/tree_nav.go` (tree-specific navigation)
- Test: `internal/ui/tree_test.go`

**Step 1: Write the failing test**

Add to `internal/ui/tree_test.go`:

```go
func TestTreeRightExpandsCollapsedSession(t *testing.T) {
	sessions := []menu.SessionEntry{{Name: "main", Windows: 1}}
	windows := []menu.WindowEntry{{ID: "@1", Label: "bash", SessionName: "main"}}
	panes := []menu.PaneEntry{{ID: "%1", Label: "p1", WindowID: "@1", SessionName: "main"}}

	treeState := tree.NewState(false)
	items := treeState.BuildItems(sessions, windows, panes)

	h := NewHarness("", 80, 24, false, false, nil, "", "", "")
	lvl := newLevel("session:tree", "tree", items, nil)
	lvl.Data = treeState
	h.Model.stack = append(h.Model.stack, lvl)
	h.Model.currentLevel().Cursor = 0

	// Store the tree source data on the model so rebuilds work.
	h.Model.treesessions = sessions
	h.Model.treeWindows = windows
	h.Model.treePanes = panes

	// Right on collapsed session should expand it.
	h.SendKey(tea.KeyRight)

	current := h.Model.currentLevel()
	if len(current.Items) != 2 {
		t.Fatalf("expected 2 items after expand (session + window), got %d", len(current.Items))
	}
	if current.Cursor != 0 {
		t.Errorf("cursor should stay on session, got %d", current.Cursor)
	}
}

func TestTreeRightOnExpandedMovesCursorDown(t *testing.T) {
	sessions := []menu.SessionEntry{{Name: "main", Windows: 1}}
	windows := []menu.WindowEntry{{ID: "@1", Label: "bash", SessionName: "main"}}
	panes := []menu.PaneEntry{}

	treeState := tree.NewState(false)
	treeState.SetExpanded("tree:s:main", true)
	items := treeState.BuildItems(sessions, windows, panes)

	h := NewHarness("", 80, 24, false, false, nil, "", "", "")
	lvl := newLevel("session:tree", "tree", items, nil)
	lvl.Data = treeState
	h.Model.stack = append(h.Model.stack, lvl)
	h.Model.currentLevel().Cursor = 0

	h.Model.treesessions = sessions
	h.Model.treeWindows = windows
	h.Model.treePanes = panes

	// Right on expanded session should move cursor down.
	h.SendKey(tea.KeyRight)

	if h.Model.currentLevel().Cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", h.Model.currentLevel().Cursor)
	}
}

func TestTreeLeftCollapsesExpandedSession(t *testing.T) {
	sessions := []menu.SessionEntry{{Name: "main", Windows: 1}}
	windows := []menu.WindowEntry{{ID: "@1", Label: "bash", SessionName: "main"}}
	panes := []menu.PaneEntry{}

	treeState := tree.NewState(false)
	treeState.SetExpanded("tree:s:main", true)
	items := treeState.BuildItems(sessions, windows, panes)

	h := NewHarness("", 80, 24, false, false, nil, "", "", "")
	lvl := newLevel("session:tree", "tree", items, nil)
	lvl.Data = treeState
	h.Model.stack = append(h.Model.stack, lvl)
	h.Model.currentLevel().Cursor = 0

	h.Model.treeSessions = sessions
	h.Model.treeWindows = windows
	h.Model.treePanes = panes

	// Left on expanded session should collapse it.
	h.SendKey(tea.KeyLeft)

	current := h.Model.currentLevel()
	if len(current.Items) != 1 {
		t.Fatalf("expected 1 item after collapse, got %d", len(current.Items))
	}
}

func TestTreeLeftOnCollapsedMovesCursorUp(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 1},
		{Name: "work", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", SessionName: "main"},
		{ID: "@2", Label: "htop", SessionName: "work"},
	}
	panes := []menu.PaneEntry{}

	treeState := tree.NewState(false)
	items := treeState.BuildItems(sessions, windows, panes)

	h := NewHarness("", 80, 24, false, false, nil, "", "", "")
	lvl := newLevel("session:tree", "tree", items, nil)
	lvl.Data = treeState
	h.Model.stack = append(h.Model.stack, lvl)
	h.Model.currentLevel().Cursor = 1 // on "work"

	h.Model.treeessions = sessions
	h.Model.treeWindows = windows
	h.Model.treePanes = panes

	// Left on collapsed session should move cursor up.
	h.SendKey(tea.KeyLeft)

	if h.Model.currentLevel().Cursor != 0 {
		t.Errorf("expected cursor at 0, got %d", h.Model.currentLevel().Cursor)
	}
}
```

**Step 2: Run tests to verify they fail**

Expected: FAIL

**Step 3: Implement**

Create `internal/ui/tree_nav.go`:

```go
package ui

import (
	menutree "github.com/atomicstack/tmux-popup-control/internal/menu/tree"
	tea "github.com/charmbracelet/bubbletea"
)

// handleTreeRight handles the Right arrow key on a tree level.
// - Collapsed node: expand it, stay on row.
// - Expanded node or leaf: move cursor down.
func (m *Model) handleTreeRight() tea.Cmd {
	current := m.currentLevel()
	if current == nil || len(current.Items) == 0 {
		return nil
	}
	item := current.Items[current.Cursor]
	treeState, _ := current.Data.(*menutree.State)
	if treeState == nil {
		return nil
	}

	if menutree.IsExpandable(item.ID) && !treeState.IsExpanded(item.ID) {
		treeState.SetExpanded(item.ID, true)
		m.rebuildTreeItems(current, treeState)
		return m.ensurePreviewForLevel(current)
	}

	// Already expanded or leaf: move down.
	if m.moveCursorDown() {
		return m.ensurePreviewForLevel(current)
	}
	return nil
}

// handleTreeLeft handles the Left arrow key on a tree level.
// - Expanded node: collapse it, stay on row.
// - Collapsed node or leaf: move cursor up.
func (m *Model) handleTreeLeft() tea.Cmd {
	current := m.currentLevel()
	if current == nil || len(current.Items) == 0 {
		return nil
	}
	item := current.Items[current.Cursor]
	treeState, _ := current.Data.(*menutree.State)
	if treeState == nil {
		return nil
	}

	if menutree.IsExpandable(item.ID) && treeState.IsExpanded(item.ID) {
		treeState.SetExpanded(item.ID, false)
		m.rebuildTreeItems(current, treeState)
		return m.ensurePreviewForLevel(current)
	}

	// Collapsed or leaf: move up.
	if m.moveCursorUp() {
		return m.ensurePreviewForLevel(current)
	}
	return nil
}

// rebuildTreeItems rebuilds the flat item list from current expand state.
func (m *Model) rebuildTreeItems(lvl *level, treeState *menutree.State) {
	cursorID := ""
	if lvl.Cursor >= 0 && lvl.Cursor < len(lvl.Items) {
		cursorID = lvl.Items[lvl.Cursor].ID
	}

	var items []menu.Item
	if lvl.Filter != "" {
		items = treeState.FilterItems(m.treeSessions, m.treeWindows, m.treePanes, lvl.Filter)
	} else {
		items = treeState.BuildItems(m.treeSessions, m.treeWindows, m.treePanes)
	}
	lvl.Items = items
	lvl.Full = items

	// Restore cursor to the same item if possible.
	for i, item := range items {
		if item.ID == cursorID {
			lvl.Cursor = i
			return
		}
	}
	if lvl.Cursor >= len(items) {
		lvl.Cursor = len(items) - 1
	}
}
```

Add fields to Model struct in `model.go`:
```go
treeSessions []menu.SessionEntry
treeWindows  []menu.WindowEntry
treePanes    []menu.PaneEntry
```

Modify `internal/ui/input.go` (lines 123-138) to intercept Left/Right for tree levels:

```go
case tea.KeyLeft:
	if isTreeLevel(current.ID) && current.Filter == "" {
		return true, m.handleTreeLeft()
	}
	// existing filter cursor logic
	before := current.FilterCursorPos()
	if !current.MoveFilterCursorRuneBackward() {
		return false, nil
	}
	// ...

case tea.KeyRight:
	if isTreeLevel(current.ID) && current.Filter == "" {
		return true, m.handleTreeRight()
	}
	// existing filter cursor logic
	before := current.FilterCursorPos()
	if !current.MoveFilterCursorRuneForward() {
		return false, nil
	}
	// ...
```

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add left/right arrow tree navigation"
```

---

### Task 10: Tree Enter action handler

**Files:**
- Modify: `internal/menu/session.go` (add `SessionTreeAction`)
- Modify: `internal/menu/menu.go:120-147` (ActionHandlers)
- Test: `internal/menu/session_test.go`

**Step 1: Write the failing test**

Add to `internal/menu/session_test.go`:

```go
func TestSessionTreeActionSwitchesSession(t *testing.T) {
	switched := ""
	origSwitch := switchClientFn
	switchClientFn = func(socket, client, target string) error {
		switched = target
		return nil
	}
	defer func() { switchClientFn = origSwitch }()

	ctx := Context{SocketPath: "/tmp/test.sock", ClientID: "client1"}
	item := Item{ID: "tree:s:work", Label: "work"}
	cmd := SessionTreeAction(ctx, item)
	msg := cmd()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatal("expected ActionResult")
	}
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	if switched != "work" {
		t.Errorf("expected switch to 'work', got %q", switched)
	}
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — `SessionTreeAction` doesn't exist.

**Step 3: Implement**

Add to `internal/menu/session.go`:

```go
// SessionTreeAction handles Enter on a tree item. It parses the item ID
// prefix to determine whether to switch session, window, or pane.
func SessionTreeAction(ctx Context, item Item) tea.Cmd {
	id := item.ID
	switch {
	case strings.HasPrefix(id, tree.PrefixPane):
		// tree:p:session:windowID:paneID
		parts := strings.SplitN(strings.TrimPrefix(id, tree.PrefixPane), ":", 3)
		if len(parts) < 3 {
			return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid pane target: %s", id)} }
		}
		session, paneID := parts[0], parts[2]
		return func() tea.Msg {
			events.Session.Switch(session)
			if err := switchClientFn(ctx.SocketPath, ctx.ClientID, session); err != nil {
				return ActionResult{Err: err}
			}
			if err := switchPaneFn(ctx.SocketPath, ctx.ClientID, paneID); err != nil {
				return ActionResult{Err: err}
			}
			return ActionResult{Info: fmt.Sprintf("Switched to pane %s", paneID)}
		}
	case strings.HasPrefix(id, tree.PrefixWindow):
		// tree:w:session:windowID
		parts := strings.SplitN(strings.TrimPrefix(id, tree.PrefixWindow), ":", 2)
		if len(parts) < 2 {
			return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid window target: %s", id)} }
		}
		session, windowID := parts[0], parts[1]
		return func() tea.Msg {
			events.Session.Switch(session)
			if err := switchClientFn(ctx.SocketPath, ctx.ClientID, session); err != nil {
				return ActionResult{Err: err}
			}
			if err := selectWindowFn(ctx.SocketPath, windowID); err != nil {
				return ActionResult{Err: err}
			}
			return ActionResult{Info: fmt.Sprintf("Switched to window %s", windowID)}
		}
	case strings.HasPrefix(id, tree.PrefixSession):
		session := strings.TrimPrefix(id, tree.PrefixSession)
		return func() tea.Msg {
			events.Session.Switch(session)
			if err := switchClientFn(ctx.SocketPath, ctx.ClientID, session); err != nil {
				return ActionResult{Err: err}
			}
			return ActionResult{Info: fmt.Sprintf("Switched to %s", session)}
		}
	default:
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("unknown tree item: %s", id)} }
	}
}
```

Register in `ActionHandlers()`:
```go
"session:tree": SessionTreeAction,
```

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/menu/
git commit -m "feat: add session tree Enter action for session/window/pane switching"
```

---

### Task 11: Preview support for tree level

**Files:**
- Modify: `internal/ui/preview.go:220-231` (previewKindForLevel)
- Modify: `internal/ui/preview.go:47-137` (ensurePreviewForLevel — add tree-specific pane resolution)
- Test: `internal/ui/tree_test.go`

**Step 1: Write the failing test**

Add to `internal/ui/tree_test.go`:

```go
func TestTreePreviewResolvesSessionPane(t *testing.T) {
	kind := previewKindForLevel("session:tree")
	if kind == previewKindNone {
		t.Fatal("expected preview kind for session:tree, got none")
	}
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — `session:tree` returns `previewKindNone`.

**Step 3: Implement**

In `previewKindForLevel()` (preview.go:220-231), add a case:
```go
case "session:tree":
	return previewKindPane
```

In `ensurePreviewForLevel()`, the `previewKindPane` case already calls `panePreviewFn` with a target pane ID. For tree items, the target must be resolved based on the item kind:
- Pane item → use the pane ID directly (parse from `tree:p:session:windowID:paneID`)
- Window item → use `activePaneIDForWindow`
- Session item → use `activePaneIDForSession`

Add a helper in `internal/ui/tree.go`:

```go
// treePreviewTarget resolves the pane ID for preview based on the tree item.
func (m *Model) treePreviewTarget(itemID string) string {
	switch {
	case strings.HasPrefix(itemID, menutree.PrefixPane):
		parts := strings.SplitN(strings.TrimPrefix(itemID, menutree.PrefixPane), ":", 3)
		if len(parts) >= 3 {
			return parts[2] // pane ID
		}
	case strings.HasPrefix(itemID, menutree.PrefixWindow):
		parts := strings.SplitN(strings.TrimPrefix(itemID, menutree.PrefixWindow), ":", 2)
		if len(parts) >= 2 {
			return m.activePaneIDForWindow(parts[1])
		}
	case strings.HasPrefix(itemID, menutree.PrefixSession):
		session := strings.TrimPrefix(itemID, menutree.PrefixSession)
		return m.activePaneIDForSession(session)
	}
	return ""
}
```

Then modify `ensurePreviewForLevel` to call `treePreviewTarget` when on a tree level, using the resolved pane ID as the target for `panePreviewFn`.

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add preview support for session tree level"
```

---

### Task 12: Tree-aware filter integration

**Files:**
- Modify: `internal/ui/tree_nav.go` (add filter override for tree levels)
- Modify: `internal/ui/input.go` (intercept filter changes on tree levels)
- Test: `internal/ui/tree_test.go`

**Step 1: Write the failing test**

Add to `internal/ui/tree_test.go`:

```go
func TestTreeFilterExpandsMatchingAncestors(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "main", Windows: 2},
		{Name: "work", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "bash", SessionName: "main"},
		{ID: "@2", Label: "vim", SessionName: "main"},
		{ID: "@3", Label: "htop", SessionName: "work"},
	}
	panes := []menu.PaneEntry{}

	treeState := tree.NewState(false)
	items := treeState.BuildItems(sessions, windows, panes)

	h := NewHarness("", 80, 24, false, false, nil, "", "", "")
	lvl := newLevel("session:tree", "tree", items, nil)
	lvl.Data = treeState
	h.Model.stack = append(h.Model.stack, lvl)
	h.Model.treeSessions = sessions
	h.Model.treeWindows = windows
	h.Model.treePanes = panes

	// Type "vim" into filter.
	h.SendText("vim")

	current := h.Model.currentLevel()
	// Should show session "main" (ancestor) and window "vim" (match).
	foundSession := false
	foundWindow := false
	for _, item := range current.Items {
		if item.ID == "tree:s:main" {
			foundSession = true
		}
		if strings.Contains(item.Label, "vim") {
			foundWindow = true
		}
	}
	if !foundSession || !foundWindow {
		var ids []string
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("expected session:main + vim window, got %v", ids)
	}
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — standard filtering doesn't understand tree structure.

**Step 3: Implement**

Override the standard `applyFilter` path for tree levels. When filter text changes on a tree level, instead of calling `FilterItems` on the flat list, call `treeState.FilterItems()` which preserves ancestors.

In `internal/ui/tree_nav.go`, add:

```go
// applyTreeFilter re-runs tree-aware filtering when the filter text changes.
func (m *Model) applyTreeFilter(lvl *level) {
	treeState, _ := lvl.Data.(*menutree.State)
	if treeState == nil {
		return
	}
	items := treeState.FilterItems(m.treeSessions, m.treeWindows, m.treePanes, lvl.Filter)
	lvl.Items = items
	lvl.Full = items
	if lvl.Cursor >= len(items) {
		lvl.Cursor = len(items) - 1
	}
	if lvl.Cursor < 0 && len(items) > 0 {
		lvl.Cursor = 0
	}
}
```

In `internal/ui/input.go`, after every filter mutation (appendToFilter, removeFilterRune, ctrl+u clear, ctrl+w word delete), add a check:

```go
if isTreeLevel(current.ID) {
	m.applyTreeFilter(current)
}
```

This replaces the standard `applyFilter` call for tree levels. The standard `SetFilter` / `applyFilter` path in `state/filter.go` still runs but its result gets overwritten by `applyTreeFilter`.

**Step 4: Run tests**

Run: `make test`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: add tree-aware filtering with ancestor preservation"
```

---

### Task 13: Populate tree source data from backend

**Files:**
- Modify: `internal/ui/commands.go` or `internal/ui/backend.go` (where backend events update stores)
- Modify: `internal/ui/tree_nav.go` (rebuild on backend update)

The tree level needs `treeSessions`, `treeWindows`, `treePanes` to be populated from the state stores. This should happen when the tree level is loaded, and when backend events update the stores.

**Step 1: Wire tree data on level load**

In the `loadSessionTreeMenu` loader (or in the UI's `handleCategoryLoadedMsg`), populate the Model's tree source data from the state stores:

```go
m.treeSessions = m.sessions.Entries()
m.treeWindows = m.windows.Entries()
m.treePanes = m.panes.Entries()
```

**Step 2: Refresh on backend updates**

In `handleBackendEventMsg` (or wherever session/window/pane stores are updated), if the current level is a tree level, refresh the tree source data and rebuild items:

```go
if current := m.currentLevel(); current != nil && isTreeLevel(current.ID) {
	m.treeSessions = m.sessions.Entries()
	m.treeWindows = m.windows.Entries()
	m.treePanes = m.panes.Entries()
	if treeState, ok := current.Data.(*menutree.State); ok {
		m.rebuildTreeItems(current, treeState)
	}
}
```

**Step 3: Run tests**

Run: `make test`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/ui/
git commit -m "feat: populate tree data from backend state stores"
```

---

### Task 14: Full integration test

**Files:**
- Test: `internal/ui/tree_test.go` (end-to-end harness test)

**Step 1: Write a comprehensive harness test**

```go
func TestTreeEndToEnd(t *testing.T) {
	sessions := []menu.SessionEntry{
		{Name: "dev", Windows: 2, Current: true},
		{Name: "ops", Windows: 1},
	}
	windows := []menu.WindowEntry{
		{ID: "@1", Label: "editor", SessionName: "dev"},
		{ID: "@2", Label: "terminal", SessionName: "dev"},
		{ID: "@3", Label: "monitor", SessionName: "ops"},
	}
	panes := []menu.PaneEntry{
		{ID: "%1", Label: "vim", WindowID: "@1", SessionName: "dev"},
		{ID: "%2", Label: "shell", WindowID: "@2", SessionName: "dev"},
		{ID: "%3", Label: "htop", WindowID: "@3", SessionName: "ops"},
	}

	h := NewHarness("", 80, 40, false, false, nil, "", "", "")
	treeState := tree.NewState(false)
	items := treeState.BuildItems(sessions, windows, panes)
	lvl := newLevel("session:tree", "tree", items, nil)
	lvl.Data = treeState
	h.Model.stack = append(h.Model.stack, lvl)
	h.Model.treeSessions = sessions
	h.Model.treeWindows = windows
	h.Model.treePanes = panes

	// 1. Both sessions collapsed.
	if len(h.Model.currentLevel().Items) != 2 {
		t.Fatalf("step 1: expected 2 items, got %d", len(h.Model.currentLevel().Items))
	}

	// 2. Right expands "dev".
	h.SendKey(tea.KeyRight)
	if len(h.Model.currentLevel().Items) != 4 {
		t.Fatalf("step 2: expected 4 items (dev + 2 windows + ops), got %d", len(h.Model.currentLevel().Items))
	}

	// 3. Down to first window, Right expands it to show panes.
	h.SendKey(tea.KeyDown) // cursor on editor window
	h.SendKey(tea.KeyRight) // expand editor
	if len(h.Model.currentLevel().Items) != 5 {
		t.Fatalf("step 3: expected 5 items, got %d", len(h.Model.currentLevel().Items))
	}

	// 4. Left collapses "editor" window.
	h.SendKey(tea.KeyLeft)
	if len(h.Model.currentLevel().Items) != 4 {
		t.Fatalf("step 4: expected 4 items after collapse, got %d", len(h.Model.currentLevel().Items))
	}

	// 5. View renders with ▶/▼ indicators.
	view := h.Model.View()
	if !strings.Contains(view, "▼") {
		t.Error("step 5: expected ▼ for expanded dev session")
	}
	if !strings.Contains(view, "▶") {
		t.Error("step 5: expected ▶ for collapsed nodes")
	}

	// 6. Filter "monitor" shows ops session + monitor window.
	h.SendText("monitor")
	items = h.Model.currentLevel().Items
	if len(items) < 2 {
		t.Fatalf("step 6: expected at least 2 filtered items, got %d", len(items))
	}
}
```

**Step 2: Run tests**

Run: `make test`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/ui/tree_test.go
git commit -m "test: add end-to-end tree level test"
```

---

### Task 15: Final integration and cleanup

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass.

**Step 2: Manual verification checklist**

Build and run against a live tmux session:
```bash
make build && ./tmux-popup-control --root-menu session:tree
```

Verify:
- [ ] Tree displays with ▶/▼ indicators and ├──/└── connectors
- [ ] Right arrow expands collapsed nodes
- [ ] Right arrow on expanded/leaf moves cursor down
- [ ] Left arrow collapses expanded nodes
- [ ] Left arrow on collapsed/leaf moves cursor up
- [ ] Up/Down navigate the flat list
- [ ] Enter on session switches to it
- [ ] Enter on window switches to it
- [ ] Enter on pane switches to it
- [ ] Escape returns to session menu
- [ ] Typing filters with ancestor preservation
- [ ] Preview panel shows pane capture
- [ ] `--menu-args expanded` starts fully expanded

**Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: address issues found during manual testing"
```
