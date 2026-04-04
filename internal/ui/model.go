package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"charm.land/bubbles/v2/cursor"
	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/cmdhelp"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/data/dispatcher"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/state"
	"github.com/atomicstack/tmux-popup-control/internal/theme"
	"github.com/atomicstack/tmux-popup-control/internal/ui/command"
	uistate "github.com/atomicstack/tmux-popup-control/internal/ui/state"
)

type level = uistate.Level

type Mode int

const (
	ModeMenu Mode = iota
	ModePaneForm
	ModeWindowForm
	ModeSessionForm
	ModePluginConfirm
	ModePluginInstall
	ModeResurrect
	ModeSessionSaveForm
	ModePaneCaptureForm
)

const menuHeaderSeparator = "→"

var defaultRootTitle = filepath.Base(os.Args[0])

var styles = theme.Default()

var headerSegmentCleaner = strings.NewReplacer("_", " ", "-", " ")

func (m Mode) String() string {
	switch m {
	case ModeMenu:
		return "menu"
	case ModePaneForm:
		return "pane_form"
	case ModeWindowForm:
		return "window_form"
	case ModeSessionForm:
		return "session_form"
	case ModePluginConfirm:
		return "plugin_confirm"
	case ModePluginInstall:
		return "plugin_install"
	case ModeResurrect:
		return "resurrect"
	case ModeSessionSaveForm:
		return "session_save_form"
	case ModePaneCaptureForm:
		return "pane_capture_form"
	default:
		return "unknown"
	}
}

type msgHandler func(tea.Msg) tea.Cmd

func newLevel(id, title string, items []menu.Item, node *menu.Node) *level {
	return uistate.NewLevel(id, title, items, node)
}

// Model implements the Bubble Tea model for the tmux popup menu.
type Model struct {
	stack                      []*level
	loading                    bool
	pendingID                  string
	pendingLabel               string
	errMsg                     string
	infoMsg                    string
	infoExpire                 time.Time
	width                      int
	height                     int
	fixedWidth                 bool
	fixedHeight                bool
	backend                    *backend.Watcher
	backendState               map[backend.Kind]error
	backendLastErr             string
	showFooter                 bool
	verbose                    bool
	sessionForm                *menu.SessionForm
	windowForm                 *menu.WindowRenameForm
	paneForm                   *menu.PaneRenameForm
	saveForm                   *menu.SaveForm
	paneCaptureForm            *menu.PaneCaptureForm
	pendingWindowSwap          *menu.Item
	pendingPaneSwap            *menu.Item
	commandItemsCache          []menu.Item
	commandSchemas             map[string]*cmdparse.CommandSchema
	commandHelp                map[string]cmdhelp.CommandHelp
	completion                 *completionState
	completionSuppressedFilter string
	noPreview                  bool
	filterCursor               cursor.Model
	filterCursorDirty          bool

	handlers map[reflect.Type]msgHandler

	registry           *menu.Registry
	bus                *command.Bus
	mode               Mode
	rootMenuID         string
	menuArgs           string
	rootTitle          string
	socketPath         string
	clientID           string
	sessionName        string
	sessions           state.SessionStore
	windows            state.WindowStore
	panes              state.PaneStore
	dispatcher         *dispatcher.Dispatcher
	preview            map[string]*previewData
	previewSeq         int
	treeSessions       []menu.SessionEntry
	treeWindows        []menu.WindowEntry
	treePanes          []menu.PaneEntry
	pullTreeSessions   []menu.SessionEntry
	pullTreeWindows    []menu.WindowEntry
	pluginConfirmState *pluginConfirmState
	pluginInstallState *pluginInstallState
	resurrectState     *resurrectState
	initCmd            tea.Cmd
	deferredAction     *menu.Node
	deferredRename     *menu.Node
}

// ModelConfig holds parameters for NewModel.
type ModelConfig struct {
	SocketPath  string
	Width       int
	Height      int
	ShowFooter  bool
	Verbose     bool
	NoPreview   bool
	Watcher     *backend.Watcher
	RootMenu    string
	MenuArgs    string
	ClientID    string
	SessionName string
}

