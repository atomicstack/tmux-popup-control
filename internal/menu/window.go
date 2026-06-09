package menu

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var (
	switchClientFn  = tmux.SwitchClient
	selectWindowFn  = tmux.SelectWindow
	selectLayoutFn  = tmux.SelectLayout
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
		"layout",
		"swap",
		"push-to-session",
		"pull-from-session",
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
	return Item{ID: id, Label: currentLabelPrefix + label}, true
}

func loadWindowSwitchMenu(ctx Context) ([]Item, error) {
	return WindowSwitchItems(ctx), nil
}

func loadWindowRenameMenu(ctx Context) ([]Item, error) {
	return windowRenameItems(ctx), nil
}

func windowRenameItems(ctx Context) []Item {
	return windowTableItems(ctx, true)
}

func WindowSwitchItems(ctx Context) []Item {
	return windowTableItems(ctx, ctx.WindowIncludeCurrent)
}

// windowTableItems builds the formatted window table shared by the rename and
// switch menus. The current window is placed first; includeCurrent controls
// whether it is shown at all (rename always includes it).
func windowTableItems(ctx Context, includeCurrent bool) []Item {
	ordered := make([]WindowEntry, 0, len(ctx.Windows))
	var current *WindowEntry
	for _, entry := range ctx.Windows {
		if entry.Current && !includeCurrent {
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
		label := stripCurrentPrefix(entry.Label)
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
	return tableItems(rows, ids, []table.Alignment{table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft})
}

func sortWindowEntries(entries []WindowEntry) {
	slices.SortFunc(entries, func(a, b WindowEntry) int {
		switch {
		case windowEntryLess(a, b):
			return -1
		case windowEntryLess(b, a):
			return 1
		default:
			return 0
		}
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
	prefix, indexStr, hasIndex := strings.Cut(raw, ":")
	if session == "" {
		session = strings.TrimSpace(prefix)
	}
	if session == "" {
		session = raw
	}
	if hasIndex {
		if idx, err := strconv.Atoi(strings.TrimSpace(indexStr)); err == nil {
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

func loadWindowPullFromSessionMenu(ctx Context) ([]Item, error) {
	sessions := make([]SessionEntry, 0, len(ctx.Sessions))
	for _, s := range ctx.Sessions {
		if s.Name != ctx.CurrentWindowSession {
			sessions = append(sessions, s)
		}
	}
	windows := make([]WindowEntry, 0, len(ctx.Windows))
	for _, w := range ctx.Windows {
		if w.Session != ctx.CurrentWindowSession {
			windows = append(windows, w)
		}
	}
	ts := NewTreeState(false)
	return ts.BuildTreeItems(TreeItemsInput{
		Sessions: sessions,
		Windows:  windows,
	}), nil
}

func loadWindowPushToSessionMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Sessions))
	for _, entry := range ctx.Sessions {
		if entry.Name == ctx.CurrentWindowSession {
			continue
		}
		items = append(items, Item{ID: entry.Name, Label: entry.Label})
	}
	return items, nil
}

func WindowPushToSessionAction(ctx Context, item Item) tea.Cmd {
	source := strings.TrimSpace(ctx.CurrentWindowID)
	targetSession := strings.TrimSpace(item.ID)
	if source == "" {
		return failCmd("no current window detected")
	}
	if targetSession == "" {
		return failCmd("invalid target session")
	}
	return runAction(
		func() { events.Window.PushToSession(source, targetSession) },
		func() error { return moveWindowFn(ctx.SocketPath, source, targetSession) },
		fmt.Sprintf("Moved window to %s", targetSession),
	)
}

func loadWindowSwapMenu(ctx Context) ([]Item, error) {
	current, ok := currentWindowItem(ctx)
	return withCurrentFirst(WindowEntriesToItems(ctx.Windows), current, ok), nil
}

func loadWindowKillMenu(ctx Context) ([]Item, error) {
	current, ok := currentWindowItem(ctx)
	return withCurrentFirst(WindowEntriesToItems(ctx.Windows), current, ok), nil
}

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

func WindowLayoutAction(ctx Context, item Item) tea.Cmd {
	layout := strings.TrimSpace(item.ID)
	if layout == "" {
		return failCmd("invalid layout")
	}
	return runAction(
		func() { events.Window.Layout(layout) },
		func() error { return selectLayoutFn(ctx.SocketPath, layout) },
		fmt.Sprintf("Applied layout %s", layout),
	)
}

func WindowSwitchAction(ctx Context, item Item) tea.Cmd {
	windowID := item.ID
	session, _, ok := strings.Cut(windowID, ":")
	if !ok {
		return failCmd("invalid window id: %s", windowID)
	}
	label := item.Label
	return runAction(
		func() { events.Window.Switch(windowID) },
		func() error {
			if err := switchClientFn(ctx.SocketPath, ctx.ClientID, session); err != nil {
				return err
			}
			return selectWindowFn(ctx.SocketPath, windowID)
		},
		fmt.Sprintf("Switched to %s", label),
	)
}

func WindowKillAction(ctx Context, item Item) tea.Cmd {
	ids := splitSelectionIDs(item.ID)
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b string) int { return cmp.Compare(b, a) })
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
		return failCmd("invalid window target")
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
	initial = stripCurrentPrefix(initial)
	if initial == "" {
		if _, after, ok := strings.Cut(item.Label, " "); ok {
			initial = strings.TrimSpace(after)
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
			Layout:     w.Layout,
		})
	}
	return entries
}

func WindowRenameCommand(req RenameRequest) tea.Cmd {
	return func() tea.Msg {
		if req.Target == "" {
			return ActionResult{Err: fmt.Errorf("window target required")}
		}
		trimmed := strings.TrimSpace(req.Value)
		if trimmed == "" {
			return ActionResult{Err: fmt.Errorf("window name required")}
		}
		events.Window.Rename(req.Target, trimmed)
		if err := renameWindowFn(req.Context.SocketPath, req.Target, trimmed); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Renamed %s to %s", req.Target, trimmed)}
	}
}

func WindowLinkAction(ctx Context, item Item) tea.Cmd {
	source := strings.TrimSpace(item.ID)
	targetSession := strings.TrimSpace(ctx.CurrentWindowSession)
	if source == "" {
		return failCmd("invalid window target")
	}
	if targetSession == "" {
		return failCmd("no active session detected")
	}
	return runAction(
		func() { events.Window.Link(source, targetSession) },
		func() error { return linkWindowFn(ctx.SocketPath, source, targetSession) },
		fmt.Sprintf("Linked %s to %s", item.Label, targetSession),
	)
}

func WindowPullFromSessionAction(ctx Context, item Item) tea.Cmd {
	// Tree items have IDs like "tree:w:session:windowIndex".
	// Extract the tmux window target (session:windowIndex).
	source := strings.TrimSpace(item.ID)
	if trimmed, ok := strings.CutPrefix(source, TreePrefixWindow); ok {
		if sess, idx, ok := strings.Cut(trimmed, ":"); ok {
			source = sess + ":" + idx
		}
	}
	targetSession := strings.TrimSpace(ctx.CurrentWindowSession)
	if source == "" {
		return failCmd("invalid window target")
	}
	if targetSession == "" {
		return failCmd("no active session detected")
	}
	return runAction(
		func() { events.Window.PullFromSession(source, targetSession) },
		func() error { return moveWindowFn(ctx.SocketPath, source, targetSession) },
		fmt.Sprintf("Pulled %s into %s", source, targetSession),
	)
}

func WindowSwapAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return failCmd("invalid window target")
	}
	return func() tea.Msg {
		events.Window.SwapSelect(target)
		return WindowSwapPrompt{Context: ctx, First: item}
	}
}

