package resurrect

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// injectable functions — defaults call the real tmux package functions.

var createSessionFn = func(socketPath, name, dir string) error {
	return tmux.CreateSession(socketPath, name, dir)
}

var createWindowFn = func(socketPath, session string, index int, name, dir string) error {
	return tmux.CreateWindow(socketPath, session, index, name, dir)
}

var renameWindowFn = func(socketPath, target, newName string) error {
	return tmux.RenameWindow(socketPath, target, newName)
}

var splitPaneFn = func(socketPath, target, dir string) error {
	return tmux.SplitPane(socketPath, target, dir)
}

var selectLayoutTargetFn = func(socketPath, target, layout string) error {
	return tmux.SelectLayoutTarget(socketPath, target, layout)
}

var selectPaneFn = func(socketPath, target string) error {
	return tmux.SelectPane(socketPath, target)
}

var selectWindowFn = func(socketPath, target string) error {
	return tmux.SelectWindow(socketPath, target)
}

var sendPaneContentsFn = func(socketPath, target, contents string) error {
	return tmux.SendPaneContents(socketPath, target, contents)
}

var switchClientFn = func(socketPath, clientID, target string) error {
	return tmux.SwitchClient(socketPath, clientID, target)
}

var existingSessionsFn = func(socketPath string) (tmux.SessionSnapshot, error) {
	return tmux.FetchSessions(socketPath)
}

// with* helpers replace the package-level vars for the duration of a test and
// return a restore function.

func withCreateSessionFn(fn func(string, string, string) error) func() {
	orig := createSessionFn
	createSessionFn = fn
	return func() { createSessionFn = orig }
}

func withCreateWindowFn(fn func(string, string, int, string, string) error) func() {
	orig := createWindowFn
	createWindowFn = fn
	return func() { createWindowFn = orig }
}

func withRenameWindowFn(fn func(string, string, string) error) func() {
	orig := renameWindowFn
	renameWindowFn = fn
	return func() { renameWindowFn = orig }
}

func withSplitPaneFn(fn func(string, string, string) error) func() {
	orig := splitPaneFn
	splitPaneFn = fn
	return func() { splitPaneFn = orig }
}

func withSelectLayoutTargetFn(fn func(string, string, string) error) func() {
	orig := selectLayoutTargetFn
	selectLayoutTargetFn = fn
	return func() { selectLayoutTargetFn = orig }
}

func withSelectPaneFn(fn func(string, string) error) func() {
	orig := selectPaneFn
	selectPaneFn = fn
	return func() { selectPaneFn = orig }
}

func withSelectWindowFn(fn func(string, string) error) func() {
	orig := selectWindowFn
	selectWindowFn = fn
	return func() { selectWindowFn = orig }
}

func withSendPaneContentsFn(fn func(string, string, string) error) func() {
	orig := sendPaneContentsFn
	sendPaneContentsFn = fn
	return func() { sendPaneContentsFn = orig }
}

func withSwitchClientFn(fn func(string, string, string) error) func() {
	orig := switchClientFn
	switchClientFn = fn
	return func() { switchClientFn = orig }
}

