package ui

import (
	"context"
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
	"github.com/atomicstack/tmux-popup-control/internal/extract"
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
	ModePluginInstall
	ModeResurrect
	ModeSessionSaveForm
	ModePaneCaptureForm
	ModeCommandOutput
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
	case ModePluginInstall:
		return "plugin_install"
	case ModeResurrect:
		return "resurrect"
	case ModeSessionSaveForm:
		return "session_save_form"
	case ModePaneCaptureForm:
		return "pane_capture_form"
	case ModeCommandOutput:
		return "command_output"
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
	ctx                        context.Context
	cancel                     context.CancelFunc
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
	layoutMutations            *layoutMutations
	pendingWindowSwap          *menu.Item
	pendingPaneSwap            *menu.Item
	commandItemsCache          []menu.Item
	commandSchemas             map[string]*cmdparse.CommandSchema
	commandHelp                map[string]cmdhelp.CommandHelp
	userOptionNames            []string
	completion                 *completionState
	completionSuppressedFilter string
	noPreview                  bool
	previewBlink               cursor.Model
	previewBlinkDirty          bool
	commandOutputTitle         string
	commandOutputLines         []string
	commandOutputOffset        int
	extractCategory            extract.Category
	extractGrabArea            extract.GrabArea
	extractSeq                 int
	extractModePopup           *completionState
	extractModePrePopup        extract.Category
	extractModeSeq             int
	extractAreaPopup           *completionState
	extractAreaPrePopup        extract.GrabArea

	registry           *menu.Registry
	bus                *command.Bus
	mode               Mode
	rootMenuID         string
	menuArgs           string
	rootTitle          string
	socketPath         string
	clientID           string
	sessionName        string
	sessions           *state.SessionStore
	windows            *state.WindowStore
	panes              *state.PaneStore
	dispatcher         *dispatcher.Dispatcher
	preview            map[string]*previewData
	previewSeq         int
	treeSessions       []menu.SessionEntry
	treeWindows        []menu.WindowEntry
	treePanes          []menu.PaneEntry
	pullTreeSessions   []menu.SessionEntry
	pullTreeWindows    []menu.WindowEntry
	pluginInstallState *pluginInstallState
	resurrectState     *resurrectState
	restoreRefresh     *restoreRefreshState
	initCmd            tea.Cmd
	deferredAction     *menu.Node
	deferredRename     *menu.Node

	confirmState              *deleteConfirmState
	pendingDeleteFilter       string
	pendingDeleteFilterCursor int
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
	ctx, cancel := context.WithCancel(context.Background())
	m := &Model{
		ctx: ctx, cancel: cancel,
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
		commandHelp:  cmdhelp.Commands(),
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
	m.previewBlink = cursor.New()
	m.applyRootMenuOverride(cfg.RootMenu)
	return m
}

// Init is part of the tea.Model interface.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{}
	if m.backend != nil {
		cmds = append(cmds, waitForBackendEvent(m.backend))
	}
	if cmd := m.previewBlink.Focus(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if m.initCmd != nil {
		cmds = append(cmds, m.initCmd)
		m.initCmd = nil
	}
	if !m.noPreview {
		cmds = append(cmds, previewTick())
	}
	if m.commandItemsCache == nil {
		if node, ok := m.registry.Find("command"); ok && node.Loader != nil {
			cmds = append(cmds, preloadCommandList(m.socketPath, node.Loader))
		}
	}
	if m.userOptionNames == nil {
		cmds = append(cmds, preloadUserOptions(m.socketPath))
	}
	if cmd := m.startRestoreRefreshIfNeeded(); cmd != nil {
		cmds = append(cmds, cmd)
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
		Attrs: map[string]any{
			"mode":        m.mode.String(),
			"stack_depth": len(m.stack),
			"loading":     m.loading,
		},
	})
	defer span.End(nil)

	cmds := make([]tea.Cmd, 0, 4)
	if cmd := m.updatePreviewBlinkModel(msg); cmd != nil {
		cmds = append(cmds, cmd)
	}
	handler := m.handlerFor(msg)
	_, isKey := msg.(tea.KeyPressMsg)
	_, isMouse := msg.(tea.MouseWheelMsg)
	if handler == nil || isKey || isMouse {
		if handled, cmd := m.handleActiveForm(msg); handled {
			cmds = append(cmds, cmd)
			return m, m.finishUpdate(cmds)
		}
	}
	if handler != nil {
		cmds = append(cmds, handler(msg))
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

func (m *Model) handlerFor(msg tea.Msg) msgHandler {
	switch msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKeyMsg
	case tea.WindowSizeMsg:
		return m.handleWindowSizeMsg
	case categoryLoadedMsg:
		return m.handleCategoryLoadedMsg
	case menu.ActionResult:
		return m.handleActionResultMsg
	case menu.WindowPrompt:
		return m.handleWindowPromptMsg
	case menu.PanePrompt:
		return m.handlePanePromptMsg
	case menu.WindowSwapPrompt:
		return m.handleWindowSwapPromptMsg
	case menu.PaneSwapPrompt:
		return m.handlePaneSwapPromptMsg
	case menu.SessionPrompt:
		return m.handleSessionPromptMsg
	case backendEventMsg:
		return m.handleBackendEventMsg
	case backendDoneMsg:
		return m.handleBackendDoneMsg
	case commandPreloadMsg:
		return m.handleCommandPreloadMsg
	case userOptionsPreloadMsg:
		return m.handleUserOptionsPreloadMsg
	case previewTickMsg:
		return m.handlePreviewTickMsg
	case previewLoadedMsg:
		return m.handlePreviewLoadedMsg
	case layoutAppliedMsg:
		return m.handleLayoutAppliedMsg
	case tea.MouseWheelMsg:
		return m.handleMouseMsg
	case menu.PluginConfirmPrompt:
		return m.handlePluginConfirmPromptMsg
	case menu.PluginInstallStart:
		return m.handlePluginInstallStartMsg
	case menu.PluginUpdateStart:
		return m.handlePluginUpdateStartMsg
	case pluginInstallStageMsg:
		return m.handlePluginInstallStageMsg
	case pluginInstallResultMsg:
		return m.handlePluginInstallResultMsg
	case menu.ResurrectStart:
		return m.handleResurrectStartMsg
	case resurrectProgressMsg:
		return m.handleResurrectProgressMsg
	case resurrectTickMsg:
		return m.handleResurrectTickMsg
	case resurrectAnimTickMsg:
		return m.handleResurrectAnimTickMsg
	case restoreRefreshTickMsg:
		return m.handleRestoreRefreshTickMsg
	case restoreRefreshLoadedMsg:
		return m.handleRestoreRefreshLoadedMsg
	case menu.SaveAsPrompt:
		return m.handleSaveAsPromptMsg
	case menu.PaneCapturePrompt:
		return m.handlePaneCapturePromptMsg
	case menu.PaneCapturePreviewMsg:
		return m.handlePaneCapturePreviewMsg
	case deleteSavedReloadedMsg:
		return m.handleDeleteSavedReloadedMsg
	case extractReloadMsg:
		return m.handleExtractReloadMsg
	case extractDoneMsg:
		return m.handleExtractDoneMsg
	case extractModeTimeoutMsg:
		return m.handleExtractModeTimeoutMsg
	}
	return nil
}

func (m *Model) finishUpdate(cmds []tea.Cmd) tea.Cmd {
	if m.previewBlinkDirty {
		m.previewBlinkDirty = false
		if cmd := m.previewBlink.Blink(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// Close cancels background contexts and discards queued layout mutations.
// An already executing tmux mutation may finish; shutdown never waits for it.
func (m *Model) Close() {
	if m.layoutMutations != nil {
		m.layoutMutations.close()
	}
	if m.cancel != nil {
		m.cancel()
	}
}
