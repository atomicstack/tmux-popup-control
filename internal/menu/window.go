package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var (
	switchClientFn = tmux.SwitchClient
	selectWindowFn = tmux.SelectWindow
	renameWindowFn = tmux.RenameWindow
	linkWindowFn   = tmux.LinkWindow
	moveWindowFn   = tmux.MoveWindow
	swapWindowsFn  = tmux.SwapWindows
	killWindowsFn  = tmux.KillWindows
)

func loadWindowMenu(Context) ([]Item, error) {
	items := []string{
		"switch",
		"link",
		"move",
		"swap",
		"rename",
		"kill",
	}
	return menuItemsFromIDs(items), nil
}

func windowItemFromEntry(entry WindowEntry) Item {
	return Item{ID: entry.ID, Label: entry.Label}
}

func currentWindowItem(ctx Context) (Item, bool) {
	id := strings.TrimSpace(ctx.CurrentWindowID)
	if id == "" {
		return Item{}, false
	}
	label := strings.TrimSpace(ctx.CurrentWindowLabel)
	if label == "" {
		label = id
	}
	return Item{ID: id, Label: fmt.Sprintf("[current] %s", label)}, true
}

func loadWindowSwitchMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Windows))
	for _, entry := range ctx.Windows {
		if entry.Current && !ctx.WindowIncludeCurrent {
			continue
		}
		items = append(items, windowItemFromEntry(entry))
	}
	return items, nil
}

func loadWindowKillMenu(ctx Context) ([]Item, error) {
	items := WindowEntriesToItems(ctx.Windows)
	if current, ok := currentWindowItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items, nil
}

func loadWindowRenameMenu(ctx Context) ([]Item, error) {
	items := WindowEntriesToItems(ctx.Windows)
	if current, ok := currentWindowItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items, nil
}

func loadWindowLinkMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Windows))
	for _, entry := range ctx.Windows {
		if entry.Session == ctx.CurrentWindowSession {
			continue
		}
		items = append(items, windowItemFromEntry(entry))
	}
	return items, nil
}

func loadWindowMoveMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Windows))
	for _, entry := range ctx.Windows {
		if entry.Session == ctx.CurrentWindowSession {
			continue
		}
		items = append(items, windowItemFromEntry(entry))
	}
	return items, nil
}

func loadWindowSwapMenu(ctx Context) ([]Item, error) {
	items := WindowEntriesToItems(ctx.Windows)
	if current, ok := currentWindowItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items, nil
}

func WindowSwitchAction(ctx Context, item Item) tea.Cmd {
	windowID := item.ID
	parts := strings.SplitN(windowID, ":", 2)
	if len(parts) != 2 {
		err := fmt.Errorf("invalid window id: %s", windowID)
		return func() tea.Msg { return ActionResult{Err: err} }
	}
	session := parts[0]
	label := item.Label
	return func() tea.Msg {
		logging.Trace("window.switch", map[string]interface{}{"target": windowID})
		if err := switchClientFn(ctx.SocketPath, session); err != nil {
			return ActionResult{Err: err}
		}
		if err := selectWindowFn(ctx.SocketPath, windowID); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Switched to %s", label)}
	}
}

func WindowKillAction(ctx Context, item Item) tea.Cmd {
	ids := splitWindowIDs(item.ID)
	label := item.Label
	return func() tea.Msg {
		logging.Trace("window.kill", map[string]interface{}{"targets": ids})
		if err := killWindowsFn(ctx.SocketPath, ids); err != nil {
			return ActionResult{Err: err}
		}
		if len(ids) == 1 {
			return ActionResult{Info: fmt.Sprintf("Killed %s", label)}
		}
		return ActionResult{Info: fmt.Sprintf("Killed %d windows", len(ids))}
	}
}

func WindowRenameAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid window target")} }
	}
	initial := strings.TrimSpace(item.Label)
	for _, entry := range ctx.Windows {
		if entry.ID == target {
			if entry.Name != "" {
				initial = entry.Name
			}
			break
		}
	}
	if initial == "" {
		parts := strings.SplitN(item.Label, " ", 2)
		if len(parts) == 2 {
			initial = strings.TrimSpace(parts[1])
		}
	}
	return func() tea.Msg {
		logging.Trace("window.rename.prompt", map[string]interface{}{"target": target})
		return WindowPrompt{Context: ctx, Target: target, Initial: initial}
	}
}

func WindowEntriesFromTmux(windows []tmux.Window) []WindowEntry {
	entries := make([]WindowEntry, 0, len(windows))
	for _, w := range windows {
		id := w.ID
		label := w.Label
		if label == "" {
			label = fmt.Sprintf("%s:%d %s", w.Session, w.Index, w.Name)
		}
		entries = append(entries, WindowEntry{
			ID:      id,
			Label:   label,
			Name:    w.Name,
			Session: w.Session,
			Index:   w.Index,
			Current: w.Current,
		})
	}
	return entries
}

