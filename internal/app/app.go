package app

import (
	"github.com/atomicstack/tmux-popup-control/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// Run bootstraps and executes the Bubble Tea program.
func Run(socketPath string, width, height int) error {
	model := ui.NewModel(socketPath, width, height)
	_, err := tea.NewProgram(model).Run()
	return err
}
