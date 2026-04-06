package ui

import (
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
)

const restoreRefreshInterval = 2 * time.Second

type restoreRefreshState struct {
	dir         string
	lastModTime time.Time
}

type restoreRefreshTickMsg struct{}

type restoreRefreshLoadedMsg struct {
	dir      string
	subtitle string
	modTime  time.Time
	items    []menu.Item
	err      error
}

var restoreRefreshScheduleFn = func() tea.Cmd {
	return tea.Tick(restoreRefreshInterval, func(time.Time) tea.Msg {
		return restoreRefreshTickMsg{}
	})
}

var restoreRefreshStatFn = func(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

func (m *Model) startRestoreRefreshIfNeeded() tea.Cmd {
	current := m.currentLevel()
	if current == nil || current.ID != "session:restore-from" {
		return nil
	}
	dir := strings.TrimSpace(current.Subtitle)
	if dir == "" {
		return nil
	}
	modTime, err := restoreRefreshStatFn(dir)
	if err != nil {
		modTime = time.Time{}
	}
	m.restoreRefresh = &restoreRefreshState{
		dir:         dir,
		lastModTime: modTime,
	}
	return restoreRefreshScheduleFn()
}

func (m *Model) stopRestoreRefresh() {
	m.restoreRefresh = nil
}

func (m *Model) shouldRefreshRestoreList() bool {
	current := m.currentLevel()
	return current != nil &&
		current.ID == "session:restore-from" &&
		m.restoreRefresh != nil &&
		strings.TrimSpace(m.restoreRefresh.dir) != ""
}

func (m *Model) handleRestoreRefreshTickMsg(msg tea.Msg) tea.Cmd {
	if !m.shouldRefreshRestoreList() {
		m.stopRestoreRefresh()
		return nil
	}

	modTime, err := restoreRefreshStatFn(m.restoreRefresh.dir)
	if err != nil {
		return restoreRefreshScheduleFn()
	}
	if modTime.Equal(m.restoreRefresh.lastModTime) {
		return restoreRefreshScheduleFn()
	}
	return m.loadRestoreRefreshCmd(m.restoreRefresh.dir, modTime)
}

func (m *Model) loadRestoreRefreshCmd(dir string, modTime time.Time) tea.Cmd {
	current := m.currentLevel()
	if current == nil {
		return nil
	}
	loader := current.Node
	if loader == nil || loader.Loader == nil {
		if node, ok := m.registry.Find("session:restore-from"); ok {
			loader = node
		}
	}
	if loader == nil || loader.Loader == nil {
		return nil
	}

	return func() tea.Msg {
		items, err := loader.Loader(m.menuContext())
		return restoreRefreshLoadedMsg{
			dir:      dir,
			subtitle: dir,
			modTime:  modTime,
			items:    items,
			err:      err,
		}
	}
}

func (m *Model) handleRestoreRefreshLoadedMsg(msg tea.Msg) tea.Cmd {
	update, ok := msg.(restoreRefreshLoadedMsg)
	if !ok || m.restoreRefresh == nil {
		return nil
	}
	m.restoreRefresh.lastModTime = update.modTime
	if !m.shouldRefreshRestoreList() {
		m.stopRestoreRefresh()
		return nil
	}
	if update.err != nil {
		return restoreRefreshScheduleFn()
	}

	current := m.currentLevel()
	if current == nil || current.ID != "session:restore-from" {
		m.stopRestoreRefresh()
		return nil
	}

	previousID := ""
	if current.Cursor >= 0 && current.Cursor < len(current.Items) && !current.Items[current.Cursor].Header {
		previousID = current.Items[current.Cursor].ID
	}

	current.Subtitle = update.subtitle
	current.UpdateItems(update.items)
	if previousID != "" {
		if idx := current.IndexOf(previousID); idx >= 0 {
			current.Cursor = idx
		}
	}
	if strings.TrimSpace(current.Filter) != "" {
		m.syncFilterViewport(current)
	} else {
		m.syncViewport(current)
	}
	return restoreRefreshScheduleFn()
}

func restoreRefreshDir(socketPath string) string {
	dir, err := resurrect.ResolveDir(socketPath)
	if err != nil {
		return ""
	}
	return dir
}
