package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"github.com/atomicstack/tmux-popup-control/internal/ui"
	tea "charm.land/bubbletea/v2"
)

// Config describes user-provided application options.
type Config struct {
	SocketPath  string
	Width       int
	Height      int
	ShowFooter  bool
	Verbose     bool
	RootMenu    string
	MenuArgs    string
	ClientID    string
	SessionName string
}

// Run bootstraps and executes the Bubble Tea program.
func Run(cfg Config) error {
	socketPath, err := tmux.ResolveSocketPath(cfg.SocketPath)
	if err != nil {
		return fmt.Errorf("resolve socket path: %w", err)
	}
	defer tmux.Shutdown()
	clientID := strings.TrimSpace(cfg.ClientID)
	if clientID == "" {
		clientID = tmux.CurrentClientID(socketPath)
	}
	watcher := backend.NewWatcher(socketPath, 1500*time.Millisecond)
	defer watcher.Stop()
	model := ui.NewModel(socketPath, cfg.Width, cfg.Height, cfg.ShowFooter, cfg.Verbose, watcher, cfg.RootMenu, cfg.MenuArgs, clientID, strings.TrimSpace(cfg.SessionName))
	program := tea.NewProgram(model)
	_, err = program.Run()
	if errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}
