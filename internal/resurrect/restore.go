package resurrect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/shquote"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

type RestoreDeps struct {
	CreateSession        func(tmux.SessionSpec) error
	CreateWindow         func(tmux.WindowSpec) error
	RenameWindow         func(socketPath, target, newName string) error
	SplitPane            func(tmux.PaneSpec) error
	SelectLayoutTarget   func(socketPath, target, layout string) error
	RespawnPane          func(tmux.PaneSpec) error
	SelectPane           func(socketPath, target string) error
	SelectWindow         func(socketPath, target string) error
	SwitchClient         func(socketPath, clientID, target string) error
	ExistingSessions     func(socketPath string) (tmux.SessionSnapshot, error)
	DefaultCommand       func(socketPath string) string
	ExistingWindowIndices func(socketPath, sessionName string) (map[int]bool, error)
	SessionOption        func(socketPath, session, option string) string
	SetSessionOption     func(socketPath, session, option, value string) error
}

var restoreDeps = RestoreDeps{
	CreateSession:         tmux.CreateSession,
	CreateWindow:          tmux.CreateWindow,
	RenameWindow:          tmux.RenameWindow,
	SplitPane:             tmux.SplitPane,
	SelectLayoutTarget:    tmux.SelectLayoutTarget,
	RespawnPane:           tmux.RespawnPane,
	SelectPane:            tmux.SelectPane,
	SelectWindow:          tmux.SelectWindow,
	SwitchClient:          tmux.SwitchClient,
	ExistingSessions:      tmux.FetchSessions,
	DefaultCommand:        tmux.DefaultCommand,
	ExistingWindowIndices: tmux.WindowIndices,
	SessionOption:         tmux.SessionOption,
	SetSessionOption:      tmux.SetSessionOption,
}

// restoreMarkerKey returns the tmux session option name used to record that
// a saved session has already been merged into an existing session.
func restoreMarkerKey(sessionName string) string {
	return "@tmux-popup-control-session-restored-" + sessionName
}

// with* helpers replace the package-level vars for the duration of a test and
// return a restore function.

func withCreateSessionFn(fn func(tmux.SessionSpec) error) func() {
	orig := restoreDeps.CreateSession
	restoreDeps.CreateSession = fn
	return func() { restoreDeps.CreateSession = orig }
}

func withCreateWindowFn(fn func(tmux.WindowSpec) error) func() {
	orig := restoreDeps.CreateWindow
	restoreDeps.CreateWindow = fn
	return func() { restoreDeps.CreateWindow = orig }
}

func withRenameWindowFn(fn func(string, string, string) error) func() {
	orig := restoreDeps.RenameWindow
	restoreDeps.RenameWindow = fn
	return func() { restoreDeps.RenameWindow = orig }
}

func withSplitPaneFn(fn func(tmux.PaneSpec) error) func() {
	orig := restoreDeps.SplitPane
	restoreDeps.SplitPane = fn
	return func() { restoreDeps.SplitPane = orig }
}

func withSelectLayoutTargetFn(fn func(string, string, string) error) func() {
	orig := restoreDeps.SelectLayoutTarget
	restoreDeps.SelectLayoutTarget = fn
	return func() { restoreDeps.SelectLayoutTarget = orig }
}

func withRespawnPaneFn(fn func(tmux.PaneSpec) error) func() {
	orig := restoreDeps.RespawnPane
	restoreDeps.RespawnPane = fn
	return func() { restoreDeps.RespawnPane = orig }
}

func withSelectPaneFn(fn func(string, string) error) func() {
	orig := restoreDeps.SelectPane
	restoreDeps.SelectPane = fn
	return func() { restoreDeps.SelectPane = orig }
}

func withSelectWindowFn(fn func(string, string) error) func() {
	orig := restoreDeps.SelectWindow
	restoreDeps.SelectWindow = fn
	return func() { restoreDeps.SelectWindow = orig }
}

func withSwitchClientFn(fn func(string, string, string) error) func() {
	orig := restoreDeps.SwitchClient
	restoreDeps.SwitchClient = fn
	return func() { restoreDeps.SwitchClient = orig }
}

func withExistingSessionsFn(fn func(string) (tmux.SessionSnapshot, error)) func() {
	orig := restoreDeps.ExistingSessions
	restoreDeps.ExistingSessions = fn
	return func() { restoreDeps.ExistingSessions = orig }
}

func withDefaultCommandFn(fn func(string) string) func() {
	orig := restoreDeps.DefaultCommand
	restoreDeps.DefaultCommand = fn
	return func() { restoreDeps.DefaultCommand = orig }
}

