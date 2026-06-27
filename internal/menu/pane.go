package menu

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

var (
	switchPaneFn = tmux.SwitchPane
	killPanesFn  = tmux.KillPanes
	joinPaneFn   = tmux.JoinPane
	breakPaneFn  = tmux.BreakPane
	swapPanesFn  = tmux.SwapPanes
	resizePaneFn = tmux.ResizePane
	renamePaneFn = tmux.RenamePane
)

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
	return Item{ID: id, Label: currentLabelPrefix + label}, true
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
	current, ok := currentPaneItem(ctx)
	return withCurrentFirst(PaneEntriesToItems(ctx.Panes), current, ok), nil
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
	current, ok := currentPaneItem(ctx)
	return withCurrentFirst(PaneEntriesToItems(ctx.Panes), current, ok), nil
}

func loadPaneKillMenu(ctx Context) ([]Item, error) {
	current, ok := currentPaneItem(ctx)
	return withCurrentFirst(PaneEntriesToItems(ctx.Panes), current, ok), nil
}

func loadPaneRenameMenu(ctx Context) ([]Item, error) {
	current, ok := currentPaneItem(ctx)
	return withCurrentFirst(PaneEntriesToItems(ctx.Panes), current, ok), nil
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
		return failCmd("invalid pane target")
	}
	label := item.Label
	return runAction(
		func() { events.Pane.Switch(target) },
		func() error { return switchPaneFn(ctx.SocketPath, ctx.ClientID, target) },
		fmt.Sprintf("Switched to %s", label),
	)
}

