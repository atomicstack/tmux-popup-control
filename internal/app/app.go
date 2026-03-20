package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"github.com/atomicstack/tmux-popup-control/internal/ui"
)

// Config describes user-provided application options.
type Config struct {
	SocketPath          string
	Width               int
	Height              int
	ShowFooter          bool
	Verbose             bool
	RootMenu            string
	MenuArgs            string
	ClientID            string
	SessionName         string
	SessionStorageDir   string
	RestorePaneContents bool
	// ResurrectOp is set to "save" or "restore" when launched as a popup
	// instance for the save/restore-sessions CLI subcommand.
	ResurrectOp   string
	ResurrectName string // snapshot name (save --name)
	ResurrectFrom string // save file path or name (restore --from)
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
	if cfg.ResurrectOp != "" {
		model.SetResurrectInit(buildResurrectStart(cfg, socketPath))
	}
	program := tea.NewProgram(model)
	_, err = program.Run()
	if errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}

func buildResurrectStart(cfg Config, socketPath string) menu.ResurrectStart {
	rcfg := resurrect.Config{
		SocketPath:          socketPath,
		CapturePaneContents: cfg.RestorePaneContents || resurrect.ResolvePaneContents(socketPath),
		Name:                cfg.ResurrectName,
	}
	// resolve save dir
	if cfg.SessionStorageDir != "" {
		rcfg.SaveDir = cfg.SessionStorageDir
	} else {
		if dir, err := resurrect.ResolveDir(socketPath); err == nil {
			rcfg.SaveDir = dir
		}
	}

	start := menu.ResurrectStart{
		Operation: cfg.ResurrectOp,
		Name:      cfg.ResurrectName,
		Config:    rcfg,
	}
	if cfg.ResurrectOp == "restore" {
		if cfg.ResurrectFrom != "" {
			start.SaveFile = cfg.ResurrectFrom
		} else if dir := rcfg.SaveDir; dir != "" {
			if path, err := resurrect.LatestSave(dir); err == nil {
				start.SaveFile = path
			}
		}
	}
	return start
}
