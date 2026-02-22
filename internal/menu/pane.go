package menu

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var (
	switchPaneFn   = tmux.SwitchPane
	killPanesFn    = tmux.KillPanes
	movePaneFn     = tmux.MovePane
	breakPaneFn    = tmux.BreakPane
	swapPanesFn    = tmux.SwapPanes
	selectLayoutFn = tmux.SelectLayout
	resizePaneFn   = tmux.ResizePane
	renamePaneFn   = tmux.RenamePane
)

func loadPaneMenu(Context) ([]Item, error) {
	items := []string{
		// vvv do NOT reorder these! vvv
		"rename",
		"resize",
		"kill",
		"layout",
		"swap",
		"join",
		"break",
		"switch",
		// ^^^ do NOT reorder these! ^^^
	}
	return menuItemsFromIDs(items), nil
}

func paneItemFromEntry(entry PaneEntry) Item {
	return Item{ID: entry.ID, Label: entry.Label}
}

func currentPaneItem(ctx Context) (Item, bool) {
	id := strings.TrimSpace(ctx.CurrentPaneID)
	if id == "" {
		return Item{}, false
	}
	label := strings.TrimSpace(ctx.CurrentPaneLabel)
	if label == "" {
		label = id
	}
	return Item{ID: id, Label: "[current] " + label}, true
}

func loadPaneSwitchMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		if entry.Current && !ctx.PaneIncludeCurrent {
			continue
		}
		items = append(items, paneItemFromEntry(entry))
	}
	return items, nil
}

func loadPaneBreakMenu(ctx Context) ([]Item, error) {
	items := PaneEntriesToItems(ctx.Panes)
	if current, ok := currentPaneItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items, nil
}

func loadPaneJoinMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Panes))
	for _, entry := range ctx.Panes {
		if entry.Current {
			continue
		}
		items = append(items, paneItemFromEntry(entry))
	}
	return items, nil
}

func loadPaneSwapMenu(ctx Context) ([]Item, error) {
	items := PaneEntriesToItems(ctx.Panes)
	if current, ok := currentPaneItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items, nil
}

func loadPaneKillMenu(ctx Context) ([]Item, error) {
	items := PaneEntriesToItems(ctx.Panes)
	if current, ok := currentPaneItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items, nil
}

func loadPaneRenameMenu(ctx Context) ([]Item, error) {
	items := PaneEntriesToItems(ctx.Panes)
	if current, ok := currentPaneItem(ctx); ok {
		items = append([]Item{current}, items...)
	}
	return items, nil
}

func loadPaneLayoutMenu(Context) ([]Item, error) {
	layouts := []string{
		"even-horizontal",
		"even-vertical",
		"main-horizontal",
		"main-vertical",
		"tiled",
	}
	return menuItemsFromIDs(layouts), nil
}

func loadPaneResizeMenu(Context) ([]Item, error) {
	directions := []string{"left", "right", "up", "down"}
	return menuItemsFromIDs(directions), nil
}

func loadPaneResizeAmountMenu(direction string) ([]Item, error) {
	var values []string
	switch direction {
	case "left", "right":
		values = []string{"1", "2", "3", "5", "10", "20", "30"}
	case "up", "down":
		values = []string{"1", "2", "3", "5", "10", "15", "20"}
	default:
		values = []string{"1", "2", "3"}
	}
	return menuItemsFromIDs(values), nil
}

func loadPaneResizeLeftMenu(Context) ([]Item, error)  { return loadPaneResizeAmountMenu("left") }
func loadPaneResizeRightMenu(Context) ([]Item, error) { return loadPaneResizeAmountMenu("right") }
func loadPaneResizeUpMenu(Context) ([]Item, error)    { return loadPaneResizeAmountMenu("up") }
func loadPaneResizeDownMenu(Context) ([]Item, error)  { return loadPaneResizeAmountMenu("down") }

func PaneSwitchAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid pane target")} }
	}
	label := item.Label
	return func() tea.Msg {
		events.Pane.Switch(target)
		if err := switchPaneFn(ctx.SocketPath, ctx.ClientID, target); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Switched to %s", label)}
	}
}