func withExistingSessionsFn(fn func(string) (tmux.SessionSnapshot, error)) func() {
	orig := existingSessionsFn
	existingSessionsFn = fn
	return func() { existingSessionsFn = orig }
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
	var tempDir string
	if hasPaneArchive {
		tempDir, err = os.MkdirTemp("", "tmux-restore-*")
		if err != nil {
			return sendError(ch, "creating temp dir: %w", err)
		}
		if err := ExtractPaneArchive(archivePath, tempDir); err != nil {
			_ = os.RemoveAll(tempDir)
			return sendError(ch, "extracting pane archive: %w", err)
		}
	}

	// fetch existing sessions to detect conflicts
	existingSnap, err := existingSessionsFn(cfg.SocketPath)
	if err != nil {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
		return sendError(ch, "fetching existing sessions: %w", err)
	}
	existingNames := make(map[string]bool, len(existingSnap.Sessions))
	for _, s := range existingSnap.Sessions {
		existingNames[s.Name] = true
	}

	// compute total work units
	total := computeRestoreTotal(sf, hasPaneArchive)

	ch <- ProgressEvent{
		Step:    0,
		Total:   total,
		Message: fmt.Sprintf("restoring %d session(s) from %s", len(sf.Sessions), file),
		Kind:    "info",
	}

	// ── Phase 2: restore ─────────────────────────────────────────────────────

	step := 0

	for _, sess := range sf.Sessions {
		conflict := existingNames[sess.Name]

		if conflict {
			// skip session creation but still consume the budgeted steps
			step++
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("skipping session %s (already exists)", sess.Name),
				Kind:    "info",
				ID:      sess.Name,
			}
		} else {
			// determine starting directory from first pane of first window
			startDir := ""
			if len(sess.Windows) > 0 && len(sess.Windows[0].Panes) > 0 {
				startDir = sess.Windows[0].Panes[0].WorkingDir
			}

			step++
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("creating session %s", sess.Name),
				Kind:    "session",
				ID:      sess.Name,
			}
			if err := createSessionFn(cfg.SocketPath, sess.Name, startDir); err != nil {
				if tempDir != "" {
					_ = os.RemoveAll(tempDir)
				}
				return sendError(ch, "creating session %s: %w", sess.Name, err)
			}
		}

		for _, win := range sess.Windows {
			winTarget := fmt.Sprintf("%s:%d", sess.Name, win.Index)

			if conflict {
				// consume budgeted window step as a no-op
				step++
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("skipping window %s (session conflict)", winTarget),
					Kind:    "info",
					ID:      winTarget,
				}
			} else if win.Index == 0 {
				// first window is auto-created by tmux; rename it
				step++
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("renaming window %s to %s", winTarget, win.Name),
					Kind:    "window",
					ID:      winTarget,
				}
				if err := renameWindowFn(cfg.SocketPath, winTarget, win.Name); err != nil {
					if tempDir != "" {
						_ = os.RemoveAll(tempDir)
					}
					return sendError(ch, "renaming window %s: %w", winTarget, err)
				}
			} else {
				// create additional windows
				step++
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("creating window %s %s", winTarget, win.Name),
					Kind:    "window",
					ID:      winTarget,
				}
				paneDir := ""
				if len(win.Panes) > 0 {
					paneDir = win.Panes[0].WorkingDir
				}
				if err := createWindowFn(cfg.SocketPath, sess.Name, win.Index, win.Name, paneDir); err != nil {
					if tempDir != "" {
						_ = os.RemoveAll(tempDir)
					}
					return sendError(ch, "creating window %s: %w", winTarget, err)
				}
			}

			// create panes (skip first — auto-created)
			for _, pane := range win.Panes {
				if pane.Index == 0 {
					continue // auto-created, skip split
				}
				paneTarget := fmt.Sprintf("%s:%d", sess.Name, win.Index)

				if conflict {
					// consume budgeted pane step as a no-op
					step++
					ch <- ProgressEvent{
						Step:    step,
						Total:   total,
						Message: fmt.Sprintf("skipping pane %s.%d (session conflict)", paneTarget, pane.Index),
						Kind:    "info",
					}
				} else {
					step++
					ch <- ProgressEvent{
						Step:    step,
						Total:   total,
						Message: fmt.Sprintf("splitting pane %s.%d", paneTarget, pane.Index),
						Kind:    "pane",
					}
					if err := splitPaneFn(cfg.SocketPath, paneTarget, pane.WorkingDir); err != nil {
						if tempDir != "" {
							_ = os.RemoveAll(tempDir)
						}
						return sendError(ch, "splitting pane %s.%d: %w", paneTarget, pane.Index, err)
					}
				}
			}
		}

		// apply layouts
		for _, win := range sess.Windows {
			winTarget := fmt.Sprintf("%s:%d", sess.Name, win.Index)
			step++
			if conflict {
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("skipping layout for %s (session conflict)", winTarget),
					Kind:    "info",
				}
			} else {
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("applying layout for %s", winTarget),
					Kind:    "info",
				}
				if err := selectLayoutTargetFn(cfg.SocketPath, winTarget, win.Layout); err != nil {
					if tempDir != "" {
						_ = os.RemoveAll(tempDir)
					}
					return sendError(ch, "applying layout for %s: %w", winTarget, err)
				}
			}
		}

		// send pane contents
		if hasPaneArchive {
			for _, win := range sess.Windows {
				for _, pane := range win.Panes {
					paneKey := fmt.Sprintf("%s:%d.%d", sess.Name, win.Index, pane.Index)
					paneTarget := paneKey
					step++
					if conflict {
						ch <- ProgressEvent{
							Step:    step,
							Total:   total,
							Message: fmt.Sprintf("skipping pane contents for %s (session conflict)", paneTarget),
							Kind:    "info",
						}
					} else {
						ch <- ProgressEvent{
							Step:    step,
							Total:   total,
							Message: fmt.Sprintf("sending pane contents for %s", paneTarget),
							Kind:    "pane",
						}
						contentPath := filepath.Join(tempDir, paneKey)
						contents, readErr := os.ReadFile(contentPath)
						if readErr != nil {
							// missing content file is non-fatal: skip this pane
							continue
						}
						if err := sendPaneContentsFn(cfg.SocketPath, paneTarget, string(contents)); err != nil {
							if tempDir != "" {
								_ = os.RemoveAll(tempDir)
							}
							return sendError(ch, "sending pane contents for %s: %w", paneTarget, err)
						}
					}
				}
			}
		}

		// select active pane per window
		for _, win := range sess.Windows {
			winTarget := fmt.Sprintf("%s:%d", sess.Name, win.Index)
			step++

			// find active pane
			activePaneIdx := 0
			for _, pane := range win.Panes {
				if pane.Active {
					activePaneIdx = pane.Index
					break
				}
			}
			paneTarget := fmt.Sprintf("%s:%d.%d", sess.Name, win.Index, activePaneIdx)

			if conflict {
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("skipping active pane for %s (session conflict)", winTarget),
					Kind:    "info",
				}
			} else {
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("selecting active pane %s", paneTarget),
					Kind:    "info",
				}
				if err := selectPaneFn(cfg.SocketPath, paneTarget); err != nil {
					if tempDir != "" {
						_ = os.RemoveAll(tempDir)
					}
					return sendError(ch, "selecting active pane %s: %w", paneTarget, err)
				}
			}
		}

		// select active window
		activeWindowIdx := 0
		for _, win := range sess.Windows {
			if win.Active {
				activeWindowIdx = win.Index
				break
			}
		}
		activeWindowTarget := fmt.Sprintf("%s:%d", sess.Name, activeWindowIdx)
		step++
		if conflict {
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("skipping active window for %s (session conflict)", sess.Name),
				Kind:    "info",
			}
		} else {
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("selecting active window %s", activeWindowTarget),
				Kind:    "info",
			}
			if err := selectWindowFn(cfg.SocketPath, activeWindowTarget); err != nil {
				if tempDir != "" {
					_ = os.RemoveAll(tempDir)
				}
				return sendError(ch, "selecting active window %s: %w", activeWindowTarget, err)
			}
		}
	}

	// restore client session
	step++
	ch <- ProgressEvent{
		Step:    step,
		Total:   total,
		Message: fmt.Sprintf("switching client to session %s", sf.ClientSession),
		Kind:    "info",
	}
	if sf.ClientSession != "" {
		if err := switchClientFn(cfg.SocketPath, "", sf.ClientSession); err != nil {
			if tempDir != "" {
				_ = os.RemoveAll(tempDir)
			}
			return sendError(ch, "switching client to session %s: %w", sf.ClientSession, err)
		}
	}

	// clean up temp dir
	step++
	ch <- ProgressEvent{
		Step:    step,
		Total:   total,
		Message: "cleaning up",
		Kind:    "info",
	}
	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}

	// done
	ch <- ProgressEvent{
		Step:    total,
		Total:   total,
		Message: fmt.Sprintf("restored %d session(s)", len(sf.Sessions)),
		Kind:    "info",
		Done:    true,
	}
	return nil
}

// computeRestoreTotal computes the total number of work units for a restore.
func computeRestoreTotal(sf *SaveFile, hasPaneArchive bool) int {
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

		if hasPaneArchive {
			for _, win := range sess.Windows {
				total += len(win.Panes) // send pane contents
			}
		}

		for range sess.Windows {
			total++ // select active pane
		}

		total++ // select active window
	}

	total++ // switch client
	total++ // cleanup

	return total
}
