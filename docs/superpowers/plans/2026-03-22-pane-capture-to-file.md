# Pane Capture-to-File Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `pane:capture` menu item that captures the current pane's full scrollback to a file, with a form UI supporting tmux format variables, strftime tokens, and a checkbox for escape sequences.

**Architecture:** New `PaneCaptureForm` (menu package) handles input, checkbox, and preview state. `CapturePaneToFile` and `ExpandFormat` (tmux package) wrap gotmuxcc operations. UI wiring follows the established prompt→form→command pattern. An `expandStrftime` helper in menu package handles `%F`, `%H`, etc. tokens that `DisplayMessage` cannot resolve.

**Tech Stack:** Go, Bubble Tea, gotmuxcc (vendored), lipgloss

**Spec:** `docs/superpowers/specs/2026-03-22-pane-capture-to-file-design.md`

---

## File Structure

| File | Responsibility | Status |
|---|---|---|
| `internal/menu/strftime.go` | `expandStrftime` + `expandTilde` pure helpers | Create |
| `internal/menu/strftime_test.go` | Tests for strftime/tilde expansion | Create |
| `internal/tmux/capture.go` | `CapturePaneToFile`, `ExpandFormat` (injectable vars) | Create |
| `internal/tmux/capture_test.go` | Tests for capture and format expansion | Create |
| `internal/menu/pane.go` | `PaneCapturePrompt`, `PaneCaptureForm`, `PaneCaptureAction`, `PaneCaptureCommand`; add `"capture"` to menu | Modify |
| `internal/menu/pane_test.go` | Tests for action, form key handling | Modify |
| `internal/menu/menu.go` | Register `"pane:capture"` in `ActionHandlers()` | Modify |
| `internal/logging/events/pane.go` | Add `Capture`, `CapturePrompt`, `CaptureCancel`, `CaptureSubmit` trace methods | Modify |
| `internal/ui/model.go` | `ModePaneCaptureForm` constant + `String()` case, `paneCaptureForm` field | Modify |
| `internal/ui/forms.go` | `handlePaneCaptureForm`, `startPaneCaptureForm`, `viewPaneCaptureForm` | Modify |
| `internal/ui/prompt.go` | `handlePaneCapturePromptMsg`, `handlePaneCapturePreviewMsg` | Modify |
| `internal/ui/view.go` | `ModePaneCaptureForm` case in `View()` switch | Modify |
| `internal/ui/pane_capture_test.go` | UI handler tests for prompt, preview, form escape | Create |

---

### Task 1: strftime and tilde expansion helpers

**Files:**
- Create: `internal/menu/strftime.go`
- Create: `internal/menu/strftime_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/menu/strftime_test.go`:

```go
package menu

import (
	"testing"
	"time"
)

func TestExpandStrftime(t *testing.T) {
	// Use a fixed time for deterministic tests.
	ts := time.Date(2026, 3, 22, 14, 30, 5, 0, time.Local)

	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"ISO date", "%F", "2026-03-22"},
		{"year", "%Y", "2026"},
		{"month", "%m", "03"},
		{"day", "%d", "22"},
		{"hour", "%H", "14"},
		{"minute", "%M", "30"},
		{"second", "%S", "05"},
		{"time", "%T", "14:30:05"},
		{"literal percent", "%%", "%"},
		{"combined", "log-%F-%H-%M-%S.txt", "log-2026-03-22-14-30-05.txt"},
		{"no tokens", "plain.txt", "plain.txt"},
		{"unknown token passthrough", "%Z-thing", "%Z-thing"},
		{"adjacent tokens", "%Y%m%d", "20260322"},
		{"trailing percent", "file%", "file%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandStrftimeAt(tt.input, ts)
			if got != tt.expect {
				t.Errorf("expandStrftimeAt(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestExpandTilde(t *testing.T) {
	home := "/Users/matt"
	tests := []struct {
		input  string
		expect string
	}{
		{"~/file.log", home + "/file.log"},
		{"~/", home + "/"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~other", "~other"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandTildeWith(tt.input, home)
			if got != tt.expect {
				t.Errorf("expandTildeWith(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run 'TestExpandStrftime|TestExpandTilde' -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write the implementation**

In `internal/menu/strftime.go`:

```go
package menu