func withExistingWindowIndicesFn(fn func(string, string) (map[int]bool, error)) func() {
	orig := restoreDeps.ExistingWindowIndices
	restoreDeps.ExistingWindowIndices = fn
	return func() { restoreDeps.ExistingWindowIndices = orig }
}

func withSessionOptionFn(fn func(string, string, string) string) func() {
	orig := restoreDeps.SessionOption
	restoreDeps.SessionOption = fn
	return func() { restoreDeps.SessionOption = orig }
}

func withSetSessionOptionFn(fn func(string, string, string, string) error) func() {
	orig := restoreDeps.SetSessionOption
	restoreDeps.SetSessionOption = fn
	return func() { restoreDeps.SetSessionOption = orig }
}

// Restore orchestrates a full session restore and emits ProgressEvents on the
// returned channel. The channel is closed after a Done event is sent.
func Restore(cfg Config, file string) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 32)
	go func() {
		defer close(ch)
		runRestore(cfg, file, ch)
	}()
	return ch
}

var greenCheck = lipgloss.NewStyle().Foreground(lipgloss.Color("#00c853")).Render("✓")

// paneStartupCommand builds the startup command for a pane that has saved
// content. The command prints the saved scrollback then execs into the shell.
// Both contentPath and defaultCmd are shell-quoted to prevent injection when
// tmux passes the string to /bin/sh -c.
func paneStartupCommand(contentPath, defaultCmd string) string {
	return fmt.Sprintf("cat %s; exec %s", shquote.Quote(contentPath), shquote.Quote(defaultCmd))
}

