package ui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/ui/command"
)

// deleteSavedLevelID is the registry id of the resurrect delete-saved menu.
const deleteSavedLevelID = "resurrect:delete-saved"

// deleteConfirmState carries the in-flight y/n confirmation prompt that
// replaces the filter input on the resurrect delete-saved page.
type deleteConfirmState struct {
	levelID           string
	item              menu.Item
	savedFilter       string
	savedFilterCursor int
}

// deleteSavedReloadedMsg refreshes the delete-saved listing in place after a
// successful delete.
type deleteSavedReloadedMsg struct {
	levelID      string
	items        []menu.Item
	err          error
	filter       string
	filterCursor int
}

// startDeleteConfirm enters the confirmation state on the current
// resurrect:delete-saved level.
func (m *Model) startDeleteConfirm(item menu.Item) {
	current := m.currentLevel()
	if current == nil {
		return
	}
	m.confirmState = &deleteConfirmState{
		levelID:           current.ID,
		item:              item,
		savedFilter:       current.Filter,
		savedFilterCursor: current.FilterCursorPos(),
	}
	current.SetFilter("", 0)
	m.errMsg = ""
	m.forceClearInfo()
}

// cancelDeleteConfirm aborts the confirmation prompt and restores the prior
// filter text.
func (m *Model) cancelDeleteConfirm() {
	if m.confirmState == nil {
		return
	}
	cs := m.confirmState
	m.confirmState = nil
	if cur := m.currentLevel(); cur != nil {
		cur.SetFilter(cs.savedFilter, cs.savedFilterCursor)
	}
}

// commitDeleteConfirm dispatches the underlying delete action. The saved
// filter state is stashed on the model so the post-delete reload can restore
// it.
func (m *Model) commitDeleteConfirm() tea.Cmd {
	if m.confirmState == nil {
		return nil
	}
	cs := m.confirmState
	m.confirmState = nil
	node, ok := m.registry.Find(cs.levelID)
	if !ok || node == nil || node.Action == nil {
		if cur := m.currentLevel(); cur != nil {
			cur.SetFilter(cs.savedFilter, cs.savedFilterCursor)
		}
		return nil
	}
	label := deleteSaveDisplayName(cs.item)
	m.loading = true
	m.pendingID = cs.levelID
	m.pendingLabel = label
	m.pendingDeleteFilter = cs.savedFilter
	m.pendingDeleteFilterCursor = cs.savedFilterCursor
	m.errMsg = ""
	m.forceClearInfo()
	return m.bus.Execute(m.menuContext(), command.Request{
		ID:      cs.levelID,
		Label:   label,
		Handler: node.Action,
		Item:    cs.item,
	})
}

// handleDeleteConfirmKey routes keystrokes while the confirmation prompt is
// shown. Unhandled keys are swallowed so they cannot stray into the menu
// behind the prompt.
func (m *Model) handleDeleteConfirmKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	case "y", "Y", "enter":
		return m.commitDeleteConfirm()
	case "n", "N", "esc":
		m.cancelDeleteConfirm()
		return nil
	}
	return nil
}

// renderDeleteConfirmPrompt builds the styled y/n prompt that replaces the
// filter input while a delete confirmation is in flight.
func (m *Model) renderDeleteConfirmPrompt() string {
	if m.confirmState == nil {
		return ""
	}
	name := deleteSaveDisplayName(m.confirmState.item)
	body := fmt.Sprintf("delete save '%s'?", name)
	hint := " (y/N)"

	bodyStyle := styles.Warning
	if bodyStyle == nil {
		fallback := lipgloss.NewStyle().Bold(true)
		bodyStyle = &fallback
	}
	hintStyle := styles.FilterPlaceholder
	if hintStyle == nil {
		fallback := lipgloss.NewStyle().Faint(true)
		hintStyle = &fallback
	}
	return bodyStyle.Render(body) + hintStyle.Render(hint)
}

// handleDeleteSavedActionResult finishes the post-confirm flow: surface a
// "deleted save NAME" notification, then reload the listing in place. Unlike
// the default ActionResult handler this does not quit the program.
func (m *Model) handleDeleteSavedActionResult(result menu.ActionResult) tea.Cmd {
	label := m.pendingLabel
	savedFilter := m.pendingDeleteFilter
	savedFilterCursor := m.pendingDeleteFilterCursor
	m.loading = false
	m.pendingID = ""
	m.pendingLabel = ""
	m.pendingDeleteFilter = ""
	m.pendingDeleteFilterCursor = 0
	if result.Err != nil {
		m.errMsg = result.Err.Error()
		if cur := m.currentLevel(); cur != nil {
			cur.SetFilter(savedFilter, savedFilterCursor)
		}
		return nil
	}
	m.setInfo(fmt.Sprintf("deleted save %s", label))
	return m.reloadDeleteSavedCmd(savedFilter, savedFilterCursor)
}

// reloadDeleteSavedCmd re-runs the delete-saved loader and emits a
// deleteSavedReloadedMsg with the fresh items so the current level can be
// updated in place.
func (m *Model) reloadDeleteSavedCmd(filter string, filterCursor int) tea.Cmd {
	node, ok := m.registry.Find(deleteSavedLevelID)
	if !ok || node == nil || node.Loader == nil {
		return nil
	}
	ctx := m.menuContext()
	loader := node.Loader
	return func() tea.Msg {
		items, err := loader(ctx)
		return deleteSavedReloadedMsg{
			levelID:      deleteSavedLevelID,
			items:        items,
			err:          err,
			filter:       filter,
			filterCursor: filterCursor,
		}
	}
}

// handleDeleteSavedReloadedMsg installs the refreshed items on the current
// level after a successful delete. The filter text is restored.
func (m *Model) handleDeleteSavedReloadedMsg(msg tea.Msg) tea.Cmd {
	update, ok := msg.(deleteSavedReloadedMsg)
	if !ok {
		return nil
	}
	current := m.currentLevel()
	if current == nil || current.ID != update.levelID {
		return nil
	}
	if update.err != nil {
		m.errMsg = update.err.Error()
		return nil
	}
	current.UpdateItems(update.items)
	current.SetFilter(update.filter, update.filterCursor)
	if len(current.Items) == 0 {
		current.Cursor = 0
	} else if current.Cursor < 0 || current.Cursor >= len(current.Items) {
		current.Cursor = 0
		current.SkipHeaders(1)
	} else if current.Cursor < len(current.Items) && current.Items[current.Cursor].Header {
		current.SkipHeaders(1)
	}
	m.syncFilterViewport(current)
	return nil
}

// deleteSaveDisplayName picks a friendly identifier for a save entry. The
// item ID is the absolute path to the snapshot file; basename is the
// unique on-disk name and matches what the action handler reports.
func deleteSaveDisplayName(item menu.Item) string {
	return filepath.Base(item.ID)
}
