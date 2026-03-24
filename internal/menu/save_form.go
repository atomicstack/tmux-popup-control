package menu

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

// SaveForm handles the "save as" text input for naming a snapshot.
type SaveForm struct {
	input            textinput.Model
	saveDir          string
	ctx              Context
	err              string
	confirmOverwrite bool
}

// NewSaveForm creates a SaveForm from a SaveAsPrompt.
func NewSaveForm(prompt SaveAsPrompt) *SaveForm {
	ti := textinput.New()
	styleFormInput(&ti)
	ti.Placeholder = "snapshot-name"
	ti.CharLimit = 64
	ti.SetWidth(40)
	ti.Focus()
	return &SaveForm{
		input:   ti,
		saveDir: prompt.SaveDir,
		ctx:     prompt.Context,
	}
}

func (f *SaveForm) Context() Context  { return f.ctx }
func (f *SaveForm) Value() string     { return strings.TrimSpace(f.input.Value()) }
func (f *SaveForm) InputView() string { return f.input.View() }
func (f *SaveForm) FocusCmd() tea.Cmd  { return f.input.Focus() }
func (f *SaveForm) Error() string     { return f.err }
func (f *SaveForm) Title() string     { return "save as" }
func (f *SaveForm) Subtitle() string  { return f.saveDir }
func (f *SaveForm) Help() string      { return "press enter to save. esc to cancel." }
func (f *SaveForm) SaveDir() string   { return f.saveDir }

// Update processes a key message and returns (cmd, done, cancel).
func (f *SaveForm) Update(msg tea.Msg) (tea.Cmd, bool, bool) {
	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		updated, cmd := f.input.Update(msg)
		f.input = updated
		return cmd, false, false
	}

	switch kp.String() {
	case "esc":
		return nil, false, true

	case "enter":
		name := f.Value()
		if name == "" {
			f.err = "name cannot be empty"
			return nil, false, false
		}
		if err := resurrect.ValidateSaveName(name); err != nil {
			f.err = err.Error()
			return nil, false, false
		}
		if resurrect.SaveFileExists(f.saveDir, name) && !f.confirmOverwrite {
			f.err = fmt.Sprintf("snapshot %q already exists — enter to overwrite", name)
			f.confirmOverwrite = true
			return nil, false, false
		}
		return nil, true, false

	case "ctrl+u":
		f.confirmOverwrite = false
		f.err = ""
		f.input.SetValue("")
		f.input.CursorStart()
		return nil, false, false

	case "backspace":
		f.confirmOverwrite = false
		f.err = ""
		updated, cmd := f.input.Update(msg)
		f.input = updated
		return cmd, false, false

	default:
		// any other character resets overwrite confirmation
		f.confirmOverwrite = false
		f.err = ""
		updated, cmd := f.input.Update(msg)
		f.input = updated
		return cmd, false, false
	}
}