func PaneKillAction(ctx Context, item Item) tea.Cmd {
	ids := splitSelectionIDs(item.ID)
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b string) int { return cmp.Compare(b, a) })
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
	ids := splitSelectionIDs(item.ID)
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b string) int { return cmp.Compare(b, a) })
	target := strings.TrimSpace(ctx.CurrentPaneID)
	// Validate before tracing (consistent with PaneSwitchAction) so we never
	// emit a join trace for a join that cannot happen.
	if target == "" {
		return failCmd("no current pane to join into")
	}
	return func() tea.Msg {
		events.Pane.Join(sorted, target)
		for _, id := range sorted {
			if err := joinPaneFn(ctx.SocketPath, id, target); err != nil {
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
		session, _, _ = strings.Cut(target, ":")
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
		return failCmd("invalid pane target")
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

func PaneResizeAction(ctx Context, direction, amount string) tea.Cmd {
	size, err := strconv.Atoi(amount)
	if err != nil {
		return failCmd("invalid amount")
	}
	return runAction(
		func() { events.Pane.Resize(direction, size) },
		func() error { return resizePaneFn(ctx.SocketPath, direction, size) },
		fmt.Sprintf("Resized %s by %d", direction, size),
	)
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
		return failCmd("invalid pane target")
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
	initial = stripCurrentPrefix(initial)
	return func() tea.Msg {
		events.Pane.RenamePrompt(target)
		return PanePrompt{Context: ctx, Target: target, Initial: initial}
	}
}

func PaneRenameCommand(req RenameRequest) tea.Cmd {
	return func() tea.Msg {
		trimmedTarget := strings.TrimSpace(req.Target)
		if trimmedTarget == "" {
			return ActionResult{Err: fmt.Errorf("pane target required")}
		}
		trimmedTitle := strings.TrimSpace(req.Value)
		if trimmedTitle == "" {
			return ActionResult{Err: fmt.Errorf("pane title required")}
		}
		targetPane := trimmedTarget
		paneLabel := trimmedTarget
		for _, entry := range req.Context.Panes {
			if strings.TrimSpace(entry.ID) == trimmedTarget {
				paneLabel = entry.Label
				if id := strings.TrimSpace(entry.PaneID); id != "" {
					targetPane = id
				}
				break
			}
		}
		events.Pane.Rename(targetPane, trimmedTitle)
		if err := renamePaneFn(req.Context.SocketPath, targetPane, trimmedTitle); err != nil {
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
			Command:   p.Command,
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

// PaneSwapPrompt asks the UI to select a second pane for swapping.
type PaneSwapPrompt struct {
	Context Context
	First   Item
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
	styleFormInput(&ti)
	ti.Placeholder = "pane-title"
	ti.CharLimit = 128
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
	return &PaneRenameForm{
		input:  ti,
		ctx:    prompt.Context,
		target: prompt.Target,
		help:   "Press Enter to rename. Esc to cancel.",
		title:  title,
	}
}

func (f *PaneRenameForm) Context() Context    { return f.ctx }
func (f *PaneRenameForm) Target() string      { return f.target }
func (f *PaneRenameForm) Title() string       { return f.title }
func (f *PaneRenameForm) Help() string        { return f.help }
func (f *PaneRenameForm) Value() string       { return strings.TrimSpace(f.input.Value()) }
func (f *PaneRenameForm) InputView() string   { return f.input.View() }
func (f *PaneRenameForm) Cursor() *tea.Cursor { return f.input.Cursor() }
func (f *PaneRenameForm) FocusCmd() tea.Cmd   { return f.input.Focus() }

func (f *PaneRenameForm) ActionID() string { return "pane:rename" }

func (f *PaneRenameForm) PendingLabel() string {
	name := f.Value()
	if name == "" {
		return f.ActionID()
	}
	return fmt.Sprintf("%s → %s", f.target, name)
}

func (f *PaneRenameForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
	if m, ok := msg.(tea.KeyPressMsg); ok {
		switch m.String() {
		case "ctrl+u":
			if f.input.Value() != "" {
				f.input.SetValue("")
				f.input.CursorStart()
			}
			return nil, false, false
		case "esc":
			events.Pane.CancelRename(f.target, events.PaneReasonEscape)
			return nil, false, true
		case "enter":
			title := f.Value()
			if title == "" {
				events.Pane.CancelRename(f.target, events.PaneReasonEmpty)
				return nil, false, true
			}
			if strings.ContainsAny(title, "\n\r\t") {
				return nil, false, false
			}
			events.Pane.SubmitRename(f.target, title)
			return PaneRenameCommand(RenameRequest{
				Context: f.ctx,
				Target:  f.target,
				Value:   title,
			}), true, false
		}
	}
	updated, cmd := f.input.Update(msg)
	f.input = updated
	return cmd, false, false
}

func (f *PaneRenameForm) SyncContext(ctx Context) {
	f.ctx = ctx
}

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
		return failCmd("no current pane")
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
	styleFormInput(&ti)
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
func (f *PaneCaptureForm) Cursor() *tea.Cursor { return f.input.Cursor() }
func (f *PaneCaptureForm) EscSeqs() bool       { return f.escSeqs }
func (f *PaneCaptureForm) Preview() string     { return f.preview }
func (f *PaneCaptureForm) PreviewErr() string  { return f.previewErr }
func (f *PaneCaptureForm) Seq() int            { return f.seq }
func (f *PaneCaptureForm) FocusCmd() tea.Cmd   { return f.input.Focus() }
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
	if m, ok := msg.(tea.KeyPressMsg); ok {
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
			events.Pane.CaptureCancel(events.PaneReasonEscape)
			return nil, false, true
		case "enter":
			v := f.Value()
			if v == "" {
				events.Pane.CaptureCancel(events.PaneReasonEmpty)
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
		// owner-only (0700) for consistency with other sensitive dirs (the
		// captured file holds pane scrollback that may contain secrets).
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return ActionResult{Err: fmt.Errorf("create directory %s: %w", dir, err)}
		}

		events.Pane.Capture(target, resolved, escSeqs)
		if err := tmux.CapturePaneToFile(ctx.SocketPath, target, resolved, escSeqs); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("captured to %s", resolved)}
	}
}
