package menu

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// ResurrectStart triggers a save or restore operation from a menu action.
type ResurrectStart struct {
	Operation string // "save" or "restore"
	Name      string // snapshot name (save-as only)
	SaveFile  string // path to restore from
	Config    resurrect.Config
}

// SaveAsPrompt requests interactive input for naming a snapshot.
type SaveAsPrompt struct {
	Context Context
	SaveDir string
}

func loadSessionMenu(Context) ([]Item, error) {
	items := []Item{
		{ID: "save", Label: "save"},
		{ID: "save-as", Label: "save-as"},
		{ID: "restore", Label: "restore"},
		{ID: "restore-from", Label: "restore-from"},
	}
	items = append(items, menuItemsFromIDs([]string{
		"kill",
		"detach",
		"rename",
		"new",
		"switch",
		"tree",
	})...)
	return items, nil
}

func loadSessionSwitchMenu(ctx Context) ([]Item, error) {
	return SessionSwitchMenuItems(ctx), nil
}

func loadSessionRenameMenu(ctx Context) ([]Item, error) {
	return SessionRenameItems(ctx.Sessions), nil
}

func loadSessionDetachMenu(ctx Context) ([]Item, error) {
	return SessionEntriesToItems(ctx.Sessions), nil
}

func loadSessionKillMenu(ctx Context) ([]Item, error) {
	return SessionEntriesToItems(ctx.Sessions), nil
}

func SessionNewAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		events.Session.NewPrompt(len(ctx.Sessions))
		return SessionPrompt{Context: ctx, Action: "session:new"}
	}
}

func SessionSwitchAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		events.Session.Switch(item.ID)
		if err := tmux.SwitchClient(ctx.SocketPath, ctx.ClientID, item.ID); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Switched to %s", item.Label)}
	}
}

func SessionRenameAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid session target")} }
	}
	return func() tea.Msg {
		events.Session.RenamePrompt(target)
		return SessionPrompt{
			Context: ctx,
			Action:  "session:rename",
			Target:  target,
			Initial: target,
		}
	}
}

func SessionDetachAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	label := strings.TrimSpace(item.Label)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid session target")} }
	}
	return func() tea.Msg {
		events.Session.Detach(target)
		if err := tmux.DetachSessions(ctx.SocketPath, []string{target}); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Detached %s", label)}
	}
}

func SessionKillAction(ctx Context, item Item) tea.Cmd {
	target := strings.TrimSpace(item.ID)
	label := strings.TrimSpace(item.Label)
	if target == "" {
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid session target")} }
	}
	return func() tea.Msg {
		events.Session.Kill(target)
		if err := tmux.KillSessions(ctx.SocketPath, []string{target}); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Killed %s", label)}
	}
}

func SessionCreateCommand(req SessionRequest) tea.Cmd {
	return func() tea.Msg {
		name := strings.TrimSpace(req.Value)
		events.Session.Create(name)
		if err := tmux.NewSession(req.Context.SocketPath, name); err != nil {
			return ActionResult{Err: err}
		}
		if err := tmux.SwitchClient(req.Context.SocketPath, req.Context.ClientID, name); err != nil {
			return ActionResult{Err: fmt.Errorf("created session %s but failed to switch: %w", name, err)}
		}
		return ActionResult{Info: fmt.Sprintf("Created and switched to %s", name)}
	}
}

func SessionRenameCommand(req SessionRequest) tea.Cmd {
	return func() tea.Msg {
		target := strings.TrimSpace(req.Target)
		if target == "" {
			return ActionResult{Err: fmt.Errorf("session target required")}
		}
		name := strings.TrimSpace(req.Value)
		if name == "" {
			return ActionResult{Err: fmt.Errorf("session name required")}
		}
		events.Session.Rename(target, name)
		if err := tmux.RenameSession(req.Context.SocketPath, target, name); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Renamed %s to %s", target, name)}
	}
}

func SessionCommandForAction(req SessionRequest) tea.Cmd {
	switch req.Action {
	case "session:rename":
		return SessionRenameCommand(req)
	default:
		return SessionCreateCommand(req)
	}
}

