package app

import (
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// Run bootstraps and executes the Bubble Tea program.
func Run(socketPath string, width, height int, showFooter bool, verbose bool) error {
	watcher := backend.NewWatcher(socketPath, 1500*time.Millisecond)
	defer watcher.Stop()
	model := ui.NewModel(socketPath, width, height, showFooter, verbose, watcher)
	_, err := tea.NewProgram(model).Run()
	return err
}
