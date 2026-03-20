package resurrect

import (
	"fmt"
	"strings"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// injectable functions — defaults call the real tmux package functions.

var fetchSessionsFn = func(socket string) (tmux.SessionSnapshot, error) {
	return tmux.FetchSessions(socket)
}

var fetchWindowsFn = func(socket string) (tmux.WindowSnapshot, error) {
	return tmux.FetchWindows(socket)
}

var fetchPanesFn = func(socket string) (tmux.PaneSnapshot, error) {
	return tmux.FetchPanes(socket)
}

var capturePaneContentsFn = func(socket, target string) (string, error) {
	return tmux.CapturePaneContents(socket, target)
}

// queryWindowOptionsFn returns a map from window ID to automatic-rename bool.
// The real implementation would query tmux; it is wired in later.
var queryWindowOptionsFn = func(socket string) (map[string]bool, error) {
	return map[string]bool{}, nil
}

// clientInfoFn returns the current client's session and last-session.
// The real implementation would call DisplayMessage; it is wired in later.
var clientInfoFn = func(socket string) (clientSession, clientLastSession string) {
	return "", ""
}

// with* helpers replace the package-level vars for the duration of a test and
// return a restore function.

func withFetchSessionsFn(fn func(string) (tmux.SessionSnapshot, error)) func() {
	orig := fetchSessionsFn
	fetchSessionsFn = fn
	return func() { fetchSessionsFn = orig }
}

func withFetchWindowsFn(fn func(string) (tmux.WindowSnapshot, error)) func() {
	orig := fetchWindowsFn
	fetchWindowsFn = fn
	return func() { fetchWindowsFn = orig }
}

func withFetchPanesFn(fn func(string) (tmux.PaneSnapshot, error)) func() {
	orig := fetchPanesFn
	fetchPanesFn = fn
	return func() { fetchPanesFn = orig }
}

func withCapturePaneContentsFn(fn func(socket, target string) (string, error)) func() {
	orig := capturePaneContentsFn
	capturePaneContentsFn = fn
	return func() { capturePaneContentsFn = orig }
}

func withQueryWindowOptionsFn(fn func(string) (map[string]bool, error)) func() {
	orig := queryWindowOptionsFn
	queryWindowOptionsFn = fn
	return func() { queryWindowOptionsFn = orig }
}

func withClientInfoFn(fn func(string) (string, string)) func() {
	orig := clientInfoFn
	clientInfoFn = fn
	return func() { clientInfoFn = orig }
}

// Save orchestrates a full session save and emits ProgressEvents on the
// returned channel. The channel is closed after a Done event is sent.
func Save(cfg Config) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 32)
	go func() {
		defer close(ch)
		runSave(cfg, ch)
	}()
	return ch
}

// errorf sends an error done event and returns the error so the caller can
// return it to trigger the deferred close.
func sendError(ch chan<- ProgressEvent, format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	ch <- ProgressEvent{Kind: "error", Done: true, Err: err}
	return err
}