// NewModel initialises the UI state with the root menu and configuration.
func NewModel(cfg ModelConfig) *Model {
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
		backend:      cfg.Watcher,
		backendState: map[backend.Kind]error{},
		showFooter:   cfg.ShowFooter,
		verbose:      cfg.Verbose,
		noPreview:    cfg.NoPreview,
		mode:         ModeMenu,
		rootTitle:    defaultRootTitle,
		menuArgs:     cfg.MenuArgs,
		socketPath:   cfg.SocketPath,
		clientID:     cfg.ClientID,
		sessionName:  cfg.SessionName,
		sessions:     sessions,
		windows:      windows,
		panes:        panes,
		dispatcher:   dispatcher.New(sessions, windows, panes),
		preview:      make(map[string]*previewData),
		commandHelp:  cmdhelp.Commands,
	}
	m.applyNodeSettings(root)
	m.syncViewport(root)
	if cfg.Width > 0 {
		m.width = cfg.Width
		m.fixedWidth = true
	}
	if cfg.Height > 0 {
		m.height = cfg.Height
		m.fixedHeight = true
	}
	c := cursor.New()
	if styles.Cursor != nil {
		c.Style = *styles.Cursor
	}
	if styles.Filter != nil {
		c.TextStyle = *styles.Filter
	}
	c.SetChar(" ")
	m.filterCursor = c
	m.applyRootMenuOverride(cfg.RootMenu)
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
	if m.initCmd != nil {
		cmds = append(cmds, m.initCmd)
		m.initCmd = nil
	}
	cmds = append(cmds, previewTick())
	if m.commandItemsCache == nil {
		if node, ok := m.registry.Find("command"); ok && node.Loader != nil {
			cmds = append(cmds, preloadCommandList(m.socketPath, node.Loader))
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Update responds to Bubble Tea messages.

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	msgType := "<nil>"
	if t := reflect.TypeOf(msg); t != nil {
		msgType = t.String()
	}
	span := logging.StartSpan("ui", "update", logging.SpanOptions{
		Target: msgType,
		Attrs: map[string]interface{}{
			"mode":        m.mode.String(),
			"stack_depth": len(m.stack),
			"loading":     m.loading,
		},
	})
	defer span.End(nil)

	cmds := make([]tea.Cmd, 0, 4)
	if cmd := m.updateFilterCursorModel(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if handled, cmd := m.handleActiveForm(msg); handled {
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		span.AddAttr("cmd_count", len(cmds))
		return m, m.finishUpdate(cmds)
	}

	if handler := m.handlerFor(msg); handler != nil {
		if cmd := handler(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		span.AddAttr("cmd_count", len(cmds))
		return m, m.finishUpdate(cmds)
	}

	span.AddAttr("cmd_count", len(cmds))
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
	case ModePluginConfirm:
		return m.handlePluginConfirm(msg)
	case ModePluginInstall:
		return m.handlePluginInstallKey(msg)
	case ModeResurrect:
		return m.handleResurrectKey(msg)
	case ModeSessionSaveForm:
		return m.handleSaveForm(msg)
	case ModePaneCaptureForm:
		return m.handlePaneCaptureForm(msg)
	default:
		return false, nil
	}
}

func (m *Model) registerHandlers() {
	m.handlers = map[reflect.Type]msgHandler{
		reflect.TypeOf(tea.KeyPressMsg{}):            m.handleKeyMsg,
		reflect.TypeOf(tea.WindowSizeMsg{}):          m.handleWindowSizeMsg,
		reflect.TypeOf(categoryLoadedMsg{}):          m.handleCategoryLoadedMsg,
		reflect.TypeOf(menu.ActionResult{}):          m.handleActionResultMsg,
		reflect.TypeOf(menu.WindowPrompt{}):          m.handleWindowPromptMsg,
		reflect.TypeOf(menu.PanePrompt{}):            m.handlePanePromptMsg,
		reflect.TypeOf(menu.WindowSwapPrompt{}):      m.handleWindowSwapPromptMsg,
		reflect.TypeOf(menu.PaneSwapPrompt{}):        m.handlePaneSwapPromptMsg,
		reflect.TypeOf(menu.SessionPrompt{}):         m.handleSessionPromptMsg,
		reflect.TypeOf(backendEventMsg{}):            m.handleBackendEventMsg,
		reflect.TypeOf(backendDoneMsg{}):             m.handleBackendDoneMsg,
		reflect.TypeOf(commandPreloadMsg{}):          m.handleCommandPreloadMsg,
		reflect.TypeOf(previewTickMsg{}):             m.handlePreviewTickMsg,
		reflect.TypeOf(previewLoadedMsg{}):           m.handlePreviewLoadedMsg,
		reflect.TypeOf(layoutAppliedMsg{}):           m.handleLayoutAppliedMsg,
		reflect.TypeOf(tea.MouseWheelMsg{}):          m.handleMouseMsg,
		reflect.TypeOf(menu.PluginConfirmPrompt{}):   m.handlePluginConfirmPromptMsg,
		reflect.TypeOf(pluginRemovalDoneMsg{}):       m.handlePluginRemovalDoneMsg,
		reflect.TypeOf(menu.PluginInstallStart{}):    m.handlePluginInstallStartMsg,
		reflect.TypeOf(menu.PluginUpdateStart{}):     m.handlePluginUpdateStartMsg,
		reflect.TypeOf(pluginInstallDoneMsg{}):       m.handlePluginInstallDoneMsg,
		reflect.TypeOf(menu.ResurrectStart{}):        m.handleResurrectStartMsg,
		reflect.TypeOf(resurrectProgressMsg{}):       m.handleResurrectProgressMsg,
		reflect.TypeOf(resurrectTickMsg{}):           m.handleResurrectTickMsg,
		reflect.TypeOf(menu.SaveAsPrompt{}):          m.handleSaveAsPromptMsg,
		reflect.TypeOf(menu.PaneCapturePrompt{}):     m.handlePaneCapturePromptMsg,
		reflect.TypeOf(menu.PaneCapturePreviewMsg{}): m.handlePaneCapturePreviewMsg,
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
		if cmd := m.filterCursor.Blink(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}
