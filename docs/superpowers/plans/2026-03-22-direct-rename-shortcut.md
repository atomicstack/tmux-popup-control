# Direct Rename Shortcut Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Skip the session/window picker and open the rename form directly when `--root-menu session:rename` or `--root-menu window:rename` is invoked with `--menu-args` containing the target name.

**Architecture:** Add a `deferredRename` field to the UI model. When `applyRootMenuOverride` detects a rename node with non-empty `menuArgs`, it stores the node as a deferred rename instead of loading the picker. When backend data arrives (`SessionsUpdated` or `WindowsUpdated`), the deferred rename fires, opening the form directly via `withPrompt` + `startSessionForm`/`startWindowForm`.

**Tech Stack:** Go, Bubble Tea, bash (tmux keybindings)

**Spec:** `docs/superpowers/specs/2026-03-22-direct-rename-shortcut-design.md`

---

### Task 1: Add `deferredRename` field and deferred rename detection in `applyRootMenuOverride`

**Files:**
- Modify: `internal/ui/model.go:130` (add field)
- Modify: `internal/ui/navigation.go:492-507` (add new branch)
- Test: `internal/ui/navigation_test.go`

- [ ] **Step 1: Write the failing test for session:rename deferred**

Add to `internal/ui/navigation_test.go`:

```go
func TestRootMenuSessionRenameDeferredWithMenuArgs(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "session:rename", "mysession", "", "")
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set when session:rename has menuArgs")
	}
	if m.deferredRename.ID != "session:rename" {
		t.Fatalf("deferredRename.ID = %q, want session:rename", m.deferredRename.ID)
	}
	if !m.loading {
		t.Fatal("expected loading=true while deferred rename is pending")
	}
	if m.rootMenuID != "session:rename" {
		t.Fatalf("rootMenuID = %q, want session:rename", m.rootMenuID)
	}
	if m.rootTitle != "session" {
		t.Fatalf("rootTitle = %q, want session", m.rootTitle)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestRootMenuSessionRenameDeferredWithMenuArgs -v`
Expected: FAIL — `deferredRename` field does not exist.

- [ ] **Step 3: Add `deferredRename` field to `Model`**

In `internal/ui/model.go`, add after line 130 (`deferredAction *menu.Node`):

```go
	deferredRename *menu.Node
```

- [ ] **Step 4: Add deferred rename branch in `applyRootMenuOverride`**

In `internal/ui/navigation.go`, insert a new block after line 491 (the closing brace of the "not found" guard) and **before** the `node.Loader != nil` branch at line 509 that would otherwise load the picker list. Both `session:rename` and `window:rename` have loaders, so without this early return they would fall through to the standard loader path:

```go
	// If the node is a rename action (session:rename or window:rename) and
	// menuArgs provides a target, defer the rename form until backend data
	// is available. The form needs session/window entries for duplicate
	// name validation and for resolving the initial value.
	if m.menuArgs != "" && (id == "session:rename" || id == "window:rename") {
		m.loading = true
		m.pendingID = node.ID
		m.deferredRename = node
		m.rootMenuID = node.ID
		title := node.ID
		if idx := strings.LastIndex(title, ":"); idx >= 0 {
			title = title[:idx]
		}
		m.rootTitle = headerSegmentCleaner.Replace(title)
		return
	}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestRootMenuSessionRenameDeferredWithMenuArgs -v`
Expected: PASS

- [ ] **Step 6: Write the failing test for window:rename deferred**

Add to `internal/ui/navigation_test.go`:

```go
func TestRootMenuWindowRenameDeferredWithMenuArgs(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "window:rename", "main:0", "", "")
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set when window:rename has menuArgs")
	}
	if m.deferredRename.ID != "window:rename" {
		t.Fatalf("deferredRename.ID = %q, want window:rename", m.deferredRename.ID)
	}
	if !m.loading {
		t.Fatal("expected loading=true while deferred rename is pending")
	}
}
```