type SessionForm struct {
	input      textinput.Model
	existing   map[string]struct{}
	ctx        Context
	err        string
	errWarning bool // true when err is a hint (e.g. empty field), not a hard error
	dirty      bool // true after the user's first interaction
	mode       sessionFormMode
	target     string
	action     string
	title      string
	help       string
}

type sessionFormMode int

const (
	sessionFormModeCreate sessionFormMode = iota
	sessionFormModeRename
)

func NewSessionForm(prompt SessionPrompt) *SessionForm {
	ti := textinput.New()
	styleFormInput(&ti)
	ti.Placeholder = "session-name"
	ti.CharLimit = 64
	ti.SetWidth(40)
	ti.Focus()
	if prompt.Initial != "" {
		ti.SetValue(prompt.Initial)
	}
	mode := sessionFormModeCreate
	title := "new"
	help := "Press Enter to create. Esc to cancel."
	target := strings.TrimSpace(prompt.Target)
	switch prompt.Action {
	case "session:rename":
		mode = sessionFormModeRename
		if target != "" {
			title = fmt.Sprintf("rename %s", target)
		} else {
			title = "rename"
		}
		help = "Press Enter to rename. Esc to cancel."
	}
	form := &SessionForm{
		input:    ti,
		existing: map[string]struct{}{},
		ctx:      prompt.Context,
		mode:     mode,
		target:   target,
		action:   prompt.Action,
		title:    title,
		help:     help,
		dirty:    prompt.Initial != "",
	}
	form.SetSessions(prompt.Context.Sessions)
	return form
}

func (f *SessionForm) Context() Context     { return f.ctx }
func (f *SessionForm) Value() string        { return strings.TrimSpace(f.input.Value()) }
func (f *SessionForm) InputView() string    { return f.input.View() }
func (f *SessionForm) Error() string        { return f.err }
func (f *SessionForm) ErrorIsWarning() bool { return f.errWarning }
func (f *SessionForm) Action() string       { return f.action }
func (f *SessionForm) Target() string       { return f.target }
func (f *SessionForm) Title() string        { return f.title }
func (f *SessionForm) Help() string         { return f.help }
func (f *SessionForm) IsRename() bool       { return f.mode == sessionFormModeRename }
func (f *SessionForm) FocusCmd() tea.Cmd    { return f.input.Focus() }

func (f *SessionForm) ActionID() string {
	if f.action != "" {
		return f.action
	}
	return "session:new"
}

func (f *SessionForm) PendingLabel() string {
	name := f.Value()
	if name == "" {
		return f.ActionID()
	}
	if f.ActionID() == "session:rename" && f.target != "" {
		return fmt.Sprintf("%s → %s", f.target, name)
	}
	return name
}

func (f *SessionForm) Request() SessionRequest {
	return SessionRequest{
		Context: f.ctx,
		Action:  f.ActionID(),
		Target:  f.target,
		Value:   f.Value(),
	}
}

func (f *SessionForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
	switch m := msg.(type) {
	case tea.KeyPressMsg:
		switch m.String() {
		case "ctrl+u":
			if f.input.Value() != "" {
				f.dirty = true
				f.input.SetValue("")
				f.input.CursorStart()
				f.err = f.validate()
			}
			return nil, false, false
		case "esc":
			if f.mode == sessionFormModeRename {
				events.Session.CancelRename(f.target, events.SessionReasonEscape)
			} else {
				events.Session.CancelNew(events.SessionReasonEscape)
			}
			return nil, false, true
		case "enter":
			f.dirty = true
			value := f.Value()
			switch f.mode {
			case sessionFormModeCreate:
				if err, _ := f.validateName(value); err != "" {
					f.err = f.validate()
					return nil, false, false
				}
				f.err = ""
				events.Session.SubmitNew(value)
				return SessionCreateCommand(f.Request()), true, false
			case sessionFormModeRename:
				if value == "" {
					events.Session.CancelRename(f.target, events.SessionReasonEmpty)
					return nil, false, true
				}
				if err, _ := f.validateName(value); err != "" {
					f.err = f.validate()
					return nil, false, false
				}
				f.err = ""
				events.Session.SubmitRename(f.target, value)
				return SessionRenameCommand(f.Request()), true, false
			}
		}
	}

	updated, cmd := f.input.Update(msg)
	if f.input.Value() != updated.Value() {
		f.dirty = true
	}
	f.input = updated
	f.err = f.validate()
	return cmd, false, false
}

