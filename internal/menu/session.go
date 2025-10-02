package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func loadSessionMenu(Context) ([]Item, error) {
	items := []string{
		// vvv do NOT reorder these! vvv
		"kill",
		"detach",
		"rename",
		"new",
		"switch",
		// ^^^ do NOT reorder these! ^^^
	}
	return menuItemsFromIDs(items), nil
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
		if err := tmux.SwitchClient(ctx.SocketPath, item.ID); err != nil {
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

func SessionCreateCommand(ctx Context, name string) tea.Cmd {
	return func() tea.Msg {
		events.Session.Create(name)
		if err := tmux.NewSession(ctx.SocketPath, name); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Created session %s", name)}
	}
}

func SessionRenameCommand(ctx Context, target, name string) tea.Cmd {
	return func() tea.Msg {
		if target == "" {
			return ActionResult{Err: fmt.Errorf("session target required")}
		}
		if name == "" {
			return ActionResult{Err: fmt.Errorf("session name required")}
		}
		events.Session.Rename(target, name)
		if err := tmux.RenameSession(ctx.SocketPath, target, name); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Renamed %s to %s", target, name)}
	}
}

func SessionCommandForAction(actionID string, ctx Context, target, name string) tea.Cmd {
	switch actionID {
	case "session:rename":
		return SessionRenameCommand(ctx, target, name)
	default:
		return SessionCreateCommand(ctx, name)
	}
}

type SessionForm struct {
	input    textinput.Model
	existing map[string]struct{}
	ctx      Context
	err      string
	mode     sessionFormMode
	target   string
	action   string
	title    string
	help     string
}

type sessionFormMode int

const (
	sessionFormModeCreate sessionFormMode = iota
	sessionFormModeRename
)

func NewSessionForm(prompt SessionPrompt) *SessionForm {
	ti := textinput.New()
	ti.Placeholder = "session-name"
	ti.CharLimit = 64
	ti.Focus()
	if prompt.Initial != "" {
		ti.SetValue(prompt.Initial)
	}
	mode := sessionFormModeCreate
	title := "Create Session"
	help := "Press Enter to create. Esc to cancel."
	target := strings.TrimSpace(prompt.Target)
	switch prompt.Action {
	case "session:rename":
		mode = sessionFormModeRename
		if target != "" {
			title = fmt.Sprintf("Rename %s", target)
		} else {
			title = "Rename Session"
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
	}
	form.SetSessions(prompt.Context.Sessions)
	form.validate()
	return form
}

func (f *SessionForm) Context() Context  { return f.ctx }
func (f *SessionForm) Value() string     { return strings.TrimSpace(f.input.Value()) }
func (f *SessionForm) InputView() string { return f.input.View() }
func (f *SessionForm) Error() string     { return f.err }
func (f *SessionForm) Action() string    { return f.action }
func (f *SessionForm) Target() string    { return f.target }
func (f *SessionForm) Title() string     { return f.title }
func (f *SessionForm) Help() string      { return f.help }
func (f *SessionForm) IsRename() bool    { return f.mode == sessionFormModeRename }

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

func (f *SessionForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "ctrl+u":
			if f.input.Value() != "" {
				f.input.SetValue("")
				f.input.CursorStart()
				f.err = f.validate()
			}
			return nil, false, false
		}
		switch m.Type {
		case tea.KeyEsc:
			if f.mode == sessionFormModeRename {
				events.Session.CancelRename(f.target, events.SessionReasonEscape)
			} else {
				events.Session.CancelNew(events.SessionReasonEscape)
			}
			return nil, false, true
		case tea.KeyEnter:
			value := f.Value()
			switch f.mode {
			case sessionFormModeCreate:
				if err := f.validateName(value); err != "" {
					f.err = err
					return nil, false, false
				}
				f.err = ""
				events.Session.SubmitNew(value)
				return SessionCreateCommand(f.ctx, value), true, false
			case sessionFormModeRename:
				if value == "" {
					events.Session.CancelRename(f.target, events.SessionReasonEmpty)
					return nil, false, true
				}
				if err := f.validateName(value); err != "" {
					f.err = err
					return nil, false, false
				}
				f.err = ""
				events.Session.SubmitRename(f.target, value)
				return SessionRenameCommand(f.ctx, f.target, value), true, false
			}
		}
	}

	updated, cmd := f.input.Update(msg)
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
	f.err = f.validate()
}

func (f *SessionForm) validate() string {
	return f.validateName(f.Value())
}

func (f *SessionForm) validateName(name string) string {
	trimmed := strings.TrimSpace(name)
	lower := strings.ToLower(trimmed)
	switch f.mode {
	case sessionFormModeRename:
		if trimmed == "" {
			return ""
		}
		if _, exists := f.existing[lower]; exists {
			return "Session already exists"
		}
		return ""
	default:
		if trimmed == "" {
			return "Session name required"
		}
		if _, exists := f.existing[lower]; exists {
			return "Session already exists"
		}
		return ""
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
