package ui

import (
	"reflect"
	"strings"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/data/dispatcher"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/state"
	"github.com/atomicstack/tmux-popup-control/internal/theme"
	"github.com/atomicstack/tmux-popup-control/internal/ui/command"
	uistate "github.com/atomicstack/tmux-popup-control/internal/ui/state"
	"github.com/charmbracelet/bubbles/cursor"
	tea "github.com/charmbracelet/bubbletea"
)

type level = uistate.Level

type Mode int

const (
	ModeMenu Mode = iota
	ModePaneForm
	ModeWindowForm
	ModeSessionForm
)

const (
	menuHeaderSeparator = "→"
	defaultRootTitle    = "main menu"
)

var styles = theme.Default()

var headerSegmentCleaner = strings.NewReplacer("_", " ", "-", " ")

type msgHandler func(tea.Msg) tea.Cmd

func newLevel(id, title string, items []menu.Item, node *menu.Node) *level {
	return uistate.NewLevel(id, title, items, node)
}

// Model implements the Bubble Tea model for the tmux popup menu.
type Model struct {
	stack             []*level
	loading           bool
	pendingID         string
	pendingLabel      string
	errMsg            string
	infoMsg           string
	infoExpire        time.Time
	width             int
	height            int
	fixedWidth        bool
	fixedHeight       bool
	backend           *backend.Watcher
	backendState      map[backend.Kind]error
	backendLastErr    string
	showFooter        bool
	verbose           bool
	sessionForm       *menu.SessionForm
	windowForm        *menu.WindowRenameForm
	paneForm          *menu.PaneRenameForm
	pendingWindowSwap *menu.Item
	pendingPaneSwap   *menu.Item
	filterCursor      cursor.Model
	filterCursorDirty bool

	handlers map[reflect.Type]msgHandler

	registry   *menu.Registry
	bus        *command.Bus
	mode       Mode
	rootMenuID string
	rootTitle  string
	socketPath string
	sessions   state.SessionStore
	windows    state.WindowStore
	panes      state.PaneStore
	dispatcher *dispatcher.Dispatcher
}

// NewModel initialises the UI state with the root menu and configuration.
func NewModel(socketPath string, width, height int, showFooter bool, verbose bool, watcher *backend.Watcher, rootMenu string) *Model {
	registry := menu.BuildRegistry()
	sessions := state.NewSessionStore()
	sessions.SetIncludeCurrent(true)
	windows := state.NewWindowStore()
	windows.SetIncludeCurrent(true)
	panes := state.NewPaneStore()
	panes.SetIncludeCurrent(true)
	rootItems := menu.RootItems()
	root := newLevel("root", "Main Menu", rootItems, registry.Root())
	m := &Model{
		stack:        []*level{root},
		registry:     registry,
		bus:          command.New(),
		backend:      watcher,
		backendState: map[backend.Kind]error{},
		showFooter:   showFooter,
		verbose:      verbose,
		mode:         ModeMenu,
		rootTitle:    defaultRootTitle,
		socketPath:   socketPath,
		sessions:     sessions,
		windows:      windows,
		panes:        panes,
		dispatcher:   dispatcher.New(sessions, windows, panes),
	}
	m.applyNodeSettings(root)
	m.syncViewport(root)
	if width > 0 {
		m.width = width
		m.fixedWidth = true
	}
	if height > 0 {
		m.height = height
		m.fixedHeight = true
	}
	c := cursor.New()
	if styles.Cursor != nil {
		c.Style = styles.Cursor.Copy()
	}
	if styles.Filter != nil {
		c.TextStyle = styles.Filter.Copy()
	}
	c.SetChar(" ")
	m.filterCursor = c
	m.applyRootMenuOverride(rootMenu)
	m.registerHandlers()
	return m
}

// Init is part of the tea.Model interface.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.backend != nil {
		cmds = append(cmds, waitForBackendEvent(m.backend))
	}
	if cmd := m.filterCursor.Focus(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Update responds to Bubble Tea messages.

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, 0, 4)
	if cmd := m.updateFilterCursorModel(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if handled, cmd := m.handleActiveForm(msg); handled {
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, m.finishUpdate(cmds)
	}

	if handler := m.handlerFor(msg); handler != nil {
		if cmd := handler(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, m.finishUpdate(cmds)
	}

	return m, m.finishUpdate(cmds)
}

func (m *Model) handleActiveForm(msg tea.Msg) (bool, tea.Cmd) {
	switch m.mode {
	case ModePaneForm:
		return m.handlePaneForm(msg)
	case ModeWindowForm:
		return m.handleWindowForm(msg)
	case ModeSessionForm:
		return m.handleSessionForm(msg)
	default:
		return false, nil
	}
}

func (m *Model) registerHandlers() {
	m.handlers = map[reflect.Type]msgHandler{
		reflect.TypeOf(tea.KeyMsg{}):            m.handleKeyMsg,
		reflect.TypeOf(tea.WindowSizeMsg{}):     m.handleWindowSizeMsg,
		reflect.TypeOf(categoryLoadedMsg{}):     m.handleCategoryLoadedMsg,
		reflect.TypeOf(menu.ActionResult{}):     m.handleActionResultMsg,
		reflect.TypeOf(menu.WindowPrompt{}):     m.handleWindowPromptMsg,
		reflect.TypeOf(menu.PanePrompt{}):       m.handlePanePromptMsg,
		reflect.TypeOf(menu.WindowSwapPrompt{}): m.handleWindowSwapPromptMsg,
		reflect.TypeOf(menu.PaneSwapPrompt{}):   m.handlePaneSwapPromptMsg,
		reflect.TypeOf(menu.SessionPrompt{}):    m.handleSessionPromptMsg,
		reflect.TypeOf(backendEventMsg{}):       m.handleBackendEventMsg,
		reflect.TypeOf(backendDoneMsg{}):        m.handleBackendDoneMsg,
		reflect.TypeOf(menu.CommandPromptMsg{}): m.handleCommandPromptMsg,
	}
}

func (m *Model) handlerFor(msg tea.Msg) msgHandler {
	if msg == nil || m.handlers == nil {
		return nil
	}
	t := reflect.TypeOf(msg)
	if handler, ok := m.handlers[t]; ok {
		return handler
	}
	if t.Kind() == reflect.Ptr {
		if handler, ok := m.handlers[t.Elem()]; ok {
			return handler
		}
	}
	return nil
}

func (m *Model) finishUpdate(cmds []tea.Cmd) tea.Cmd {
	if m.filterCursorDirty {
		m.filterCursorDirty = false
		m.filterCursor.Blink = false
		if cmd := m.filterCursor.BlinkCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