import (
	"os"
	"strings"
	"time"
)

// strftime token → Go time layout mapping.
var strftimeTokens = map[byte]string{
	'F': "2006-01-02",
	'Y': "2006",
	'm': "01",
	'd': "02",
	'H': "15",
	'M': "04",
	'S': "05",
	'T': "15:04:05",
}

// expandStrftime replaces strftime tokens (%F, %H, etc.) with formatted time
// values. Unrecognised %x tokens are passed through unchanged. %% produces a
// literal %.
func expandStrftime(s string) string {
	return expandStrftimeAt(s, time.Now())
}

// expandStrftimeAt is the testable core — takes an explicit timestamp.
func expandStrftimeAt(s string, t time.Time) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			b.WriteByte(s[i])
			continue
		}
		if i+1 >= len(s) {
			b.WriteByte('%')
			continue
		}
		next := s[i+1]
		if next == '%' {
			b.WriteByte('%')
			i++
			continue
		}
		if layout, ok := strftimeTokens[next]; ok {
			b.WriteString(t.Format(layout))
			i++
			continue
		}
		// Unknown token — pass through unchanged.
		b.WriteByte('%')
	}
	return b.String()
}

// expandTilde replaces a leading ~/ with the user's home directory.
func expandTilde(s string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return s
	}
	return expandTildeWith(s, home)
}