func WindowSwapCommand(ctx Context, first, second Item) tea.Cmd {
	return func() tea.Msg {
		events.Window.Swap(first.ID, second.ID)
		if err := swapWindowsFn(ctx.SocketPath, first.ID, second.ID); err != nil {
			return ActionResult{Err: err}
		}
		firstLabel := first.Label
		if firstLabel == "" {
			firstLabel = first.ID
		}
		secondLabel := second.Label
		if secondLabel == "" {
			secondLabel = second.ID
		}
		return ActionResult{Info: fmt.Sprintf("Swapped %s ↔ %s", firstLabel, secondLabel)}
	}
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
	styleFormInput(&ti)
	ti.Placeholder = "window-name"
	ti.CharLimit = 64
	ti.SetWidth(40)
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

func (f *WindowRenameForm) Context() Context    { return f.ctx }
func (f *WindowRenameForm) Target() string      { return f.target }
func (f *WindowRenameForm) Title() string       { return f.title }
func (f *WindowRenameForm) Help() string        { return f.help }
func (f *WindowRenameForm) Value() string       { return strings.TrimSpace(f.input.Value()) }
func (f *WindowRenameForm) InputView() string   { return f.input.View() }
func (f *WindowRenameForm) Cursor() *tea.Cursor { return f.input.Cursor() }
func (f *WindowRenameForm) FocusCmd() tea.Cmd   { return f.input.Focus() }

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
	case tea.KeyPressMsg:
		switch m.String() {
		case "ctrl+u":
			if f.input.Value() != "" {
				f.input.SetValue("")
				f.input.CursorStart()
			}
			return nil, false, false
		case "esc":
			events.Window.CancelRename(f.target, events.ReasonEscape)
			return nil, false, true
		case "enter":
			name := f.Value()
			if name == "" {
				events.Window.CancelRename(f.target, events.ReasonEmpty)
				return nil, false, true
			}
			if strings.ContainsAny(name, "\n\r\t") {
				return nil, false, false
			}
			events.Window.SubmitRename(f.target, name)
			return WindowRenameCommand(RenameRequest{
				Context: f.ctx,
				Target:  f.target,
				Value:   name,
			}), true, false
		}
	}
	updated, cmd := f.input.Update(msg)
	f.input = updated
	return cmd, false, false
}
