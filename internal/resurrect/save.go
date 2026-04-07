package resurrect

import (
	"fmt"
	"strings"
	"time"

	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

type SaveDeps struct {
	FetchSessions       func(socket string) (tmux.SessionSnapshot, error)
	FetchWindows        func(socket string) (tmux.WindowSnapshot, error)
	FetchPanes          func(socket string) (tmux.PaneSnapshot, error)
	CapturePaneContents func(socket, target string) (string, error)
	QueryWindowOptions  func(socket string) (map[string]bool, error)
	ClientInfo          func(socket, clientID string) (clientSession, clientLastSession string)
}

var saveDeps = SaveDeps{
	FetchSessions:       tmux.FetchSessions,
	FetchWindows:        tmux.FetchWindows,
	FetchPanes:          tmux.FetchPanes,
	CapturePaneContents: tmux.CapturePaneContents,
	QueryWindowOptions: func(string) (map[string]bool, error) {
		return map[string]bool{}, nil
	},
	ClientInfo: tmux.ClientSessionInfo,
}

// with* helpers replace the package-level vars for the duration of a test and
// return a restore function.

func withFetchSessionsFn(fn func(string) (tmux.SessionSnapshot, error)) func() {
	orig := saveDeps.FetchSessions
	saveDeps.FetchSessions = fn
	return func() { saveDeps.FetchSessions = orig }
}

func withFetchWindowsFn(fn func(string) (tmux.WindowSnapshot, error)) func() {
	orig := saveDeps.FetchWindows
	saveDeps.FetchWindows = fn
	return func() { saveDeps.FetchWindows = orig }
}

func withFetchPanesFn(fn func(string) (tmux.PaneSnapshot, error)) func() {
	orig := saveDeps.FetchPanes
	saveDeps.FetchPanes = fn
	return func() { saveDeps.FetchPanes = orig }
}

func withCapturePaneContentsFn(fn func(socket, target string) (string, error)) func() {
	orig := saveDeps.CapturePaneContents
	saveDeps.CapturePaneContents = fn
	return func() { saveDeps.CapturePaneContents = orig }
}

func withQueryWindowOptionsFn(fn func(string) (map[string]bool, error)) func() {
	orig := saveDeps.QueryWindowOptions
	saveDeps.QueryWindowOptions = fn
	return func() { saveDeps.QueryWindowOptions = orig }
}

func withClientInfoFn(fn func(string, string) (string, string)) func() {
	orig := saveDeps.ClientInfo
	saveDeps.ClientInfo = fn
	return func() { saveDeps.ClientInfo = orig }
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

	sessionSnap, err := saveDeps.FetchSessions(cfg.SocketPath)
	if err != nil {
		return sendError(ch, "fetching sessions: %w", err)
	}

	windowSnap, err := saveDeps.FetchWindows(cfg.SocketPath)
	if err != nil {
		return sendError(ch, "fetching windows: %w", err)
	}

	paneSnap, err := saveDeps.FetchPanes(cfg.SocketPath)
	if err != nil {
		return sendError(ch, "fetching panes: %w", err)
	}

	nSessions := len(sessionSnap.Sessions)

	// group windows by session
	windowsBySession := make(map[string][]tmux.Window, nSessions)
	for _, w := range windowSnap.Windows {
		windowsBySession[w.Session] = append(windowsBySession[w.Session], w)
	}

	// group panes by session, then by window
	type sessionWindow struct {
		session   string
		windowIdx int
	}
	panesByWindow := make(map[sessionWindow][]tmux.Pane)
	panesBySession := make(map[string][]tmux.Pane, nSessions)
	for _, p := range paneSnap.Panes {
		key := sessionWindow{session: p.Session, windowIdx: p.WindowIdx}
		panesByWindow[key] = append(panesByWindow[key], p)
		panesBySession[p.Session] = append(panesBySession[p.Session], p)
	}

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

	// ── Phase 2: build session tree (depth-first) ───────────────────────────

	// optionally query window options
	autoRenameMap, err := saveDeps.QueryWindowOptions(cfg.SocketPath)
	if err != nil {
		autoRenameMap = map[string]bool{}
	}

	// client info
	clientSess, clientLastSess := saveDeps.ClientInfo(cfg.SocketPath, cfg.ClientID)

	step := 0
	now := time.Now()
	var saveFile SaveFile
	saveFile.Version = currentVersion
	saveFile.Timestamp = now
	saveFile.Name = cfg.Name
	saveFile.Kind = normalizeSaveKind(cfg.Kind)
	saveFile.HasPaneContents = cfg.CapturePaneContents
	saveFile.ClientSession = clientSess
	saveFile.ClientLastSession = clientLastSess

	paneContents := map[string]string{}

	for _, s := range sessionSnap.Sessions {
		step++
		ch <- ProgressEvent{
			Step:    step,
			Total:   total,
			Message: fmt.Sprintf("saving session %s...", s.Name),
			Kind:    "session",
			ID:      s.Name,
		}

		sess := Session{
			Name:     s.Name,
			Path:     s.Path,
			Created:  parseCreated(s),
			Attached: s.Attached,
		}

		// ── windows for this session ────────────────────────────────
		wins := windowsBySession[s.Name]
		if len(wins) > 0 {
			var winIDs []string
			for _, w := range wins {
				winIDs = append(winIDs, fmt.Sprintf("%s:%d", s.Name, w.Index))

				autoRename := autoRenameMap[w.InternalID]
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

			step += len(wins)
			ch <- ProgressEvent{
				Step:    step,
				Total:   total,
				Message: fmt.Sprintf("saving windows for session %s: %s", s.Name, strings.Join(winIDs, " ")),
				Kind:    "window",
				ID:      s.Name,
			}
		}

		// ── capture panes for this session ──────────────────────────
		if cfg.CapturePaneContents {
			panes := panesBySession[s.Name]
			if len(panes) > 0 {
				var paneIDs []string
				for _, p := range panes {
					paneIDs = append(paneIDs, p.ID)
					content, err := saveDeps.CapturePaneContents(cfg.SocketPath, p.ID)
					if err != nil {
						return sendError(ch, "capturing pane %s: %w", p.ID, err)
					}
					paneContents[p.ID] = strings.TrimRight(content, "\n") + "\n"
				}

				step += len(panes)
				ch <- ProgressEvent{
					Step:    step,
					Total:   total,
					Message: fmt.Sprintf("capturing panes for session %s: %s", s.Name, strings.Join(paneIDs, " ")),
					Kind:    "pane",
					ID:      s.Name,
				}
			}
		}

		saveFile.Sessions = append(saveFile.Sessions, sess)
	}

	// ── Phase 3: write JSON ─────────────────────────────────────────────────

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

	// ── Phase 4: write pane archive ─────────────────────────────────────────

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

	// ── Phase 5: update last symlink ────────────────────────────────────────

	if shouldUpdateLast(cfg) {
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

	// ── Done ────────────────────────────────────────────────────────────────

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

func normalizeSaveKind(kind SaveKind) SaveKind {
	if kind == SaveKindAuto {
		return SaveKindAuto
	}
	return SaveKindManual
}

func shouldUpdateLast(cfg Config) bool {
	return normalizeSaveKind(cfg.Kind) == SaveKindAuto || cfg.Name == ""
}