func runSave(cfg Config, ch chan<- ProgressEvent) error {
	// ── Phase 1: discovery ───────────────────────────────────────────────────

	sessionSnap, err := fetchSessionsFn(cfg.SocketPath)
	if err != nil {
		return sendError(ch, "fetching sessions: %w", err)
	}

	windowSnap, err := fetchWindowsFn(cfg.SocketPath)
	if err != nil {
		return sendError(ch, "fetching windows: %w", err)
	}

	paneSnap, err := fetchPanesFn(cfg.SocketPath)
	if err != nil {
		return sendError(ch, "fetching panes: %w", err)
	}

	// compute total work units
	nSessions := len(sessionSnap.Sessions)
	nWindows := len(windowSnap.Windows)
	nPanes := len(paneSnap.Panes)

	total := nSessions + nWindows
	if cfg.CapturePaneContents {
		total += nPanes
	}
	total++ // write json
	if cfg.CapturePaneContents {
		total++ // write archive
	}
	if cfg.Name == "" {
		total++ // update last symlink
	}

	ch <- ProgressEvent{
		Step:    0,
		Total:   total,
		Message: "discovering sessions...",
		Kind:    "info",
	}

	// ── Phase 2: build session tree ──────────────────────────────────────────

	// group windows by session
	windowsBySession := make(map[string][]tmux.Window, nSessions)
	for _, w := range windowSnap.Windows {
		windowsBySession[w.Session] = append(windowsBySession[w.Session], w)
	}

	// group panes by session:windowIndex
	type sessionWindow struct {
		session   string
		windowIdx int
	}
	panesByWindow := make(map[sessionWindow][]tmux.Pane)
	for _, p := range paneSnap.Panes {
		key := sessionWindow{session: p.Session, windowIdx: p.WindowIdx}
		panesByWindow[key] = append(panesByWindow[key], p)
	}

	// optionally query window options
	autoRenameMap, err := queryWindowOptionsFn(cfg.SocketPath)
	if err != nil {
		// non-fatal: continue without automatic-rename info
		autoRenameMap = map[string]bool{}
	}

	// client info
	clientSess, clientLastSess := clientInfoFn(cfg.SocketPath)

	step := 0
	now := time.Now()
	var saveFile SaveFile
	saveFile.Version = currentVersion
	saveFile.Timestamp = now
	saveFile.Name = cfg.Name
	saveFile.HasPaneContents = cfg.CapturePaneContents
	saveFile.ClientSession = clientSess
	saveFile.ClientLastSession = clientLastSess

	for _, s := range sessionSnap.Sessions {
		step++
		ch <- ProgressEvent{
			Step:    step,
			Total:   total,
			Message: fmt.Sprintf("saving session %s", s.Name),
			Kind:    "session",
			ID:      s.Name,
		}

		created := parseCreated(s)

		sess := Session{
			Name:     s.Name,
			Created:  created,
			Attached: s.Attached,
		}

		for _, w := range windowsBySession[s.Name] {
			step++
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("saving window %s:%d", s.Name, w.Index),
				Kind:    "window",
				ID:      w.ID,
			}

			autoRename := autoRenameMap[w.InternalID]

			// build panes for this window
			key := sessionWindow{session: s.Name, windowIdx: w.Index}
			var savedPanes []Pane
			for _, p := range panesByWindow[key] {
				savedPanes = append(savedPanes, Pane{
					Index:      p.Index,
					WorkingDir: p.Path,
					Title:      p.Title,
					Command:    p.Command,
					Width:      p.Width,
					Height:     p.Height,
					Active:     p.Active,
				})
			}

			sess.Windows = append(sess.Windows, Window{
				Index:           w.Index,
				Name:            w.Name,
				Layout:          w.Layout,
				Active:          w.Active,
				AutomaticRename: autoRename,
				Panes:           savedPanes,
			})
		}

		saveFile.Sessions = append(saveFile.Sessions, sess)
	}

	// ── Phase 3: capture pane contents ──────────────────────────────────────

	paneContents := map[string]string{}
	if cfg.CapturePaneContents {
		for _, p := range paneSnap.Panes {
			step++
			target := p.ID
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("capturing pane %s", target),
				Kind:    "pane",
				ID:      target,
			}
			content, err := capturePaneContentsFn(cfg.SocketPath, target)
			if err != nil {
				return sendError(ch, "capturing pane %s: %w", target, err)
			}
			paneContents[target] = strings.TrimRight(content, "\n") + "\n"
		}
	}

	// ── Phase 4: write JSON ──────────────────────────────────────────────────

	jsonPath := savePath(cfg.SaveDir, cfg.Name)
	step++
	ch <- ProgressEvent{
		Step:    step,
		Total:   total,
		Message: fmt.Sprintf("writing %s", jsonPath),
		Kind:    "info",
	}
	if err := WriteSaveFile(jsonPath, &saveFile); err != nil {
		return sendError(ch, "writing save file: %w", err)
	}

	// ── Phase 5: write pane archive ──────────────────────────────────────────

	if cfg.CapturePaneContents {
		archivePath := paneArchivePath(jsonPath)
		step++
		ch <- ProgressEvent{
			Step:    step,
			Total:   total,
			Message: fmt.Sprintf("writing pane archive %s", archivePath),
			Kind:    "info",
		}
		if err := WritePaneArchive(archivePath, paneContents); err != nil {
			return sendError(ch, "writing pane archive: %w", err)
		}
	}

	// ── Phase 6: update last symlink ─────────────────────────────────────────

	if cfg.Name == "" {
		step++
		ch <- ProgressEvent{
			Step:    step,
			Total:   total,
			Message: "updating last symlink",
			Kind:    "info",
		}
		if err := updateLastSymlink(cfg.SaveDir, jsonPath); err != nil {
			return sendError(ch, "updating last symlink: %w", err)
		}
	}

	// ── Done ─────────────────────────────────────────────────────────────────

	ch <- ProgressEvent{
		Step:    total,
		Total:   total,
		Message: fmt.Sprintf("saved %d session(s) to %s", nSessions, jsonPath),
		Kind:    "info",
		Done:    true,
	}
	return nil
}

// parseCreated returns the session creation timestamp.
// tmux.Session does not currently expose the raw Created string from the
// gotmux layer — wiring that value is left for a future task. For now we
// return 0; the restore flow does not depend on this field.
func parseCreated(_ tmux.Session) int64 {
	return 0
}