func PaneKillAction(ctx Context, item Item) tea.Cmd {
	ids := splitPaneIDs(item.ID)
	sorted := append([]string(nil), ids...)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	label := item.Label
	return func() tea.Msg {
		events.Pane.Kill(sorted)
		if err := killPanesFn(ctx.SocketPath, sorted); err != nil {
			return ActionResult{Err: err}
		}
		if len(sorted) == 1 {
			return ActionResult{Info: fmt.Sprintf("Killed %s", label)}
		}
		return ActionResult{Info: fmt.Sprintf("Killed %d panes", len(sorted))}
	}
}

func PaneJoinAction(ctx Context, item Item) tea.Cmd {
	ids := splitPaneIDs(item.ID)
	sorted := append([]string(nil), ids...)
	sort.Sort(sort.Reverse(sort.StringSlice(sorted)))
	target := strings.TrimSpace(ctx.CurrentPaneID)
	return func() tea.Msg {
		events.Pane.Join(sorted, target)
		for _, id := range sorted {
			if err := movePaneFn(ctx.SocketPath, id, ""); err != nil {
				return ActionResult{Err: err}
			}
		}
		return ActionResult{Info: fmt.Sprintf("Joined %d pane(s)", len(sorted))}
	}
}

func PaneBreakAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	label := item.Label
	session := ctx.CurrentWindowSession
	if session == "" {
		parts := strings.SplitN(target, ":", 2)
		if len(parts) > 0 {
			session = parts[0]
		}
	}
	nextIdx := 0
	for _, win := range ctx.Windows {
		if win.Session == session && win.Index >= nextIdx {
			nextIdx = win.Index + 1
		}
	}
	destination := ""
	if session != "" {
		destination = fmt.Sprintf("%s:%d", session, nextIdx)
	}
	return func() tea.Msg {
		events.Pane.Break(target, destination)
		if err := breakPaneFn(ctx.SocketPath, target, destination); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Broke %s into new window", label)}
	}
}

func PaneSwapAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid pane target")} }
	}
	return func() tea.Msg {
		events.Pane.SwapSelect(target)
		return PaneSwapPrompt{Context: ctx, First: item}
	}
}

func PaneSwapCommand(ctx Context, first, second Item) tea.Cmd {
	return func() tea.Msg {
		events.Pane.Swap(first.ID, second.ID)
		if err := swapPanesFn(ctx.SocketPath, first.ID, second.ID); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Swapped %s ↔ %s", first.Label, second.Label)}
	}
}

func PaneLayoutAction(ctx Context, item Item) tea.Cmd {
	layout := strings.TrimSpace(item.ID)
	if layout == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid layout")} }
	}
	return func() tea.Msg {
		events.Pane.Layout(layout)
		if err := selectLayoutFn(ctx.SocketPath, layout); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Applied layout %s", layout)}
	}
}

func PaneResizeAction(ctx Context, direction, amount string) tea.Cmd {
	size, err := strconv.Atoi(amount)
	if err != nil {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid amount")} }
	}
	return func() tea.Msg {
		events.Pane.Resize(direction, size)
		if err := resizePaneFn(ctx.SocketPath, direction, size); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Resized %s by %d", direction, size)}
	}
}

func PaneResizeLeftAction(ctx Context, item Item) tea.Cmd {
	return PaneResizeAction(ctx, "left", item.ID)
}
func PaneResizeRightAction(ctx Context, item Item) tea.Cmd {
	return PaneResizeAction(ctx, "right", item.ID)
}
func PaneResizeUpAction(ctx Context, item Item) tea.Cmd { return PaneResizeAction(ctx, "up", item.ID) }
func PaneResizeDownAction(ctx Context, item Item) tea.Cmd {
	return PaneResizeAction(ctx, "down", item.ID)
}

func PaneRenameAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid pane target")} }
	}
	initial := strings.TrimSpace(item.Label)
	for _, entry := range ctx.Panes {
		if entry.ID == target {
			if entry.Title != "" {
				initial = entry.Title
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
	return func() tea.Msg {
		events.Pane.RenamePrompt(target)
		return PanePrompt{Context: ctx, Target: target, Initial: initial}
	}
}

func PaneRenameCommand(ctx Context, target, title string) tea.Cmd {
	return func() tea.Msg {
		trimmedTarget := strings.TrimSpace(target)
		if trimmedTarget == "" {
			return ActionResult{Err: fmt.Errorf("pane target required")}
		}
		trimmedTitle := strings.TrimSpace(title)
		if trimmedTitle == "" {
			return ActionResult{Err: fmt.Errorf("pane title required")}
		}
		targetPane := trimmedTarget
		paneLabel := trimmedTarget
		for _, entry := range ctx.Panes {
			if strings.TrimSpace(entry.ID) == trimmedTarget {
				paneLabel = entry.Label
				if id := strings.TrimSpace(entry.PaneID); id != "" {
					targetPane = id
				}
				break
			}
		}
		events.Pane.Rename(targetPane, trimmedTitle)
		if err := renamePaneFn(ctx.SocketPath, targetPane, trimmedTitle); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Renamed %s to %s", paneLabel, trimmedTitle)}
	}
}

func PaneEntriesFromTmux(panes []tmux.Pane) []PaneEntry {
	entries := make([]PaneEntry, 0, len(panes))
	for _, p := range panes {
		entries = append(entries, PaneEntry{
			ID:        p.ID,
			Label:     p.Label,
			PaneID:    p.PaneID,
			Session:   p.Session,
			Window:    p.Window,
			WindowIdx: p.WindowIdx,
			Index:     p.Index,
			Current:   p.Current,
			Title:     p.Title,
		})
	}
	return entries
}

func PaneEntriesToItems(entries []PaneEntry) []Item {
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, Item{ID: entry.ID, Label: entry.Label})
	}
	return items
}

func splitPaneIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ',' || r == ' '
	})
	seen := make(map[string]struct{}, len(parts))
	ids := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		ids = append(ids, clean)
	}
	return ids
}

// PaneSwapPrompt asks the UI to select a second pane for swapping.
type PaneSwapPrompt struct {
	Context Context
	First   Item
}

type PanePrompt struct {
	Context Context
	Target  string
	Initial string
}

type PaneRenameForm struct {
	input  textinput.Model
	ctx    Context
	target string
	help   string
	title  string
}

func NewPaneRenameForm(prompt PanePrompt) *PaneRenameForm {
	ti := textinput.New()
	ti.Placeholder = "pane-title"
	ti.CharLimit = 128
	if prompt.Initial != "" {
		ti.SetValue(prompt.Initial)
		ti.CursorEnd()
	}
	ti.Focus()
	title := fmt.Sprintf("Rename %s", prompt.Target)
	if prompt.Initial != "" {
		title = fmt.Sprintf("Rename %s", prompt.Initial)
	}
	return &PaneRenameForm{
		input:  ti,
		ctx:    prompt.Context,
		target: prompt.Target,
		help:   "Press Enter to rename. Esc to cancel.",
		title:  title,
	}
}

func (f *PaneRenameForm) Context() Context  { return f.ctx }
func (f *PaneRenameForm) Target() string    { return f.target }
func (f *PaneRenameForm) Title() string     { return f.title }
func (f *PaneRenameForm) Help() string      { return f.help }
func (f *PaneRenameForm) Value() string     { return strings.TrimSpace(f.input.Value()) }
func (f *PaneRenameForm) InputView() string { return f.input.View() }

func (f *PaneRenameForm) ActionID() string { return "pane:rename" }

func (f *PaneRenameForm) PendingLabel() string {
	name := f.Value()
	if name == "" {
		return f.ActionID()
	}
	return fmt.Sprintf("%s → %s", f.target, name)
}

func (f *PaneRenameForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
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
			events.Pane.CancelRename(f.target, events.PaneReasonEscape)
			return nil, false, true
		case tea.KeyEnter:
			title := f.Value()
			if title == "" {
				events.Pane.CancelRename(f.target, events.PaneReasonEmpty)
				return nil, false, true
			}
			events.Pane.SubmitRename(f.target, title)
			return nil, true, false
		}
	}
	updated, cmd := f.input.Update(msg)
	f.input = updated
	return cmd, false, false
}

func (f *PaneRenameForm) SyncContext(ctx Context) {
	f.ctx = ctx
}