func runRestore(cfg Config, file string, ch chan<- ProgressEvent) error {
	// ── Phase 1: discovery ───────────────────────────────────────────────────

	sf, err := ReadSaveFile(file)
	if err != nil {
		return sendError(ch, "reading save file: %w", err)
	}

	// check for companion pane archive
	archivePath := paneArchivePath(file)
	hasPaneArchive := false
	if _, err := os.Stat(archivePath); err == nil {
		hasPaneArchive = true
	}

	// extract pane archive to a temp dir if present
	var contentDir string
	if hasPaneArchive {
		contentDir, err = os.MkdirTemp("", "tmux-restore-*")
		if err != nil {
			return sendError(ch, "creating temp dir: %w", err)
		}
		if err := ExtractPaneArchive(archivePath, contentDir); err != nil {
			_ = os.RemoveAll(contentDir)
			return sendError(ch, "extracting pane archive: %w", err)
		}
	}

	// fetch existing sessions to detect conflicts
	existingSnap, err := restoreDeps.ExistingSessions(cfg.SocketPath)
	if err != nil {
		return sendError(ch, "fetching existing sessions: %w", err)
	}
	existingNames := make(map[string]bool, len(existingSnap.Sessions))
	for _, s := range existingSnap.Sessions {
		existingNames[s.Name] = true
	}

	// resolve default command for startup command chains
	defaultCmd := ""
	if hasPaneArchive {
		defaultCmd = restoreDeps.DefaultCommand(cfg.SocketPath)
	}

	// lookupPaneCmd returns the startup command for a pane if it has saved
	// content, or empty string otherwise.
	lookupPaneCmd := func(sessName string, winIdx, paneIdx int) string {
		if !hasPaneArchive {
			return ""
		}
		paneKey := fmt.Sprintf("%s:%d.%d", sessName, winIdx, paneIdx)
		path := filepath.Join(contentDir, paneKey)
		if _, statErr := os.Stat(path); statErr == nil {
			return paneStartupCommand(path, defaultCmd)
		}
		return ""
	}

	// compute total work units
	total := computeRestoreTotal(sf)

	ch <- ProgressEvent{
		Step:    0,
		Total:   total,
		Message: fmt.Sprintf("restoring %d session(s) from %s", len(sf.Sessions), file),
		Kind:    "info",
	}

	// ── Phase 2: restore (depth-first, grouped messages) ────────────────────

	step := 0

	for _, sess := range sf.Sessions {
		merge := existingNames[sess.Name]

		// ── session creation or merge header ────────────────────────

		// indexMap translates saved window indices to actual target indices.
		// For new sessions, indices are used as-is. For merges, saved
		// windows are appended after the highest existing index.
		indexMap := make(map[int]int, len(sess.Windows))

		if merge {
			// idempotency: skip if this slot was already merged
			markerKey := restoreMarkerKey(sess.Name)
			if restoreDeps.SessionOption(cfg.SocketPath, sess.Name, markerKey) != "" {
				sessSteps := 2 + 3*len(sess.Windows)
				for _, win := range sess.Windows {
					for _, pane := range win.Panes {
						if pane.Index != 0 {
							sessSteps++
						}
					}
				}
				step += sessSteps
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("skipping session %s (already restored)", sess.Name),
					Kind:    "info",
					ID:      sess.Name,
				}
				continue
			}

			existingIndices, err := restoreDeps.ExistingWindowIndices(cfg.SocketPath, sess.Name)
			if err != nil {
				return sendError(ch, "listing windows for session %s: %w", sess.Name, err)
			}
			maxIdx := -1
			for idx := range existingIndices {
				if idx > maxIdx {
					maxIdx = idx
				}
			}
			nextIdx := maxIdx + 1
			for _, win := range sess.Windows {
				indexMap[win.Index] = nextIdx
				nextIdx++
			}

			step++
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("merging into session %s...", sess.Name),
				Kind:    "session",
				ID:      sess.Name,
			}
		} else {
			for _, win := range sess.Windows {
				indexMap[win.Index] = win.Index
			}

			step++
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("restoring session %s...", sess.Name),
				Kind:    "session",
				ID:      sess.Name,
			}
			// create the session with $HOME as the working directory so
			// that new windows inherit the correct default. we must pass
			// -c explicitly because tmux otherwise inherits the cwd of
			// whatever process (control-mode client, popup, etc.) sends
			// the new-session command.
			sessionDir := os.Getenv("HOME")
			if err := restoreDeps.CreateSession(tmux.SessionSpec{
				SocketPath: cfg.SocketPath,
				Name:       sess.Name,
				Dir:        sessionDir,
			}); err != nil {
				return sendError(ch, "creating session %s: %w", sess.Name, err)
			}

			// the first pane is auto-created with the session; respawn it
			// in the correct working directory with any startup command.
			// this avoids polluting session_path with a pane-specific dir.
			if len(sess.Windows) > 0 && len(sess.Windows[0].Panes) > 0 {
				p0 := sess.Windows[0].Panes[0]
				paneCmd := lookupPaneCmd(sess.Name, sess.Windows[0].Index, p0.Index)
				if p0.WorkingDir != "" || paneCmd != "" {
					paneTarget := fmt.Sprintf("%s:0.0", sess.Name)
					if err := restoreDeps.RespawnPane(tmux.PaneSpec{
						SocketPath: cfg.SocketPath,
						Target:     paneTarget,
						Dir:        p0.WorkingDir,
						Command:    paneCmd,
					}); err != nil {
						return sendError(ch, "respawning pane %s: %w", paneTarget, err)
					}
				}
			}
		}

		// ── windows (grouped) ───────────────────────────────────────

		var winIDs []string
		for _, win := range sess.Windows {
			targetIdx := indexMap[win.Index]
			winTarget := fmt.Sprintf("%s:%d", sess.Name, targetIdx)
			winIDs = append(winIDs, winTarget)

			if !merge && win.Index == 0 {
				// first window of a new session is auto-created; rename it
				if err := restoreDeps.RenameWindow(cfg.SocketPath, winTarget, win.Name); err != nil {
					return sendError(ch, "renaming window %s: %w", winTarget, err)
				}
			} else {
				winDir := ""
				winCmd := ""
				if len(win.Panes) > 0 {
					winDir = win.Panes[0].WorkingDir
					winCmd = lookupPaneCmd(sess.Name, win.Index, win.Panes[0].Index)
				}
				if err := restoreDeps.CreateWindow(tmux.WindowSpec{
					SocketPath: cfg.SocketPath,
					Session:    sess.Name,
					Index:      targetIdx,
					Name:       win.Name,
					Dir:        winDir,
					Command:    winCmd,
				}); err != nil {
					return sendError(ch, "creating window %s: %w", winTarget, err)
				}
			}
		}
		if len(winIDs) > 0 {
			step += len(winIDs)
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("restoring windows for session %s: %s", sess.Name, strings.Join(winIDs, " ")),
				Kind:    "window",
			}
		}

		// ── pane splits (grouped) ───────────────────────────────────

		var paneIDs []string
		for _, win := range sess.Windows {
			targetIdx := indexMap[win.Index]
			for _, pane := range win.Panes {
				if pane.Index == 0 {
					continue
				}
				paneTarget := fmt.Sprintf("%s:%d", sess.Name, targetIdx)
				paneCmd := lookupPaneCmd(sess.Name, win.Index, pane.Index)
				if err := restoreDeps.SplitPane(tmux.PaneSpec{
					SocketPath: cfg.SocketPath,
					Target:     paneTarget,
					Dir:        pane.WorkingDir,
					Command:    paneCmd,
				}); err != nil {
					return sendError(ch, "splitting pane %s.%d: %w", paneTarget, pane.Index, err)
				}
				paneIDs = append(paneIDs, fmt.Sprintf("%s.%d", paneTarget, pane.Index))
			}
		}
		if len(paneIDs) > 0 {
			step += len(paneIDs)
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("splitting panes for session %s: %s", sess.Name, strings.Join(paneIDs, " ")),
				Kind:    "pane",
			}
		}

		// ── finalize: layouts, active panes, active window ──────────

		for _, win := range sess.Windows {
			targetIdx := indexMap[win.Index]
			winTarget := fmt.Sprintf("%s:%d", sess.Name, targetIdx)
			step++
			if err := restoreDeps.SelectLayoutTarget(cfg.SocketPath, winTarget, win.Layout); err != nil {
				return sendError(ch, "applying layout for %s: %w", winTarget, err)
			}
		}

		for _, win := range sess.Windows {
			targetIdx := indexMap[win.Index]
			activePaneIdx := 0
			for _, pane := range win.Panes {
				if pane.Active {
					activePaneIdx = pane.Index
					break
				}
			}
			paneTarget := fmt.Sprintf("%s:%d.%d", sess.Name, targetIdx, activePaneIdx)
			step++
			if err := restoreDeps.SelectPane(cfg.SocketPath, paneTarget); err != nil {
				return sendError(ch, "selecting active pane %s: %w", paneTarget, err)
			}
		}

		activeWindowIdx := 0
		for _, win := range sess.Windows {
			if win.Active {
				activeWindowIdx = win.Index
				break
			}
		}
		activeWindowTarget := fmt.Sprintf("%s:%d", sess.Name, indexMap[activeWindowIdx])
		step++
		if err := restoreDeps.SelectWindow(cfg.SocketPath, activeWindowTarget); err != nil {
			return sendError(ch, "selecting active window %s: %w", activeWindowTarget, err)
		}

		ch <- ProgressEvent{
			Step:    step,
			Total:   total,
			Message: fmt.Sprintf("finalizing session %s...", sess.Name),
			Kind:    "info",
		}

		// mark restored sessions so re-running the same restore is idempotent
		markerKey := restoreMarkerKey(sess.Name)
		if err := restoreDeps.SetSessionOption(cfg.SocketPath, sess.Name, markerKey, "1"); err != nil {
			return sendError(ch, "setting restore marker for session %s: %w", sess.Name, err)
		}
	}

	// restore client session
	step++
	if sf.ClientSession != "" {
		if err := restoreDeps.SwitchClient(cfg.SocketPath, cfg.ClientID, sf.ClientSession); err != nil {
			return sendError(ch, "switching client to session %s: %w", sf.ClientSession, err)
		}
	}
	ch <- ProgressEvent{
		Step:    step,
		Total:   total,
		Message: fmt.Sprintf("switched client to session %s %s", sf.ClientSession, greenCheck),
		Kind:    "info",
	}

	// schedule background cleanup of extracted pane content files. the
	// startup commands (cat "<file>"; exec <shell>) run asynchronously inside
	// the new panes; we wait 5 seconds to give them time to read the files.
	// if the process exits before the timer fires the temp dir persists
	// harmlessly — the OS cleans it up eventually.
	if contentDir != "" {
		go func() {
			time.Sleep(5 * time.Second)
			_ = os.RemoveAll(contentDir)
		}()
	}

	// done
	ch <- ProgressEvent{
		Step:    total,
		Total:   total,
		Message: fmt.Sprintf("restored %d session(s) %s", len(sf.Sessions), greenCheck),
		Kind:    "info",
		Done:    true,
	}
	return nil
}

// computeRestoreTotal computes the total number of work units for a restore.
// Pane content restore is handled at creation time (via startup commands),
// so there are no separate send-pane-contents steps.
func computeRestoreTotal(sf *SaveFile) int {
	total := 0
	for _, sess := range sf.Sessions {
		total++ // create or skip session

		for _, win := range sess.Windows {
			total++ // create or rename window

			// non-first panes (split-window)
			for _, pane := range win.Panes {
				if pane.Index != 0 {
					total++
				}
			}

			total++ // select-layout
		}

		for range sess.Windows {
			total++ // select active pane
		}

		total++ // select active window
	}

	total++ // switch client

	return total
}