func (f *SessionForm) SetSessions(entries []SessionEntry) {
	f.existing = make(map[string]struct{}, len(entries))
	targetLower := strings.ToLower(strings.TrimSpace(f.target))
	for _, entry := range entries {
		trim := strings.ToLower(strings.TrimSpace(entry.Name))
		if trim == "" {
			continue
		}
		if f.mode == sessionFormModeRename && trim == targetLower {
			continue
		}
		f.existing[trim] = struct{}{}
	}
	if f.dirty {
		f.err = f.validate()
	}
}

func (f *SessionForm) validate() string {
	msg, warning := f.validateName(f.Value())
	f.errWarning = warning
	return msg
}

func (f *SessionForm) validateName(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	lower := strings.ToLower(trimmed)
	// Reject control characters and colons that break tmux target parsing.
	if strings.ContainsAny(trimmed, "\n\r\t:") {
		return "name must not contain control characters or colons", false
	}
	switch f.mode {
	case sessionFormModeRename:
		if trimmed == "" {
			return "", false
		}
		if _, exists := f.existing[lower]; exists {
			return "Session already exists", false
		}
		return "", false
	default:
		if trimmed == "" {
			if !f.dirty {
				return "", false
			}
			return "Session name required", true
		}
		if _, exists := f.existing[lower]; exists {
			return "Session already exists", false
		}
		return "", false
	}
}

func SessionSaveAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		return ResurrectStart{
			Operation: "save",
			Config: resurrect.Config{
				SocketPath:          ctx.SocketPath,
				SaveDir:             dir,
				CapturePaneContents: resurrect.ResolvePaneContents(ctx.SocketPath),
				ClientID:            ctx.ClientID,
			},
		}
	}
}

func SessionSaveAsAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		return SaveAsPrompt{Context: ctx, SaveDir: dir}
	}
}

func SessionRestoreAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		path, err := resurrect.LatestSave(dir)
		if err != nil {
			return ActionResult{Err: err}
		}
		return ResurrectStart{
			Operation: "restore",
			SaveFile:  path,
			Config: resurrect.Config{
				SocketPath:          ctx.SocketPath,
				SaveDir:             dir,
				CapturePaneContents: resurrect.ResolvePaneContents(ctx.SocketPath),
				ClientID:            ctx.ClientID,
			},
		}
	}
}

func SessionRestoreFromAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		dir, err := resurrect.ResolveDir(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: fmt.Errorf("resolving save dir: %w", err)}
		}
		return ResurrectStart{
			Operation: "restore",
			SaveFile:  item.ID,
			Config: resurrect.Config{
				SocketPath:          ctx.SocketPath,
				SaveDir:             dir,
				CapturePaneContents: resurrect.ResolvePaneContents(ctx.SocketPath),
				ClientID:            ctx.ClientID,
			},
		}
	}
}

