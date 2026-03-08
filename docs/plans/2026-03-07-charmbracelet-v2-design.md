# Charmbracelet Ecosystem v2 Migration Design

## Goal

Upgrade all charmbracelet dependencies from their current versions (bubbletea v0.25, bubbles v0.16, lipgloss v0.13) to the coordinated v2 releases (Feb 2025). Replace `muesli/reflow` with `charmbracelet/x/ansi`.

## Version Targets

| Package | Current | Target |
|---|---|---|
| bubbletea | v0.25.0 | v2.0.1 (`charm.land/bubbletea/v2`) |
| bubbles | v0.16.0 | v2.0.0 (`charm.land/bubbles/v2`) |
| lipgloss | v0.13.1 | v2.0.0 (`charm.land/lipgloss/v2`) |
| muesli/reflow | v0.3.0 | **drop** (replace with `charmbracelet/x/ansi`) |
| muesli/termenv | v0.15.2 | **drop** (indirect, removed automatically) |

All three charmbracelet packages must be upgraded together since bubbles v2 requires bubbletea v2 and lipgloss v2.

## Approach

Big-bang migration on the main branch. All changes are mechanical — import path swaps, renamed types, new function signatures. The v1.0 releases were honorary tags with no API changes, so there is no value in an intermediate stop.

## Change Inventory

### 1. Import Paths (all files)

| Old | New |
|---|---|
| `github.com/charmbracelet/bubbletea` | `charm.land/bubbletea/v2` |
| `github.com/charmbracelet/bubbles/cursor` | `charm.land/bubbles/v2/cursor` |
| `github.com/charmbracelet/bubbles/textinput` | `charm.land/bubbles/v2/textinput` |
| `github.com/charmbracelet/lipgloss` | `charm.land/lipgloss/v2` |
| `github.com/charmbracelet/lipgloss/tree` | `charm.land/lipgloss/v2/tree` |
| `github.com/muesli/reflow/truncate` | `github.com/charmbracelet/x/ansi` |

### 2. Bubble Tea Model Interface (`internal/ui/model.go`)

- `Init()` returns `(*Model, tea.Cmd)` instead of `tea.Cmd`
- `View()` returns `tea.View` instead of `string`; alt-screen and mouse mode declared on the View struct
- `tea.NewProgram` no longer takes `WithAltScreen()` / `WithMouseCellMotion()` options

### 3. Key Input (`internal/ui/input.go`, `navigation.go`, menu forms)

- `tea.KeyMsg` renamed to `tea.KeyPressMsg`
- `msg.Type` field-based matching replaced with `msg.String()` matching
- `msg.Runes` ([]rune) replaced with `msg.Text` (string)
- `msg.Alt` replaced with `msg.Mod.Contains(tea.ModAlt)`
- Space: `msg.String()` returns `"space"` not `" "`
- `tea.KeyBackspace`, `tea.KeyRunes`, `tea.KeySpace`, etc. — replaced by string comparison

### 4. Mouse Handling (`internal/ui/view.go`)

- `tea.MouseMsg` split into `tea.MouseClickMsg`, `tea.MouseWheelMsg`, etc.
- `tea.MouseButtonWheelUp/Down` renamed to `tea.MouseUp/Down`
- Handler registration uses `tea.MouseWheelMsg{}`

### 5. Lipgloss (`internal/theme/theme.go`, `internal/ui/`)

- `Style.Copy()` removed — styles are value types, just assign
- `lipgloss.Color()` returns `color.Color` instead of string type (chaining unchanged)
- No renderer changes needed (this project doesn't use `DefaultRenderer()`)

### 6. Reflow → ansi (`internal/ui/view.go`)

- `truncate.StringWithTail(s, uint(n), tail)` → `ansi.Truncate(s, n, tail)` (int, not uint)

### 7. Cursor Component (`internal/ui/model.go`, `input.go`)

- `cursor.Blink` field → `cursor.IsBlinked()` getter
- `cursor.BlinkCmd()` → `cursor.Blink()`

### 8. Test Updates

- All `tea.KeyMsg{Type: tea.KeyFoo}` → `tea.KeyPressMsg` with new field names
- `tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}` → `tea.KeyPressMsg{Text: "x"}`

### 9. Build / Vendor

- Update `go.mod` with new module paths
- Drop `muesli/reflow`, `muesli/termenv` dependencies
- Add `charmbracelet/x/ansi` dependency
- `go mod tidy && go mod vendor`
- May need Go version bump if v2 requires newer minimum

## Files Affected

| Area | Files |
|---|---|
| App bootstrap | `internal/app/app.go` |
| UI model/handlers | `model.go`, `navigation.go`, `input.go`, `view.go`, `forms.go`, `backend.go`, `commands.go`, `preview.go`, `prompt.go`, `tree.go`, `harness.go` |
| Theme | `internal/theme/theme.go` |
| Menu forms | `internal/menu/session.go`, `window.go`, `pane.go`, `keybinding.go`, `menu.go`, `command.go` |
| Entry point | `main.go` |
| Tests | `input_test.go`, `tree_test.go`, `navigation_test.go`, `prompt_test.go`, `integration_test.go` |
| Build | `go.mod`, `go.sum`, `vendor/` |

## Risks

- **lipgloss/tree API changes**: The tree sub-package may have additional breaking changes beyond import paths. Will need to verify `tree.New()`, `tree.Root()`, `.Child()`, `.Enumerator()` still work.
- **cursor.Blink semantics**: Need to verify the exact replacement for the `Blink` field write pattern used in `finishUpdate`.
- **Vendor update**: Running `go mod vendor` with `GOPROXY=off` requires the new modules to already be in the module cache. May need a temporary `GOPROXY=on` fetch.