- [ ] **Step 7: Run test to verify it passes (should already pass)**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestRootMenuWindowRenameDeferredWithMenuArgs -v`
Expected: PASS (the branch handles both IDs)

- [ ] **Step 8: Write test for empty menuArgs fallback**

Add to `internal/ui/navigation_test.go`:

```go
func TestRootMenuSessionRenameWithoutMenuArgsFallsThrough(t *testing.T) {
	// When menuArgs is empty, session:rename should load the picker list
	// via the standard loader path, not defer.
	m := NewModel("", 80, 24, false, false, nil, "session:rename", "", "", "")
	if m.deferredRename != nil {
		t.Fatal("expected deferredRename to be nil when menuArgs is empty")
	}
}
```

- [ ] **Step 9: Run test to verify it passes**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestRootMenuSessionRenameWithoutMenuArgsFallsThrough -v`
Expected: PASS

- [ ] **Step 10: Commit**

```bash
git add internal/ui/model.go internal/ui/navigation.go internal/ui/navigation_test.go
git commit -m "feat: defer rename form when root-menu rename has menu-args"
```

---

### Task 2: Fire deferred rename form from `applyBackendEvent`

**Files:**
- Modify: `internal/ui/backend.go:66-92` (session path), `94-131` (window path)
- Test: `internal/ui/navigation_test.go`

- [ ] **Step 1: Write the failing test for session rename form opening on SessionsUpdated**

Add to `internal/ui/navigation_test.go`:

```go
func TestDeferredSessionRenameFiresOnSessionsUpdated(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "session:rename", "main", "", "")
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set")
	}

	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{
			{Name: "main", Label: "main: 2 windows"},
			{Name: "other", Label: "other: 1 window"},
		},
		Current: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})

	if h.Model().deferredRename != nil {
		t.Fatal("expected deferredRename cleared after SessionsUpdated")
	}
	if h.Model().mode != ModeSessionForm {
		t.Fatalf("mode = %v, want ModeSessionForm", h.Model().mode)
	}
	if h.Model().sessionForm == nil {
		t.Fatal("expected sessionForm to be set")
	}
	if h.Model().sessionForm.Target() != "main" {
		t.Fatalf("sessionForm target = %q, want main", h.Model().sessionForm.Target())
	}
	if h.Model().loading {
		t.Fatal("expected loading=false after form opens")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestDeferredSessionRenameFiresOnSessionsUpdated -v`
Expected: FAIL — deferred rename is not handled in `applyBackendEvent`.

- [ ] **Step 3: Add deferred session rename handling in `applyBackendEvent`**

In `internal/ui/backend.go`, inside the `if res.SessionsUpdated {` block (after the existing `m.sessionForm.SetSessions` block, around line 88), add:

```go
		if m.deferredRename != nil && m.deferredRename.ID == "session:rename" {
			m.deferredRename = nil
			target := strings.TrimSpace(m.menuArgs)
			m.withPrompt(func() promptResult {
				m.startSessionForm(menu.SessionPrompt{
					Context: ctx,
					Action:  "session:rename",
					Target:  target,
					Initial: target,
				})
				return promptResult{}
			})
		}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestDeferredSessionRenameFiresOnSessionsUpdated -v`
Expected: PASS

- [ ] **Step 5: Write the failing test for window rename form opening on WindowsUpdated**

Add to `internal/ui/navigation_test.go`:

```go
func TestDeferredWindowRenameFiresOnWindowsUpdated(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "window:rename", "main:0", "", "")
	if m.deferredRename == nil {
		t.Fatal("expected deferredRename to be set")
	}

	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Send sessions first — should NOT trigger the deferred window rename.
	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}},
		Current:  "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})
	if h.Model().deferredRename == nil {
		t.Fatal("deferredRename should still be pending after session event")
	}

	// Now send windows — should trigger the deferred window rename.
	winSnap := tmux.WindowSnapshot{
		Windows: []tmux.Window{
			{ID: "main:0", Session: "main", Name: "vim", Label: "0: vim", Current: true},
			{ID: "main:1", Session: "main", Name: "zsh", Label: "1: zsh"},
		},
		CurrentID:      "main:0",
		CurrentSession: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: winSnap}})

	if h.Model().deferredRename != nil {
		t.Fatal("expected deferredRename cleared after WindowsUpdated")
	}
	if h.Model().mode != ModeWindowForm {
		t.Fatalf("mode = %v, want ModeWindowForm", h.Model().mode)
	}
	if h.Model().windowForm == nil {
		t.Fatal("expected windowForm to be set")
	}
	if h.Model().windowForm.Target() != "main:0" {
		t.Fatalf("windowForm target = %q, want main:0", h.Model().windowForm.Target())
	}
	if h.Model().loading {
		t.Fatal("expected loading=false after form opens")
	}
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestDeferredWindowRenameFiresOnWindowsUpdated -v`
Expected: FAIL — deferred window rename not handled.

