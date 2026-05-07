package ui

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// currentPaneIDWithFallback returns storeID if non-empty, otherwise reads
// TMUX_POPUP_CONTROL_PANE_ID (set by main.sh) so pane actions work even
// before the backend's first poll completes.
func currentPaneIDWithFallback(storeID string) string {
	if storeID != "" {
		return storeID
	}
	return strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_ID"))
}

func (m *Model) handleActionResultMsg(msg tea.Msg) tea.Cmd {
	result, ok := msg.(menu.ActionResult)
	if !ok {
		return nil
	}
	if m.pendingID == deleteSavedLevelID {
		return m.handleDeleteSavedActionResult(result)
	}
	title := m.pendingLabel
	m.loading = false
	m.pendingID = ""
	m.pendingLabel = ""
	if result.Err != nil {
		m.errMsg = result.Err.Error()
		m.forceClearInfo()
		events.Action.Error(result.Err)
		return nil
	}
	if strings.TrimSpace(result.Output) != "" {
		m.showCommandOutput(title, result.Output)
		events.Action.Success(result.Info)
		return nil
	}
	if result.Info != "" && m.verbose {
		m.setInfo(result.Info)
	} else {
		m.forceClearInfo()
	}
	events.Action.Success(result.Info)
	return tea.Quit
}

func (m *Model) showCommandOutput(title, output string) {
	normalized := strings.ReplaceAll(output, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = strings.TrimRight(normalized, "\n")
	m.commandOutputTitle = strings.TrimSpace(title)
	if normalized == "" {
		m.commandOutputLines = []string{""}
	} else {
		m.commandOutputLines = strings.Split(normalized, "\n")
	}
	m.commandOutputOffset = 0
	m.mode = ModeCommandOutput
	m.errMsg = ""
	m.forceClearInfo()
}

func (m *Model) clearCommandOutput() {
	m.commandOutputTitle = ""
	m.commandOutputLines = nil
	m.commandOutputOffset = 0
	m.mode = ModeMenu
}

func (m *Model) handleCommandOutputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		m.clearCommandOutput()
	case "up":
		m.scrollCommandOutput(-1)
	case "down":
		m.scrollCommandOutput(1)
	case "pgup":
		m.scrollCommandOutput(-m.commandOutputPageSize())
	case "pgdown":
		m.scrollCommandOutput(m.commandOutputPageSize())
	case "home":
		m.commandOutputOffset = 0
	case "end":
		m.commandOutputOffset = m.maxCommandOutputOffset()
	}
	return nil
}

func (m *Model) commandOutputPageSize() int {
	rows := m.height - 2
	if m.commandOutputTitle != "" {
		rows--
	}
	if rows < 1 {
		return 1
	}
	return rows
}

func (m *Model) maxCommandOutputOffset() int {
	maxOffset := len(m.commandOutputLines) - m.commandOutputPageSize()
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func (m *Model) scrollCommandOutput(delta int) {
	offset := m.commandOutputOffset + delta
	offset = max(offset, 0)
	if maxOffset := m.maxCommandOutputOffset(); offset > maxOffset {
		offset = maxOffset
	}
	m.commandOutputOffset = offset
}

// commandPreloadMsg delivers the pre-fetched command list items.
type commandPreloadMsg struct {
	items []menu.Item
	err   error
}

// userOptionsPreloadMsg delivers the pre-fetched set of live @-prefixed
// tmux option names so completion can merge them into catalog candidates.
type userOptionsPreloadMsg struct {
	names []string
	err   error
}

// preloadCommandList fires an async command to fetch the command list.
func preloadCommandList(socketPath string, loader menu.Loader) tea.Cmd {
	return func() tea.Msg {
		span := logging.StartSpan("ui", "load.command_menu", logging.SpanOptions{
			Target: "command",
			Attrs: map[string]any{
				"socket_path": socketPath,
			},
		})
		items, err := loader(menu.Context{SocketPath: socketPath})
		span.AddAttr("item_count", len(items))
		span.End(err)
		return commandPreloadMsg{items: items, err: err}
	}
}

// preloadUserOptions fires an async command to fetch the set of live
// @-prefixed tmux option names.
func preloadUserOptions(socketPath string) tea.Cmd {
	return func() tea.Msg {
		names, err := menu.LoadUserOptions(menu.Context{SocketPath: socketPath})
		return userOptionsPreloadMsg{names: names, err: err}
	}
}

func (m *Model) handleCommandPreloadMsg(msg tea.Msg) tea.Cmd {
	preload, ok := msg.(commandPreloadMsg)
	if !ok {
		return nil
	}
	if preload.err != nil {
		logging.Error(preload.err)
		return nil
	}
	m.commandItemsCache = preload.items
	labels := make([]string, 0, len(preload.items))
	for _, item := range preload.items {
		labels = append(labels, item.Label)
	}
	m.commandSchemas = cmdparse.BuildRegistry(labels)
	return nil
}

func (m *Model) handleUserOptionsPreloadMsg(msg tea.Msg) tea.Cmd {
	preload, ok := msg.(userOptionsPreloadMsg)
	if !ok {
		return nil
	}
	if preload.err != nil {
		// Non-fatal: the catalog still covers the common case. Log and
		// leave m.userOptionNames empty.
		logging.Error(preload.err)
		return nil
	}
	m.userOptionNames = preload.names
	return nil
}

func (m *Model) loadMenuCmd(id, title string, loader menu.Loader) tea.Cmd {
	return func() tea.Msg {
		span := logging.StartSpan("ui", "load.menu", logging.SpanOptions{
			Target: id,
			Attrs: map[string]any{
				"title":       title,
				"socket_path": m.socketPath,
			},
		})
		items, err := loader(m.menuContext())
		if err != nil {
			logging.Error(err)
		}
		span.AddAttr("item_count", len(items))
		span.End(err)
		return categoryLoadedMsg{id: id, title: title, items: items, err: err}
	}
}

// categoryLoadedMsg mirrors the async loader response.
type categoryLoadedMsg struct {
	id    string
	title string
	items []menu.Item
	err   error
}

func (m *Model) menuContext() menu.Context {
	ctx := menu.Context{
		SocketPath:           m.socketPath,
		ClientID:             m.clientID,
		MenuArgs:             m.menuArgs,
		Sessions:             m.sessions.Entries(),
		Current:              m.sessions.Current(),
		IncludeCurrent:       m.sessions.IncludeCurrent(),
		Windows:              m.windows.Entries(),
		CurrentWindowID:      m.windows.CurrentID(),
		CurrentWindowLabel:   m.windows.CurrentLabel(),
		CurrentWindowSession: m.windows.CurrentSession(),
		WindowIncludeCurrent: m.windows.IncludeCurrent(),
		Panes:                m.panes.Entries(),
		CurrentPaneID:        currentPaneIDWithFallback(m.panes.CurrentID()),
		CurrentPaneLabel:     m.panes.CurrentLabel(),
		PaneIncludeCurrent:   m.panes.IncludeCurrent(),
	}
	for _, w := range ctx.Windows {
		if w.Current {
			ctx.CurrentWindowLayout = w.Layout
			break
		}
	}
	return ctx
}