func loadSessionRestoreFromMenu(ctx Context) ([]Item, error) {
	dir, err := resurrect.ResolveDir(ctx.SocketPath)
	if err != nil {
		return nil, nil // empty list, no error shown
	}
	entries, err := resurrect.ListSaves(dir)
	if err != nil {
		return nil, nil
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// Reverse to oldest-first so the most recent entry is at the bottom.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	now := time.Now()
	alignments := []table.Alignment{
		table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignLeft, table.AlignRight, table.AlignLeft,
	}
	headerRow := []string{"name", "type", "age", "date", "time", "size", "info"}
	rows := make([][]string, len(entries))
	ids := make([]string, len(entries))
	for i, e := range entries {
		saveType := string(e.Kind)
		if saveType == "" {
			saveType = string(resurrect.SaveKindManual)
		}
		name := e.DisplayName()
		if e.Kind == resurrect.SaveKindAuto {
			name = "auto"
		}
		age := resurrect.RelativeTime(e.Timestamp, now)
		date := e.Timestamp.Format("2006-01-02")
		timeStr := e.Timestamp.Format("15:04:05")
		size := humanizeSaveSize(e.Size)
		info := fmt.Sprintf("%2ds %3dw %3dp", e.SessionCount, e.WindowCount, e.PaneCount)
		if e.HasPaneContents {
			info += " +contents"
		}
		rows[i] = []string{name, saveType, age, date, timeStr, size, info}
		ids[i] = e.Path
	}
	aligned := formatRestoreRows(headerRow, rows, alignments)
	items := make([]Item, len(aligned))
	items[0] = Item{Label: aligned[0], Header: true}
	for i := 1; i < len(aligned); i++ {
		entry := entries[i-1]
		items[i] = Item{
			ID:          ids[i-1],
			Label:       aligned[i],
			StyledLabel: styleSaveEntryLine(aligned[i], entry.Kind),
		}
	}
	return items, nil
}

func formatRestoreRows(header []string, rows [][]string, alignments []table.Alignment) []string {
	allRows := append([][]string{header}, rows...)
	widths := restoreColumnWidths(allRows)
	out := make([]string, 1+len(rows))
	out[0] = formatRestoreRow(header, widths, nil)
	for i, row := range rows {
		out[i+1] = formatRestoreRow(row, widths, alignments)
	}
	return out
}

func restoreColumnWidths(rows [][]string) []int {
	if len(rows) == 0 {
		return nil
	}
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for i, cell := range row {
			if w := len([]rune(cell)); w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
}

func formatRestoreRow(row []string, widths []int, alignments []table.Alignment) string {
	var b strings.Builder
	for i, cell := range row {
		if i > 0 {
			b.WriteString("  ")
		}
		padding := widths[i] - len([]rune(cell))
		if padding < 0 {
			padding = 0
		}
		if i < len(alignments) && alignments[i] == table.AlignRight {
			b.WriteString(strings.Repeat(" ", padding))
			b.WriteString(cell)
			continue
		}
		b.WriteString(cell)
		b.WriteString(strings.Repeat(" ", padding))
	}
	return b.String()
}

func humanizeSaveSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, suffix := range units {
		value /= unit
		if value < unit || suffix == units[len(units)-1] {
			rounded := math.Round(value*10) / 10
			if rounded == math.Trunc(rounded) {
				return fmt.Sprintf("%.0f %s", rounded, suffix)
			}
			return fmt.Sprintf("%.1f %s", rounded, suffix)
		}
	}
	return fmt.Sprintf("%d B", size)
}

var (
	saveEntryFgManual = ansi.NewStyle().ForegroundColor(ansi.IndexedColor(33)).String()
	saveEntryFgAuto   = ansi.NewStyle().ForegroundColor(ansi.IndexedColor(93)).String()
	saveEntryFgReset  = ansi.NewStyle().ForegroundColor(nil).String()
)

func styleSaveEntryLine(line string, kind resurrect.SaveKind) string {
	switch kind {
	case resurrect.SaveKindAuto:
		return saveEntryFgAuto + line + saveEntryFgReset
	default:
		return saveEntryFgManual + line + saveEntryFgReset
	}
}

func loadSessionTreeMenu(ctx Context) ([]Item, error) {
	allExpanded := strings.TrimSpace(ctx.MenuArgs) == "expanded"
	treeState := NewTreeState(allExpanded)
	items := treeState.BuildTreeItems(TreeItemsInput{
		Sessions: ctx.Sessions,
		Windows:  ctx.Windows,
		Panes:    ctx.Panes,
	})
	return items, nil
}

// SessionTreeAction handles Enter on a tree item. It parses the item ID
// prefix to determine whether to switch session, window, or pane.
func SessionTreeAction(ctx Context, item Item) tea.Cmd {
	id := item.ID
	switch {
	case strings.HasPrefix(id, TreePrefixPane):
		// tree:p:session:windowIndex:paneDisplayID
		parts := strings.SplitN(strings.TrimPrefix(id, TreePrefixPane), ":", 3)
		if len(parts) < 3 {
			return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid pane target: %s", id)} }
		}
		session, paneTarget := parts[0], parts[2]
		return func() tea.Msg {
			events.Session.Switch(session)
			if err := tmux.SwitchClient(ctx.SocketPath, ctx.ClientID, session); err != nil {
				return ActionResult{Err: err}
			}
			if err := tmux.SwitchPane(ctx.SocketPath, ctx.ClientID, paneTarget); err != nil {
				return ActionResult{Err: err}
			}
			return ActionResult{Info: fmt.Sprintf("Switched to pane %s", paneTarget)}
		}
	case strings.HasPrefix(id, TreePrefixWindow):
		// tree:w:session:windowIndex
		parts := strings.SplitN(strings.TrimPrefix(id, TreePrefixWindow), ":", 2)
		if len(parts) < 2 {
			return func() tea.Msg { return ActionResult{Err: fmt.Errorf("invalid window target: %s", id)} }
		}
		session, windowIdx := parts[0], parts[1]
		windowTarget := session + ":" + windowIdx
		return func() tea.Msg {
			events.Session.Switch(session)
			if err := tmux.SwitchClient(ctx.SocketPath, ctx.ClientID, session); err != nil {
				return ActionResult{Err: err}
			}
			if err := tmux.SelectWindow(ctx.SocketPath, windowTarget); err != nil {
				return ActionResult{Err: err}
			}
			return ActionResult{Info: fmt.Sprintf("Switched to window %s", windowTarget)}
		}
	case strings.HasPrefix(id, TreePrefixSession):
		session := strings.TrimPrefix(id, TreePrefixSession)
		return func() tea.Msg {
			events.Session.Switch(session)
			if err := tmux.SwitchClient(ctx.SocketPath, ctx.ClientID, session); err != nil {
				return ActionResult{Err: err}
			}
			return ActionResult{Info: fmt.Sprintf("Switched to %s", session)}
		}
	default:
		return func() tea.Msg { return ActionResult{Err: fmt.Errorf("unknown tree item: %s", id)} }
	}
}

func SessionEntriesFromTmux(sessions []tmux.Session) []SessionEntry {
	entries := make([]SessionEntry, 0, len(sessions))
	for _, sess := range sessions {
		entry := SessionEntry{
			Name:     sess.Name,
			Label:    sess.Label,
			Attached: sess.Attached,
			Current:  sess.Current,
			Clients:  append([]string(nil), sess.Clients...),
			Windows:  sess.Windows,
		}
		entries = append(entries, entry)
	}
	return entries
}

func sessionDisplayLabel(entry SessionEntry) string {
	label := strings.TrimSpace(entry.Label)
	if label == "" {
		label = entry.Name
	}
	if entry.Current {
		if !strings.HasPrefix(label, "[ current ]") {
			label = "[ current ] " + label
		}
	}
	return label
}

func SessionEntriesToItems(entries []SessionEntry) []Item {
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, Item{ID: entry.Name, Label: entry.Label})
	}
	return items
}

