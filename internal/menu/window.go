package menu

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var (
	switchClientFn  = tmux.SwitchClient
	selectWindowFn  = tmux.SelectWindow
	renameWindowFn  = tmux.RenameWindow
	linkWindowFn    = tmux.LinkWindow
	moveWindowFn    = tmux.MoveWindow
	swapWindowsFn   = tmux.SwapWindows
	unlinkWindowsFn = tmux.UnlinkWindows
)

func loadWindowMenu(Context) ([]Item, error) {
	items := []string{
		// vvv do NOT reorder these! vvv
		"kill",
		"rename",
		"swap",
		"move",
		"link",
		"switch",
		// ^^^ do NOT reorder these! ^^^
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
	return Item{ID: id, Label: "[current] " + label}, true
}

func loadWindowSwitchMenu(ctx Context) ([]Item, error) {
	return WindowSwitchItems(ctx), nil
}

func loadWindowRenameMenu(ctx Context) ([]Item, error) {
	return windowRenameItems(ctx), nil
}

func windowRenameItems(ctx Context) []Item {
	ordered := make([]WindowEntry, 0, len(ctx.Windows))
	var current *WindowEntry
	for _, entry := range ctx.Windows {
		if entry.Current {
			copy := entry
			current = &copy
			continue
		}
		ordered = append(ordered, entry)
	}
	sortWindowEntries(ordered)
	if current != nil {
		ordered = append([]WindowEntry{*current}, ordered...)
	}
	if len(ordered) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(ordered))
	ids := make([]string, 0, len(ordered))
	for _, entry := range ordered {
		label := strings.TrimSpace(entry.Label)
		label = strings.TrimPrefix(label, "[current] ")
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = label
		}
		windowID := strings.TrimSpace(entry.ID)
		if windowID == "" {
			windowID = fmt.Sprintf("#%d", entry.Index)
		}
		internalID := strings.TrimSpace(entry.InternalID)
		if internalID == "" {
			internalID = "-"
		}
		currentMark := ""
		if entry.Current {
			currentMark = "current"
		}
		rows = append(rows, []string{name, windowID, internalID, currentMark})
		ids = append(ids, entry.ID)
	}
	aligned := table.Format(rows, []table.Alignment{table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft})
	items := make([]Item, len(aligned))
	for i, label := range aligned {
		items[i] = Item{ID: ids[i], Label: label}
	}
	return items
}

func WindowSwitchItems(ctx Context) []Item {
	ordered := make([]WindowEntry, 0, len(ctx.Windows))
	var current *WindowEntry
	for _, entry := range ctx.Windows {
		if entry.Current && !ctx.WindowIncludeCurrent {
			continue
		}
		if entry.Current {
			copy := entry
			current = &copy
			continue
		}
		ordered = append(ordered, entry)
	}
	sortWindowEntries(ordered)
	if current != nil {
		ordered = append([]WindowEntry{*current}, ordered...)
	}
	if len(ordered) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(ordered))
	ids := make([]string, 0, len(ordered))
	for _, entry := range ordered {
		label := strings.TrimSpace(entry.Label)
		label = strings.TrimPrefix(label, "[current] ")
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = label
		}
		windowID := strings.TrimSpace(entry.ID)
		if windowID == "" {
			windowID = fmt.Sprintf("#%d", entry.Index)
		}
		internalID := strings.TrimSpace(entry.InternalID)
		if internalID == "" {
			internalID = "-"
		}
		currentMark := ""
		if entry.Current {
			currentMark = "current"
		}
		rows = append(rows, []string{name, windowID, internalID, currentMark})
		ids = append(ids, entry.ID)
	}
	aligned := table.Format(rows, []table.Alignment{table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft})
	items := make([]Item, len(aligned))
	for i, label := range aligned {
		items[i] = Item{ID: ids[i], Label: label}
	}
	return items
}

func sortWindowEntries(entries []WindowEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return windowEntryLess(entries[i], entries[j])
	})
}

func windowEntryLess(a, b WindowEntry) bool {
	as := buildWindowOrderKey(a)
	bs := buildWindowOrderKey(b)
	if as.session != bs.session {
		return as.session < bs.session
	}
	if as.hasIndex && bs.hasIndex && as.index != bs.index {
		return as.index < bs.index
	}
	if as.hasIndex != bs.hasIndex {
		return as.hasIndex
	}
	if a.Index != b.Index {
		return a.Index < b.Index
	}
	return a.ID < b.ID
}

type windowOrderKey struct {
	session  string
	index    int
	hasIndex bool
}

func buildWindowOrderKey(entry WindowEntry) windowOrderKey {
	session := strings.TrimSpace(entry.Session)
	raw := strings.TrimSpace(entry.ID)
	parts := strings.SplitN(raw, ":", 2)
	if session == "" && len(parts) > 0 {
		session = strings.TrimSpace(parts[0])
	}
	if session == "" {
		session = raw
	}
	if len(parts) == 2 {
		if idx, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			return windowOrderKey{session: session, index: idx, hasIndex: true}
		}
	}
	if entry.Index >= 0 {
		return windowOrderKey{session: session, index: entry.Index, hasIndex: true}
	}
	return windowOrderKey{session: session}
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

func loadWindowKillMenu(ctx Context) ([]Item, error) {
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
		events.Window.Switch(windowID)
		if err := switchClientFn(ctx.SocketPath, ctx.ClientID, session); err != nil {
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
	sorted := append([]string(nil), ids...)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	label := item.Label
	return func() tea.Msg {
		events.Window.Kill(sorted)
		if err := unlinkWindowsFn(ctx.SocketPath, sorted); err != nil {
			return ActionResult{Err: err}
		}
		if len(sorted) == 1 {
			return ActionResult{Info: fmt.Sprintf("Removed %s", label)}
		}
		return ActionResult{Info: fmt.Sprintf("Removed %d windows", len(sorted))}
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
	if strings.HasPrefix(initial, "[current]") {
		parts := strings.SplitN(initial, " ", 2)
		if len(parts) == 2 {
			initial = strings.TrimSpace(parts[1])
		}
	}
	if initial == "" {
		parts := strings.SplitN(item.Label, " ", 2)
		if len(parts) == 2 {
			initial = strings.TrimSpace(parts[1])
		}
	}
	return func() tea.Msg {
		events.Window.RenamePrompt(target)
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
			ID:         id,
			Label:      label,
			Name:       w.Name,
			Session:    w.Session,
			Index:      w.Index,
			InternalID: w.InternalID,
			Current:    w.Current,
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
		events.Window.Rename(target, trimmed)
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
		events.Window.Link(source, targetSession)
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
		events.Window.Move(source, targetSession)
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
		events.Window.SwapSelect(target)
		return WindowSwapPrompt{Context: ctx, First: item}
	}
}

func WindowSwapCommand(ctx Context, firstID, secondID, firstLabel, secondLabel string) tea.Cmd {
	return func() tea.Msg {
		events.Window.Swap(firstID, secondID)
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
		switch m.String() {
		case "ctrl+u":
			if f.input.Value() != "" {
				f.input.SetValue("")
				f.input.CursorStart()
			}
			return nil, false, false
		}
		switch m.Type {
		case tea.KeyEsc:
			events.Window.CancelRename(f.target, events.ReasonEscape)
			return nil, false, true
		case tea.KeyEnter:
			name := f.Value()
			if name == "" {
				events.Window.CancelRename(f.target, events.ReasonEmpty)
				return nil, false, true
			}
			events.Window.SubmitRename(f.target, name)
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