// expandTildeWith is the testable core — takes an explicit home path.
func expandTildeWith(s, home string) string {
	if strings.HasPrefix(s, "~/") {
		return home + s[1:]
	}
	return s
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run 'TestExpandStrftime|TestExpandTilde' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/menu/strftime.go internal/menu/strftime_test.go
git commit -m "feat: add strftime and tilde expansion helpers for pane capture"
```

---

### Task 2: tmux capture and format expansion functions

**Files:**
- Create: `internal/tmux/capture.go`
- Create: `internal/tmux/capture_test.go`

- [ ] **Step 1: Write the failing tests**

In `internal/tmux/capture_test.go`:

```go
package tmux

import (
	"os"
	"path/filepath"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func TestCapturePaneToFile(t *testing.T) {
	captured := "line1\nline2\nline3"
	var gotTarget string
	var gotOpts *gotmux.CaptureOptions

	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			gotTarget = target
			gotOpts = op
			return captured, nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	dir := t.TempDir()
	outPath := filepath.Join(dir, "capture.log")

	err := CapturePaneToFile("sock", "%3", outPath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTarget != "%3" {
		t.Errorf("target = %q, want %%3", gotTarget)
	}
	if gotOpts.StartLine != "-" {
		t.Errorf("StartLine = %q, want \"-\"", gotOpts.StartLine)
	}
	if gotOpts.EscTxtNBgAttr {
		t.Error("EscTxtNBgAttr should be false when escSeqs=false")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	if string(data) != captured {
		t.Errorf("file content = %q, want %q", string(data), captured)
	}
}

func TestCapturePaneToFileWithEscSeqs(t *testing.T) {
	var gotOpts *gotmux.CaptureOptions
	fake := &fakeClient{
		capturePaneFn: func(_ string, op *gotmux.CaptureOptions) (string, error) {
			gotOpts = op
			return "content", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	dir := t.TempDir()
	outPath := filepath.Join(dir, "esc.log")
	err := CapturePaneToFile("sock", "%1", outPath, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotOpts.EscTxtNBgAttr {
		t.Error("EscTxtNBgAttr should be true when escSeqs=true")
	}
}

func TestExpandFormat(t *testing.T) {
	fake := &fakeClient{
		displayMessageFn: func(target, format string) (string, error) {
			return "expanded:" + format, nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	result, err := ExpandFormat("sock", "%3", "#{pane_id}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "expanded:#{pane_id}" {
		t.Errorf("result = %q, want %q", result, "expanded:#{pane_id}")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/tmux/... -run 'TestCapturePaneToFile|TestExpandFormat' -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write the implementation**

In `internal/tmux/capture.go`:

```go
package tmux

import (
	"fmt"
	"os"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// capturePaneToFileFn is the injectable var for tests.
var capturePaneToFileFn = CapturePaneToFile

// CapturePaneToFile captures the full scrollback of a pane and writes it to a
// file. escSeqs controls whether ANSI escape sequences are included (-e flag).
func CapturePaneToFile(socketPath, paneTarget, filePath string, escSeqs bool) error {
	target := strings.TrimSpace(paneTarget)
	if target == "" {
		return fmt.Errorf("pane target required")
	}
	client, err := newTmux(socketPath)
	if err != nil {
		return err
	}
	output, err := client.CapturePane(target, &gotmux.CaptureOptions{
		EscTxtNBgAttr: escSeqs,
		StartLine:     "-",
	})
	if err != nil {
		return fmt.Errorf("capture-pane %s: %w", target, err)
	}
	return os.WriteFile(filePath, []byte(output), 0644)
}

// expandFormatFn is the injectable var for tests.
var expandFormatFn = ExpandFormat

// ExpandFormat resolves a tmux format string against a target pane via
// display-message. Used for the live filename preview.
func ExpandFormat(socketPath, target, format string) (string, error) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", err
	}
	result, err := client.DisplayMessage(target, format)
	if err != nil {
		return "", fmt.Errorf("expand format %q: %w", format, err)
	}
	return strings.TrimSpace(result), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/tmux/... -run 'TestCapturePaneToFile|TestExpandFormat' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tmux/capture.go internal/tmux/capture_test.go
git commit -m "feat: add CapturePaneToFile and ExpandFormat tmux helpers"
```

---

### Task 3: trace events for pane capture

**Files:**
- Modify: `internal/logging/events/pane.go`

- [ ] **Step 1: Add trace methods**

Append to `internal/logging/events/pane.go`, after the existing `SubmitRename` method:

```go
func (PaneTracer) CapturePrompt(target string) {
	logging.Trace("pane.capture.prompt", map[string]interface{}{"target": target})
}

func (PaneTracer) Capture(target, filePath string, escSeqs bool) {
	logging.Trace("pane.capture", map[string]interface{}{"target": target, "file": filePath, "esc_seqs": escSeqs})
}

func (PaneTracer) CaptureCancel(reason paneReason) {
	logging.Trace("pane.capture.cancel", map[string]interface{}{"reason": string(reason)})
}

func (PaneTracer) CaptureSubmit(filePath string) {
	logging.Trace("pane.capture.submit", map[string]interface{}{"file": filePath})
}
```

- [ ] **Step 2: Verify it compiles**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go build ./internal/logging/events/...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/logging/events/pane.go
git commit -m "feat: add pane capture trace events"
```

---

### Task 4: PaneCapturePrompt, PaneCaptureForm, and action handler

**Files:**
- Modify: `internal/menu/pane.go` — add types and functions
- Modify: `internal/menu/menu.go` — register action
- Modify: `internal/menu/pane_test.go` — add tests

- [ ] **Step 1: Write the failing tests**

Append to `internal/menu/pane_test.go`. Also add `tea "charm.land/bubbletea/v2"` to the import block:

```go
func TestPaneCaptureActionReturnsPrompt(t *testing.T) {
	ctx := Context{SocketPath: "sock", CurrentPaneID: "%3"}
	msg := PaneCaptureAction(ctx, Item{ID: "pane:capture", Label: "capture"})()
	prompt, ok := msg.(PaneCapturePrompt)
	if !ok {
		t.Fatalf("expected PaneCapturePrompt, got %T", msg)
	}
	if prompt.Context.CurrentPaneID != "%3" {
		t.Errorf("pane ID = %q, want %%3", prompt.Context.CurrentPaneID)
	}
	if prompt.Template == "" {
		t.Error("template should not be empty")
	}
}

func TestPaneCaptureActionEmptyPaneID(t *testing.T) {
	ctx := Context{SocketPath: "sock", CurrentPaneID: ""}
	msg := PaneCaptureAction(ctx, Item{ID: "pane:capture"})()
	result, ok := msg.(ActionResult)
	if !ok {
		t.Fatalf("expected ActionResult, got %T", msg)
	}
	if result.Err == nil {
		t.Fatal("expected error for empty pane ID")
	}
}

func TestPaneCaptureFormToggleEscSeqs(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	if form.EscSeqs() {
		t.Fatal("escSeqs should default to false")
	}
	// Simulate tab press.
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if !form.EscSeqs() {
		t.Fatal("escSeqs should be true after tab")
	}
	form.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if form.EscSeqs() {
		t.Fatal("escSeqs should be false after second tab")
	}
}

func TestPaneCaptureFormEscCancels(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	_, done, cancel := form.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !cancel {
		t.Fatal("esc should cancel")
	}
	if done {
		t.Fatal("esc should not signal done")
	}
}

func TestPaneCaptureFormEnterSubmits(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	_, done, cancel := form.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !done {
		t.Fatal("enter should signal done")
	}
	if cancel {
		t.Fatal("enter should not cancel")
	}
	if form.Value() != "test.log" {
		t.Errorf("Value() = %q, want %q", form.Value(), "test.log")
	}
}

func TestPaneCaptureFormCtrlUClears(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	form.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if form.Value() != "" {
		t.Errorf("Value() = %q after ctrl+u, want empty", form.Value())
	}
}

func TestPaneCaptureFormSeqIncrementsOnInput(t *testing.T) {
	form := NewPaneCaptureForm(PaneCapturePrompt{
		Context:  Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	seq0 := form.Seq()
	form.Update(tea.KeyPressMsg{Code: 'a'})
	if form.Seq() <= seq0 {
		t.Error("seq should increment on input change")
	}
}

func TestLoadPaneMenuIncludesCapture(t *testing.T) {
	items, err := loadPaneMenu(Context{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range items {
		if item.ID == "capture" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("pane menu should include 'capture' item")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run 'TestPaneCapture|TestLoadPaneMenuIncludesCapture' -v`
Expected: FAIL — types/functions not defined

- [ ] **Step 3: Add "capture" to loadPaneMenu**

In `internal/menu/pane.go`, change the items list in `loadPaneMenu` — add `"capture"` after `"break"` and before `"switch"`:

```go
func loadPaneMenu(Context) ([]Item, error) {
	items := []string{
		// vvv do NOT reorder these! vvv
		"rename",
		"resize",
		"kill",
		"swap",
		"join",
		"break",
		"capture",
		"switch",
		// ^^^ do NOT reorder these! ^^^
	}
	return menuItemsFromIDs(items), nil
}
```

- [ ] **Step 4: Add PaneCapturePrompt, PaneCaptureForm, and PaneCaptureAction**

Append to `internal/menu/pane.go` (add `"os"` and `"time"` to the imports):

```go
const defaultCaptureTemplate = "~/tmux-#{pane_id}.%F-%H-%M-%S.log"

// PaneCapturePrompt asks the UI to show the capture-to-file form.
type PaneCapturePrompt struct {
	Context  Context
	Template string
}

// PaneCapturePreviewMsg carries an expanded path preview back to the UI.
type PaneCapturePreviewMsg struct {
	Path string
	Err  string
	Seq  int
}

// PaneCaptureAction returns a PaneCapturePrompt for the current pane.
func PaneCaptureAction(ctx Context, _ Item) tea.Cmd {
	target := strings.TrimSpace(ctx.CurrentPaneID)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("no current pane")} }
	}
	return func() tea.Msg {
		events.Pane.CapturePrompt(target)
		return PaneCapturePrompt{Context: ctx, Template: defaultCaptureTemplate}
	}
}

// PaneCaptureForm handles the capture-to-file form UI.
type PaneCaptureForm struct {
	input      textinput.Model
	ctx        Context
	escSeqs    bool
	preview    string
	previewErr string
	seq        int
}

// NewPaneCaptureForm creates a PaneCaptureForm from a PaneCapturePrompt.
func NewPaneCaptureForm(prompt PaneCapturePrompt) *PaneCaptureForm {
	ti := textinput.New()
	ti.Placeholder = "file path"
	ti.CharLimit = 256
	ti.SetWidth(60)
	if prompt.Template != "" {
		ti.SetValue(prompt.Template)
		ti.CursorEnd()
	}
	ti.Focus()
	return &PaneCaptureForm{
		input: ti,
		ctx:   prompt.Context,
	}
}

func (f *PaneCaptureForm) Context() Context    { return f.ctx }
func (f *PaneCaptureForm) Value() string       { return f.input.Value() }
func (f *PaneCaptureForm) InputView() string   { return f.input.View() }
func (f *PaneCaptureForm) EscSeqs() bool       { return f.escSeqs }
func (f *PaneCaptureForm) Preview() string     { return f.preview }
func (f *PaneCaptureForm) PreviewErr() string  { return f.previewErr }
func (f *PaneCaptureForm) Seq() int            { return f.seq }
func (f *PaneCaptureForm) ActionID() string    { return "pane:capture" }

func (f *PaneCaptureForm) Title() string {
	return "capture to file"
}

func (f *PaneCaptureForm) Help() string {
	return "tab: toggle escape sequences · enter: save · esc: cancel"
}

func (f *PaneCaptureForm) PendingLabel() string {
	v := f.Value()
	if v == "" {
		return f.ActionID()
	}
	return v
}

func (f *PaneCaptureForm) SetPreview(path, errMsg string) {
	f.preview = path
	f.previewErr = errMsg
}

func (f *PaneCaptureForm) SyncContext(ctx Context) {
	f.ctx = ctx
}

func (f *PaneCaptureForm) CheckboxView() string {
	if f.escSeqs {
		return "■ capture escape sequences"
	}
	return "□ capture escape sequences"
}

// Update processes a key message and returns (cmd, done, cancel).
func (f *PaneCaptureForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		switch m.String() {
		case "tab":
			f.escSeqs = !f.escSeqs
			return nil, false, false
		case "ctrl+u":
			if f.input.Value() != "" {
				f.input.SetValue("")
				f.input.CursorStart()
				f.seq++
			}
			return nil, false, false
		case "esc":
			events.Pane.CaptureCancel(PaneReasonEscape)
			return nil, false, true
		case "enter":
			v := f.Value()
			if v == "" {
				events.Pane.CaptureCancel(PaneReasonEmpty)
				return nil, false, true
			}
			events.Pane.CaptureSubmit(v)
			return nil, true, false
		}
	}
	prevVal := f.input.Value()
	updated, cmd := f.input.Update(msg)
	f.input = updated
	if f.input.Value() != prevVal {
		f.seq++
	}
	return cmd, false, false
}

// ExpandPreviewCmd returns a tea.Cmd that expands the current template and
// sends back a PaneCapturePreviewMsg.
func (f *PaneCaptureForm) ExpandPreviewCmd() tea.Cmd {
	template := f.Value()
	seq := f.seq
	ctx := f.ctx
	return func() tea.Msg {
		expanded := expandTilde(template)
		expanded = expandStrftime(expanded)
		result, err := tmux.ExpandFormat(ctx.SocketPath, ctx.CurrentPaneID, expanded)
		if err != nil {
			return PaneCapturePreviewMsg{Err: err.Error(), Seq: seq}
		}
		return PaneCapturePreviewMsg{Path: result, Seq: seq}
	}
}

// PaneCaptureCommand executes the capture: expands the template, captures the
// pane, and writes the file.
func PaneCaptureCommand(ctx Context, template string, escSeqs bool) tea.Cmd {
	return func() tea.Msg {
		target := strings.TrimSpace(ctx.CurrentPaneID)
		if target == "" {
			return ActionResult{Err: fmt.Errorf("no current pane")}
		}
		filePath := expandTilde(template)
		filePath = expandStrftime(filePath)
		resolved, err := tmux.ExpandFormat(ctx.SocketPath, target, filePath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("expand path: %w", err)}
		}

		dir := filepath.Dir(resolved)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return ActionResult{Err: fmt.Errorf("create directory %s: %w", dir, err)}
		}

		events.Pane.Capture(target, resolved, escSeqs)
		if err := tmux.CapturePaneToFile(ctx.SocketPath, target, resolved, escSeqs); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("captured to %s", resolved)}
	}
}
```

Add `"os"`, `"path/filepath"` to the imports in `pane.go`. Note: `"time"` is not needed in the import since `expandStrftime` (in `strftime.go`) uses `time.Now()` internally.

- [ ] **Step 5: Register action in menu.go**

In `internal/menu/menu.go`, add to the `ActionHandlers()` map after `"pane:rename"`:

```go
"pane:capture":      PaneCaptureAction,
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run 'TestPaneCapture|TestLoadPaneMenuIncludesCapture' -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/menu/pane.go internal/menu/pane_test.go internal/menu/menu.go
git commit -m "feat: add PaneCaptureForm, action, and menu registration"
```

---

### Task 5: UI wiring — mode, model fields, handler registration

**Files:**
- Modify: `internal/ui/model.go` — add mode constant, string case, form field, handler registration

- [ ] **Step 1: Add ModePaneCaptureForm constant**

In `internal/ui/model.go`, add `ModePaneCaptureForm` after `ModeSessionSaveForm` in the `const` block:

```go
const (
	ModeMenu Mode = iota
	ModePaneForm
	ModeWindowForm
	ModeSessionForm
	ModePluginConfirm
	ModePluginInstall
	ModeResurrect
	ModeSessionSaveForm
	ModePaneCaptureForm
)
```

- [ ] **Step 2: Add String() case**

In the `String()` method, add before `default`:

```go
case ModePaneCaptureForm:
	return "pane_capture_form"
```

- [ ] **Step 3: Add form field to Model struct**

In the `Model` struct, after the `saveForm` field:

```go
paneCaptureForm   *menu.PaneCaptureForm
```

- [ ] **Step 4: Add handler registrations**

In `registerHandlers()`, add:

```go
reflect.TypeOf(menu.PaneCapturePrompt{}):    m.handlePaneCapturePromptMsg,
reflect.TypeOf(menu.PaneCapturePreviewMsg{}): m.handlePaneCapturePreviewMsg,
```

- [ ] **Step 5: Add ModePaneCaptureForm to handleActiveForm**

In `handleActiveForm`, add a case before `default`:

```go
case ModePaneCaptureForm:
	return m.handlePaneCaptureForm(msg)
```

- [ ] **Step 6: Verify it compiles**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go build ./internal/ui/...`
Expected: will fail until forms.go and prompt.go changes are in place — that's expected. Proceed to task 6.

---

### Task 6: UI forms and prompt handlers

**Files:**
- Modify: `internal/ui/forms.go` — add form handler, start, and view functions
- Modify: `internal/ui/prompt.go` — add prompt and preview message handlers
- Modify: `internal/ui/view.go` — add View() case

- [ ] **Step 1: Add form handler to forms.go**

Append to `internal/ui/forms.go`:

```go
func (m *Model) handlePaneCaptureForm(msg tea.Msg) (bool, tea.Cmd) {
	if m.paneCaptureForm == nil {
		return false, nil
	}
	seqBefore := m.paneCaptureForm.Seq()
	cmd, done, cancel := m.paneCaptureForm.Update(msg)
	if cancel {
		m.paneCaptureForm = nil
		m.mode = ModeMenu
		return true, cmd
	}
	if done {
		ctx := m.paneCaptureForm.Context()
		template := m.paneCaptureForm.Value()
		escSeqs := m.paneCaptureForm.EscSeqs()
		actionID := m.paneCaptureForm.ActionID()
		pendingLabel := m.paneCaptureForm.PendingLabel()
		m.paneCaptureForm = nil
		m.mode = ModeMenu
		m.loading = true
		m.pendingID = actionID
		m.pendingLabel = pendingLabel
		return true, menu.PaneCaptureCommand(ctx, template, escSeqs)
	}
	// Only fire preview expansion when the input actually changed (seq advanced).
	if m.paneCaptureForm.Seq() != seqBefore {
		cmds := []tea.Cmd{}
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		cmds = append(cmds, m.paneCaptureForm.ExpandPreviewCmd())
		return true, tea.Batch(cmds...)
	}
	return true, cmd
}

func (m *Model) startPaneCaptureForm(prompt menu.PaneCapturePrompt) {
	m.paneCaptureForm = menu.NewPaneCaptureForm(prompt)
	m.mode = ModePaneCaptureForm
}

func (m *Model) viewPaneCaptureForm(header string) string {
	f := m.paneCaptureForm
	lines := []string{}
	if header != "" {
		title := f.Title()
		lines = append(lines, header+menuHeaderSeparator+title)
	} else {
		lines = append(lines, f.Title())
	}
	lines = append(lines, "", f.InputView(), "")

	// Checkbox line.
	checkboxLine := f.CheckboxView()
	if f.EscSeqs() && styles.CheckboxChecked != nil {
		checkboxLine = styles.CheckboxChecked.Render("■") + " capture escape sequences"
	} else if styles.Checkbox != nil {
		checkboxLine = styles.Checkbox.Render("□") + " capture escape sequences"
	}
	lines = append(lines, checkboxLine, "")

	// Preview line.
	if f.PreviewErr() != "" {
		lines = append(lines, styles.Error.Render(f.PreviewErr()))
	} else if f.Preview() != "" {
		preview := lipgloss.NewStyle().Faint(true).Render(f.Preview())
		lines = append(lines, preview)
	}
	lines = append(lines, "", f.Help())
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 2: Add prompt and preview handlers to prompt.go**

Append to `internal/ui/prompt.go`:

```go
func (m *Model) handlePaneCapturePromptMsg(msg tea.Msg) tea.Cmd {
	prompt, ok := msg.(menu.PaneCapturePrompt)
	if !ok {
		return nil
	}
	return m.withPrompt(func() promptResult {
		m.startPaneCaptureForm(prompt)
		return promptResult{Cmd: m.paneCaptureForm.ExpandPreviewCmd()}
	})
}

func (m *Model) handlePaneCapturePreviewMsg(msg tea.Msg) tea.Cmd {
	preview, ok := msg.(menu.PaneCapturePreviewMsg)
	if !ok {
		return nil
	}
	if m.paneCaptureForm == nil {
		return nil
	}
	if preview.Seq != m.paneCaptureForm.Seq() {
		return nil // stale response
	}
	m.paneCaptureForm.SetPreview(preview.Path, preview.Err)
	return nil
}
```

- [ ] **Step 3: Add View() case**

In `internal/ui/view.go`, in the `View()` method's `switch m.mode` block, add after the `ModePaneForm` case:

```go
case ModePaneCaptureForm:
	if m.paneCaptureForm != nil {
		content = m.viewPaneCaptureForm(header)
		return m.wrapView(content)
	}
```

- [ ] **Step 4: Verify it compiles**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go build ./...`
Expected: success

- [ ] **Step 5: Run all tests**

Run: `make test`
Expected: all existing tests pass, no regressions

- [ ] **Step 6: Commit**

```bash
git add internal/ui/model.go internal/ui/forms.go internal/ui/prompt.go internal/ui/view.go
git commit -m "feat: wire up pane capture form in UI layer"
```

---

### Task 7: UI handler tests

**Files:**
- Create: `internal/ui/pane_capture_test.go`

- [ ] **Step 1: Write the tests**

In `internal/ui/pane_capture_test.go`:

```go
package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestPaneCapturePromptSwitchesToForm(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	h := NewHarness(m)
	h.Send(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1", SocketPath: "sock"},
		Template: "test.log",
	})
	if m.mode != ModePaneCaptureForm {
		t.Fatalf("mode = %v, want ModePaneCaptureForm", m.mode)
	}
	if m.paneCaptureForm == nil {
		t.Fatal("paneCaptureForm should not be nil")
	}
}

func TestPaneCapturePreviewMsgUpdatesPreview(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.paneCaptureForm = menu.NewPaneCaptureForm(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	m.mode = ModePaneCaptureForm
	seq := m.paneCaptureForm.Seq()
	h := NewHarness(m)
	h.Send(menu.PaneCapturePreviewMsg{Path: "/resolved/path.log", Seq: seq})
	if m.paneCaptureForm.Preview() != "/resolved/path.log" {
		t.Errorf("preview = %q, want %q", m.paneCaptureForm.Preview(), "/resolved/path.log")
	}
}

func TestPaneCapturePreviewMsgStaleDiscarded(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.paneCaptureForm = menu.NewPaneCaptureForm(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	m.mode = ModePaneCaptureForm
	m.paneCaptureForm.SetPreview("original", "")
	// Send a preview msg with a stale seq (seq - 1).
	h := NewHarness(m)
	h.Send(menu.PaneCapturePreviewMsg{Path: "/stale/path.log", Seq: m.paneCaptureForm.Seq() - 1})
	if m.paneCaptureForm.Preview() != "original" {
		t.Errorf("stale preview was applied: %q", m.paneCaptureForm.Preview())
	}
}

func TestPaneCaptureFormEscReturnsToMenu(t *testing.T) {
	m := NewModel("", 80, 24, false, false, nil, "", "", "", "")
	m.paneCaptureForm = menu.NewPaneCaptureForm(menu.PaneCapturePrompt{
		Context:  menu.Context{CurrentPaneID: "%1"},
		Template: "test.log",
	})
	m.mode = ModePaneCaptureForm
	h := NewHarness(m)
	h.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.mode != ModeMenu {
		t.Fatalf("mode = %v, want ModeMenu after esc", m.mode)
	}
	if m.paneCaptureForm != nil {
		t.Fatal("paneCaptureForm should be nil after esc")
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/ui/... -run 'TestPaneCapture' -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/ui/pane_capture_test.go
git commit -m "test: add UI handler tests for pane capture form"
```

---

### Task 8: Full test suite and final verification

**Files:**
- All test files from prior tasks

- [ ] **Step 1: Run the full test suite**

Run: `make test`
Expected: PASS — all tests including new ones

- [ ] **Step 2: Run build to verify clean compilation**

Run: `make build`
Expected: `./tmux-popup-control` binary produced

- [ ] **Step 3: Verify menu item appears**

Manually verify (if tmux is available): launch with `TMUX_POPUP_CONTROL_ROOT_MENU=pane` and confirm "capture" appears in the pane submenu between "break" and "switch".

- [ ] **Step 4: Commit any remaining fixups**

Only if needed — no commit if everything passed cleanly.

---

## Implementation Notes

- **`PaneReasonEscape` / `PaneReasonEmpty`**: These constants already exist in the events package (`internal/logging/events/pane.go`) and are reused for `CaptureCancel`. No new constants needed.
- **The `withPaneStub` test helper** in `pane_test.go` is generic and works for any `*T` — reuse it for stubbing `capturePaneToFileFn` and `expandFormatFn` in future tests if needed.
- **Preview fires on every keystroke**: The sequence counter pattern ensures only the latest result is applied. No timer-based debounce is needed — `DisplayMessage` is fast via control-mode.
- **`os.MkdirAll`** in `PaneCaptureCommand` ensures intermediate directories are created if the user specifies a path like `~/captures/tmux/...`.
