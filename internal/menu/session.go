package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

func loadSessionMenu(Context) ([]Item, error) {
	items := []string{
		"switch",
		"new",
		"rename",
		"detach",
		"kill",
	}
	return menuItemsFromIDs(items), nil
}

func loadSessionSwitchMenu(ctx Context) ([]Item, error) {
	items := make([]Item, 0, len(ctx.Sessions))
	for _, sess := range ctx.Sessions {
		if !ctx.IncludeCurrent && sess.Current {
			continue
		}
		items = append(items, Item{ID: sess.Name, Label: sess.Label})
	}
	return items, nil
}

func loadSessionRenameMenu(ctx Context) ([]Item, error) {
	return SessionEntriesToItems(ctx.Sessions), nil
}

func loadSessionDetachMenu(ctx Context) ([]Item, error) {
	return SessionEntriesToItems(ctx.Sessions), nil
}

func loadSessionKillMenu(ctx Context) ([]Item, error) {
	return SessionEntriesToItems(ctx.Sessions), nil
}

func SessionNewAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		logging.Trace("session.new.prompt", map[string]interface{}{"existing": len(ctx.Sessions)})
		return SessionPrompt{Context: ctx, Action: "session:new"}
	}
}

func SessionSwitchAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		logging.Trace("session.switch", map[string]interface{}{"target": item.ID})
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
		logging.Trace("session.rename.prompt", map[string]interface{}{"target": target})
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
		logging.Trace("session.detach", map[string]interface{}{"target": target})
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
		logging.Trace("session.kill", map[string]interface{}{"target": target})
		if err := tmux.KillSessions(ctx.SocketPath, []string{target}); err != nil {
			return ActionResult{Err: err}
		}
		return ActionResult{Info: fmt.Sprintf("Killed %s", label)}
	}
}

func SessionCreateCommand(ctx Context, name string) tea.Cmd {
	return func() tea.Msg {
		logging.Trace("session.new.create", map[string]interface{}{"name": name})
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
		logging.Trace("session.rename", map[string]interface{}{"target": target, "name": name})
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
		switch m.Type {
		case tea.KeyEsc:
			if f.mode == sessionFormModeRename {
				logging.Trace("session.rename.cancel", map[string]interface{}{"target": f.target, "reason": "escape"})
			} else {
				logging.Trace("session.new.cancel", nil)
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
				logging.Trace("session.new.submit", map[string]interface{}{"name": value})
				return SessionCreateCommand(f.ctx, value), true, false
			case sessionFormModeRename:
				if value == "" {
					logging.Trace("session.rename.cancel", map[string]interface{}{"target": f.target, "reason": "empty"})
					return nil, false, true
				}
				if err := f.validateName(value); err != "" {
					f.err = err
					return nil, false, false
				}
				f.err = ""
				logging.Trace("session.rename.submit", map[string]interface{}{"target": f.target, "name": value})
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
		}
		entries = append(entries, entry)
	}
	return entries
}

func SessionEntriesToItems(entries []SessionEntry) []Item {
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		items = append(items, Item{ID: entry.Name, Label: entry.Label})
	}
	return items
}
