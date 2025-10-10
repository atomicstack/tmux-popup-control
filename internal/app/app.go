package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"github.com/atomicstack/tmux-popup-control/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

// Config describes user-provided application options.
type Config struct {
	SocketPath string
	Width      int
	Height     int
	ShowFooter bool
	Verbose    bool
	RootMenu   string
}

// Run bootstraps and executes the Bubble Tea program.
func Run(cfg Config) error {
	socketPath, err := tmux.ResolveSocketPath(cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("resolve socket path: %w", err)
	}
	watcher := backend.NewWatcher(socketPath, 1500*time.Millisecond)
	defer watcher.Stop()
	model := ui.NewModel(socketPath, cfg.Width, cfg.Height, cfg.ShowFooter, cfg.Verbose, watcher, cfg.RootMenu)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	if errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}