- [ ] **Step 7: Add deferred window rename handling in `applyBackendEvent`**

In `internal/ui/backend.go`, inside the `if res.WindowsUpdated {` block (after the existing window:kill handling, around line 131), add:

```go
		if m.deferredRename != nil && m.deferredRename.ID == "window:rename" {
			m.deferredRename = nil
			target := strings.TrimSpace(m.menuArgs)
			initial := target
			for _, entry := range ctx.Windows {
				if entry.ID == target {
					if entry.Name != "" {
						initial = entry.Name
					}
					break
				}
			}
			m.withPrompt(func() promptResult {
				m.startWindowForm(menu.WindowPrompt{
					Context: ctx,
					Target:  target,
					Initial: initial,
				})
				return promptResult{}
			})
		}
```

- [ ] **Step 8: Run test to verify it passes**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestDeferredWindowRenameFiresOnWindowsUpdated -v`
Expected: PASS

- [ ] **Step 9: Write test for window name resolution from entries**

Add to `internal/ui/navigation_test.go`:

```go
func TestDeferredWindowRenameResolvesWindowName(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "window:rename", "main:0", "", "")
	h := NewHarness(m)
	h.Send(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Provide sessions first.
	sessSnap := tmux.SessionSnapshot{
		Sessions: []tmux.Session{{Name: "main", Label: "main: 1 window"}},
		Current:  "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindSessions, Data: sessSnap}})

	// Provide windows with a name for main:0.
	winSnap := tmux.WindowSnapshot{
		Windows: []tmux.Window{
			{ID: "main:0", Session: "main", Name: "editor", Label: "0: editor", Current: true},
		},
		CurrentID:      "main:0",
		CurrentSession: "main",
	}
	h.Send(backendEventMsg{event: backend.Event{Kind: backend.KindWindows, Data: winSnap}})

	if h.Model().windowForm == nil {
		t.Fatal("expected windowForm to be set")
	}
	// The form's initial value should be the resolved window name, not the ID.
	got := h.Model().windowForm.Value()
	if got != "editor" {
		t.Fatalf("windowForm initial value = %q, want %q", got, "editor")
	}
}
```

- [ ] **Step 10: Run test to verify it passes**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run TestDeferredWindowRenameResolvesWindowName -v`
Expected: PASS

- [ ] **Step 11: Run full test suite**

Run: `make test`
Expected: all tests pass.

- [ ] **Step 12: Commit**

```bash
git add internal/ui/backend.go internal/ui/navigation_test.go
git commit -m "feat: fire deferred rename form when backend data arrives"
```

---

### Task 3: Add tmux keybindings in `main.tmux`

**Files:**
- Modify: `main.tmux:63` (append new bindings)

- [ ] **Step 1: Add session rename keybinding**

Append to `main.tmux`:

```bash

[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RENAME="$(opt key-session-rename)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_SESSION_RENAME='$'
tmux bind-key -T prefix -N "Renames session via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_SESSION_RENAME" run-shell -b "$LAUNCH_SCRIPT --root-menu session:rename --menu-args '#{session_name}'"
```

- [ ] **Step 2: Add window rename keybinding**

Append to `main.tmux`:

```bash

[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME="$(opt key-window-rename)"
[[ -z "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" ]] && TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME=','
tmux bind-key -T prefix -N "Renames window via $BINARY_NAME" "$TMUX_POPUP_CONTROL_KEY_WINDOW_RENAME" run-shell -b "$LAUNCH_SCRIPT --root-menu window:rename --menu-args '#{session_name}:#{window_index}'"
```

- [ ] **Step 3: Run `make build` to ensure the binary still builds**

Run: `make build`
Expected: builds successfully.

- [ ] **Step 4: Commit**

```bash
git add main.tmux
git commit -m "feat: add session rename and window rename keybindings"
```

---

### Task 4: Run full test suite and build

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: all tests pass.

- [ ] **Step 2: Run build**

Run: `make build`
Expected: builds successfully.