func SessionRenameItems(entries []SessionEntry) []Item {
	ordered := make([]SessionEntry, 0, len(entries))
	var current *SessionEntry
	for _, entry := range entries {
		if entry.Current {
			copy := entry
			current = &copy
			continue
		}
		ordered = append(ordered, entry)
	}
	if current != nil {
		ordered = append(ordered, *current)
	}
	return sessionTableItems(ordered)
}

// SessionSwitchMenuItems produces the formatted table used on the session switch screen.
func SessionSwitchMenuItems(ctx Context) []Item {
	entries := make([]SessionEntry, 0, len(ctx.Sessions))
	for _, entry := range ctx.Sessions {
		if entry.Current && !ctx.IncludeCurrent {
			continue
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil
	}
	return sessionTableItems(entries)
}

func sessionTableItems(entries []SessionEntry) []Item {
	if len(entries) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(entries))
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name
		windows := fmt.Sprintf("%d windows", entry.Windows)
		status := sessionStatus(entry)
		current := ""
		if entry.Current {
			current = "current"
		}
		rows = append(rows, []string{name, windows, status, current})
		ids = append(ids, entry.Name)
	}
	aligned := table.Format(rows, []table.Alignment{table.AlignLeft, table.AlignRight, table.AlignLeft, table.AlignLeft})
	items := make([]Item, len(aligned))
	for i, label := range aligned {
		items[i] = Item{ID: ids[i], Label: label}
	}
	return items
}

func sessionStatus(entry SessionEntry) string {
	if !entry.Attached {
		return ""
	}
	status := "attached"
	if count := len(entry.Clients); count > 1 {
		status = fmt.Sprintf("attached (%d)", count)
	}
	return status
}