func WindowRenameCommand(ctx Context, target, name string) tea.Cmd {
	return func() tea.Msg {
		if target == "" {
			return ActionResult{Err: fmt.Errorf("window target required")}
		}
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return ActionResult{Err: fmt.Errorf("window name required")}
		}
		logging.Trace("window.rename", map[string]interface{}{"target": target, "name": trimmed})
		if err := renameWindowFn(ctx.SocketPath, target, trimmed); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Renamed %s to %s", target, trimmed)}
	}
}

func WindowLinkAction(ctx Context, item Item) tea.Cmd {
	source := strings.TrimSpace(item.ID)
	targetSession := strings.TrimSpace(ctx.CurrentWindowSession)
	if source == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid window target")} }
	}
	if targetSession == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("no active session detected")} }
	}
	return func() tea.Msg {
		logging.Trace("window.link", map[string]interface{}{"source": source, "session": targetSession})
		if err := linkWindowFn(ctx.SocketPath, source, targetSession); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Linked %s to %s", item.Label, targetSession)}
	}
}

func WindowMoveAction(ctx Context, item Item) tea.Cmd {
	source := strings.TrimSpace(item.ID)
	targetSession := strings.TrimSpace(ctx.CurrentWindowSession)
	if source == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid window target")} }
	}
	if targetSession == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("no active session detected")} }
	}
	return func() tea.Msg {
		logging.Trace("window.move", map[string]interface{}{"source": source, "session": targetSession})
		if err := moveWindowFn(ctx.SocketPath, source, targetSession); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Moved %s to %s", item.Label, targetSession)}
	}
}

func WindowSwapAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid window target")} }
	}
	return func() tea.Msg {
		logging.Trace("window.swap.select", map[string]interface{}{"first": target})
		return WindowSwapPrompt{Context: ctx, First: item}
	}
}

func WindowSwapCommand(ctx Context, firstID, secondID, firstLabel, secondLabel string) tea.Cmd {
	return func() tea.Msg {
		logging.Trace("window.swap", map[string]interface{}{"first": firstID, "second": secondID})
		if err := swapWindowsFn(ctx.SocketPath, firstID, secondID); err != nil {
			return ActionResult{Err: err}
		}
		if firstLabel == "" {
			firstLabel = firstID
		}
		if secondLabel == "" {
			secondLabel = secondID
		}
		return ActionResult{Info: fmt.Sprintf("Swapped %s ↔ %s", firstLabel, secondLabel)}
	}
}

type WindowPrompt struct {
	Context Context
	Target  string
	Initial string
}

type WindowSwapPrompt struct {
	Context Context
	First   Item
}

type WindowRenameForm struct {
	input  textinput.Model
	ctx    Context
	target string
	help   string
	title  string
}

func NewWindowRenameForm(prompt WindowPrompt) *WindowRenameForm {
	ti := textinput.New()
	ti.Placeholder = "window-name"
	ti.CharLimit = 64
	if prompt.Initial != "" {
		ti.SetValue(prompt.Initial)
		ti.CursorEnd()
	}
	ti.Focus()
	title := fmt.Sprintf("Rename %s", prompt.Target)
	if prompt.Initial != "" {
		title = fmt.Sprintf("Rename %s", prompt.Initial)
	}
	form := &WindowRenameForm{
		input:  ti,
		ctx:    prompt.Context,
		target: prompt.Target,
		help:   "Press Enter to rename. Esc to cancel.",
		title:  title,
	}
	return form
}

func (f *WindowRenameForm) Context() Context  { return f.ctx }
func (f *WindowRenameForm) Target() string    { return f.target }
func (f *WindowRenameForm) Title() string     { return f.title }
func (f *WindowRenameForm) Help() string      { return f.help }
func (f *WindowRenameForm) Value() string     { return strings.TrimSpace(f.input.Value()) }
func (f *WindowRenameForm) InputView() string { return f.input.View() }

func (f *WindowRenameForm) ActionID() string { return "window:rename" }

func (f *WindowRenameForm) PendingLabel() string {
	name := f.Value()
	if name == "" {
		return f.ActionID()
	}
	return fmt.Sprintf("%s → %s", f.target, name)
}

func (f *WindowRenameForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.Type {
		case tea.KeyEsc:
			logging.Trace("window.rename.cancel", map[string]interface{}{"target": f.target, "reason": "escape"})
			return nil, false, true
		case tea.KeyEnter:
			name := f.Value()
			if name == "" {
				logging.Trace("window.rename.cancel", map[string]interface{}{"target": f.target, "reason": "empty"})
				return nil, false, true
			}
			logging.Trace("window.rename.submit", map[string]interface{}{"target": f.target, "name": name})
			return WindowRenameCommand(f.ctx, f.target, name), true, false
		}
	}
	updated, cmd := f.input.Update(msg)
	f.input = updated
	return cmd, false, false
}

func splitWindowIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ','
	})
	ids := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		ids = append(ids, raw)
	}
	return ids
}
