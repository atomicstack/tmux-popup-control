package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/backend"
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/resurrect"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"github.com/atomicstack/tmux-popup-control/internal/ui"
	"github.com/charmbracelet/colorprofile"
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
	NoPreview           bool
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
	model := ui.NewModel(ui.ModelConfig{
		SocketPath:  socketPath,
		Width:       cfg.Width,
		Height:      cfg.Height,
		ShowFooter:  cfg.ShowFooter,
		Verbose:     cfg.Verbose,
		NoPreview:   cfg.NoPreview,
		Watcher:     watcher,
		RootMenu:    cfg.RootMenu,
		MenuArgs:    cfg.MenuArgs,
		ClientID:    clientID,
		SessionName: strings.TrimSpace(cfg.SessionName),
	})
	if cfg.ResurrectOp != "" {
		model.SetResurrectInit(buildResurrectStart(cfg, socketPath, clientID))
	}
	program := tea.NewProgram(model, programOptions()...)
	span := logging.StartSpan("app", "run", logging.SpanOptions{
		Target: cfg.RootMenu,
		Attrs: map[string]any{
			"socket_path":   socketPath,
			"client_id":     clientID,
			"resurrect_op":  cfg.ResurrectOp,
			"show_footer":   cfg.ShowFooter,
			"restore_panes": cfg.RestorePaneContents,
		},
	})
	_, err = program.Run()
	span.End(err)
	if errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}

func programOptions() []tea.ProgramOption {
	options := make([]tea.ProgramOption, 0, 1)
	if profile, ok := colorProfileOverride(); ok {
		options = append(options, tea.WithColorProfile(profile))
	}
	return options
}

func colorProfileOverride() (colorprofile.Profile, bool) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TMUX_POPUP_CONTROL_COLOR_PROFILE"))) {
	case "":
		return colorprofile.Profile(0), false
	case "truecolor", "true-color", "24bit", "24-bit":
		return colorprofile.TrueColor, true
	case "ansi256", "ansi-256", "256", "256color", "256-color":
		return colorprofile.ANSI256, true
	case "ansi":
		return colorprofile.ANSI, true
	case "ascii":
		return colorprofile.ASCII, true
	default:
		return colorprofile.Profile(0), false
	}
}

func buildResurrectStart(cfg Config, socketPath, clientID string) menu.ResurrectStart {
	rcfg := resurrect.Config{
		SocketPath:          socketPath,
		CapturePaneContents: cfg.RestorePaneContents || resurrect.ResolvePaneContents(socketPath),
		Name:                cfg.ResurrectName,
		ClientID:            clientID,
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
