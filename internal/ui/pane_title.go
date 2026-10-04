package ui

import (
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// paneTitleBase is the popup pane's title at the root menu; it matches the
// new-pane -T value main.sh and showPopup open the popup with.
const paneTitleBase = "tmux-popup-control"

// setPaneTitleFn retitles a pane (select-pane -T). Swappable in tests.
var setPaneTitleFn = tmux.RenamePane

// popupPaneID returns the popup's own pane when the binary was launched as a
// popup (main.sh records the host pane in TMUX_POPUP_CONTROL_PANE_ID), and ""
// otherwise, so running the binary in an ordinary pane never renames it.
func popupPaneID() string {
	if strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_PANE_ID")) == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv("TMUX_PANE"))
}

// paneTitle is the title the popup pane should carry for the current menu
// position: the app name, followed by the breadcrumb once there is one.
func (m *Model) paneTitle() string {
	if header := m.menuHeader(); header != "" {
		return paneTitleBase + ": " + header
	}
	return paneTitleBase
}

// syncPaneTitleCmd returns a command that retitles the popup pane when the
// breadcrumb changed since the last call, or nil when nothing needs doing.
func (m *Model) syncPaneTitleCmd() tea.Cmd {
	pane := m.titlePane
	if pane == "" {
		return nil
	}
	title := m.paneTitle()
	if title == m.lastPaneTitle {
		return nil
	}
	m.lastPaneTitle = title
	socket := m.socketPath
	return func() tea.Msg {
		if err := setPaneTitleFn(socket, pane, title); err != nil {
			logging.Error(err)
		}
		return nil
	}
}
