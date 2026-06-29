package resurrect

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/atomicstack/tmux-popup-control/internal/shquote"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

type RestoreDeps struct {
	CreateSession         func(tmux.SessionSpec) error
	CreateWindow          func(tmux.WindowSpec) error
	RenameWindow          func(socketPath, target, newName string) error
	SplitPane             func(tmux.PaneSpec) error
	SelectLayoutTarget    func(socketPath, target, layout string) error
	RespawnPane           func(tmux.PaneSpec) error
	WaitFor               func(ctx context.Context, socketPath, channel string) error
	SelectPane            func(socketPath, target string) error
	SelectWindow          func(socketPath, target string) error
	SwitchClient          func(socketPath, clientID, target string) error
	ExistingSessions      func(socketPath string) (tmux.SessionSnapshot, error)
	DefaultCommand        func(socketPath string) string
	ExistingWindowIndices func(socketPath, sessionName string) (map[int]bool, error)
	SessionOption         func(socketPath, session, option string) string
	SetSessionOption      func(socketPath, session, option, value string) error
}

var restoreDeps = RestoreDeps{
	CreateSession:      tmux.CreateSession,
	CreateWindow:       tmux.CreateWindow,
	RenameWindow:       tmux.RenameWindow,
	SplitPane:          tmux.SplitPane,
	SelectLayoutTarget: tmux.SelectLayoutTarget,
	RespawnPane:        tmux.RespawnPane,
	WaitFor: func(ctx context.Context, socketPath, channel string) error {
		return tmux.WaitFor(ctx, socketPath, channel, replayWaitTimeout)
	},
	SelectPane:            tmux.SelectPane,
	SelectWindow:          tmux.SelectWindow,
	SwitchClient:          tmux.SwitchClient,
	ExistingSessions:      tmux.FetchSessions,
	DefaultCommand:        tmux.DefaultCommand,
	ExistingWindowIndices: tmux.WindowIndices,
	SessionOption:         tmux.SessionOption,
	SetSessionOption:      tmux.SetSessionOption,
}

// replayWaitTimeout bounds how long the restore waits for a single pane's
// content-replay channel to be signaled. Once tmux is draining pane PTYs
// normally (see the no-output flow-control change), a `cat` of saved
// scrollback signals in well under a second; this generous ceiling exists only
// so a pane whose replay died never wedges the whole restore — it fails with a
// clear error instead of blocking forever.
const replayWaitTimeout = 30 * time.Second

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

func withWaitForFn(fn func(ctx context.Context, socketPath, channel string) error) func() {
	orig := restoreDeps.WaitFor
	restoreDeps.WaitFor = fn
	return func() { restoreDeps.WaitFor = orig }
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
// The provided context cancels the background goroutine if the consumer stops
// draining the channel, preventing a goroutine leak on a buffered-channel
// blocking send.
func Restore(ctx context.Context, cfg Config, file string) <-chan ProgressEvent {
	ch := make(chan ProgressEvent, 32)
	go func() {
		defer close(ch)
		runRestore(ctx, cfg, file, ch)
	}()
	return ch
}

var greenCheck = lipgloss.NewStyle().Foreground(lipgloss.Color("#00c853")).Render("✓")

// paneStartupCommand builds the startup command for a pane that has saved
// content. The command prints the saved scrollback then execs into the shell.
// Both contentPath and defaultCmd are shell-quoted to prevent injection when
// tmux passes the string to /bin/sh -c.
func paneStartupCommand(contentPath, readyChannel, defaultCmd, tmuxCommand, socketPath string) string {
	cmd := shquote.JoinCommand("cat", contentPath)
	if strings.TrimSpace(readyChannel) != "" {
		args := []string{tmuxCommand}
		if strings.TrimSpace(socketPath) != "" {
			args = append(args, "-S", socketPath)
		}
		args = append(args, "wait-for", "-S", readyChannel)
		cmd += "; " + shquote.JoinCommand(args...)
	}
	return cmd + fmt.Sprintf("; exec %s", shquote.Quote(defaultCmd))
}

func tmuxCommandPath() string {
	path, err := exec.LookPath("tmux")
	if err != nil {
		return "tmux"
	}
	return path
}

func paneReplayWaitChannel(sessName string, winIdx, paneIdx int) string {
	return fmt.Sprintf("tmux-popup-control-pane-restored-%s:%d.%d", sessName, winIdx, paneIdx)
}

// restoreRun carries the cross-cutting state shared by the per-session restore
// helpers: the cancellation context, configuration, the progress channel, the
// precomputed total, the pane-content lookup, and a running step counter.
type restoreRun struct {
	ctx           context.Context
	cfg           Config
	ch            chan<- ProgressEvent
	total         int
	step          int
	lookupPaneCmd func(sessName string, winIdx, paneIdx int) string
}

// emit sends a progress event, advancing through the run. It returns false when
// the context was cancelled before the send completed so callers can abort.
func (r *restoreRun) emit(ev ProgressEvent) bool {
	ev.Total = r.total
	return sendProgress(r.ctx, r.ch, ev)
}

func runRestore(ctx context.Context, cfg Config, file string, ch chan<- ProgressEvent) error {
	// ── Phase 1: discovery ───────────────────────────────────────────────────

	sf, err := ReadSaveFile(file)
	if err != nil {
		return sendError(ctx, ch, "reading save file: %w", err)
	}

	contentDir, lookupPaneCmd, err := preparePaneContent(ctx, cfg, file, ch)
	if err != nil {
		return err
	}

	// fetch existing sessions to detect conflicts
	existingSnap, err := restoreDeps.ExistingSessions(cfg.SocketPath)
	if err != nil {
		return sendError(ctx, ch, "fetching existing sessions: %w", err)
	}
	existingNames := make(map[string]bool, len(existingSnap.Sessions))
	for _, s := range existingSnap.Sessions {
		existingNames[s.Name] = true
	}

	run := &restoreRun{
		ctx:           ctx,
		cfg:           cfg,
		ch:            ch,
		total:         computeRestoreTotal(sf),
		lookupPaneCmd: lookupPaneCmd,
	}

	if !run.emit(ProgressEvent{
		Step:    0,
		Message: fmt.Sprintf("restoring %d session(s) from %s", len(sf.Sessions), file),
		Kind:    "info",
	}) {
		return ctx.Err()
	}

	// ── Phase 2: restore (depth-first, grouped messages) ────────────────────

	for _, sess := range sf.Sessions {
		if err := run.restoreSession(sess, existingNames[sess.Name]); err != nil {
			return err
		}
	}

	if err := run.switchClient(sf); err != nil {
		return err
	}

	scheduleContentCleanup(contentDir)

	// done
	run.emit(ProgressEvent{
		Step:    run.total,
		Message: fmt.Sprintf("restored %d session(s) %s", len(sf.Sessions), greenCheck),
		Kind:    "info",
		Done:    true,
	})
	return nil
}

// preparePaneContent extracts the companion pane archive (if present) into a
// temp dir and returns that dir plus a lookup closure that yields the startup
// command for a pane with saved content (empty string when there is none).
// The returned contentDir is "" when no archive exists.
func preparePaneContent(ctx context.Context, cfg Config, file string, ch chan<- ProgressEvent) (string, func(string, int, int) string, error) {
	archivePath := paneArchivePath(file)
	if _, err := os.Stat(archivePath); err != nil {
		// no archive: lookup always returns empty.
		return "", func(string, int, int) string { return "" }, nil
	}

	contentDir, err := os.MkdirTemp("", "tmux-restore-*")
	if err != nil {
		return "", nil, sendError(ctx, ch, "creating temp dir: %w", err)
	}
	if err := ExtractPaneArchive(archivePath, contentDir); err != nil {
		_ = os.RemoveAll(contentDir)
		return "", nil, sendError(ctx, ch, "extracting pane archive: %w", err)
	}

	defaultCmd := restoreDeps.DefaultCommand(cfg.SocketPath)
	tmuxCmd := tmuxCommandPath()

	lookup := func(sessName string, winIdx, paneIdx int) string {
		paneKey := fmt.Sprintf("%s:%d.%d", sessName, winIdx, paneIdx)
		path := filepath.Join(contentDir, paneKey)
		if _, statErr := os.Stat(path); statErr == nil {
			return paneStartupCommand(path, paneReplayWaitChannel(sessName, winIdx, paneIdx), defaultCmd, tmuxCmd, cfg.SocketPath)
		}
		return ""
	}
	return contentDir, lookup, nil
}

// scheduleContentCleanup removes the extracted pane-content temp dir after a
// short delay so the asynchronous pane startup commands have time to read the
// files. A "" dir is a no-op.
func scheduleContentCleanup(contentDir string) {
	if contentDir == "" {
		return
	}
	go func() {
		time.Sleep(5 * time.Second)
		_ = os.RemoveAll(contentDir)
	}()
}

// restoreSession restores (or merges, or skips) a single saved session,
// emitting the same progress-event sequence as the original monolithic loop.
func (r *restoreRun) restoreSession(sess Session, merge bool) error {
	// indexMap translates saved window indices to actual target indices. For
	// new sessions, indices are used as-is. For merges, saved windows are
	// appended after the highest existing index.
	indexMap, skipped, err := r.createOrMergeSession(sess, merge)
	if err != nil || skipped {
		return err
	}

	replayWaitChannels, err := r.restoreWindows(sess, indexMap, merge)
	if err != nil {
		return err
	}

	paneChannels, err := r.splitPanes(sess, indexMap)
	if err != nil {
		return err
	}
	replayWaitChannels = append(replayWaitChannels, paneChannels...)

	return r.finalizeSession(sess, indexMap, replayWaitChannels)
}

// createOrMergeSession handles the session-creation / merge / skip header. It
// returns the window index map, whether the session was skipped (already
// restored), and any error.
func (r *restoreRun) createOrMergeSession(sess Session, merge bool) (map[int]int, bool, error) {
	indexMap := make(map[int]int, len(sess.Windows))

	if !merge {
		for _, win := range sess.Windows {
			indexMap[win.Index] = win.Index
		}

		r.step++
		if !r.emit(ProgressEvent{
			Step:    r.step,
			Message: fmt.Sprintf("restoring session %s...", sess.Name),
			Kind:    "session",
			ID:      sess.Name,
		}) {
			return nil, false, r.ctx.Err()
		}
		// create the session with $HOME as the working directory so that new
		// windows inherit the correct default. we must pass -c explicitly
		// because tmux otherwise inherits the cwd of whatever process
		// (control-mode client, popup, etc.) sends the new-session command.
		sessionDir := os.Getenv("HOME")
		if err := restoreDeps.CreateSession(tmux.SessionSpec{
			SocketPath: r.cfg.SocketPath,
			Name:       sess.Name,
			Dir:        sessionDir,
		}); err != nil {
			return nil, false, sendError(r.ctx, r.ch, "creating session %s: %w", sess.Name, err)
		}

		// the first pane is auto-created with the session; respawn it in the
		// correct working directory with any startup command. this avoids
		// polluting session_path with a pane-specific dir.
		if len(sess.Windows) > 0 && len(sess.Windows[0].Panes) > 0 {
			p0 := sess.Windows[0].Panes[0]
			paneCmd := r.lookupPaneCmd(sess.Name, sess.Windows[0].Index, p0.Index)
			if p0.WorkingDir != "" || paneCmd != "" {
				paneTarget := fmt.Sprintf("%s:0.0", sess.Name)
				if err := restoreDeps.RespawnPane(tmux.PaneSpec{
					SocketPath: r.cfg.SocketPath,
					Target:     paneTarget,
					Dir:        p0.WorkingDir,
					Command:    paneCmd,
				}); err != nil {
					return nil, false, sendError(r.ctx, r.ch, "respawning pane %s: %w", paneTarget, err)
				}
			}
		}
		return indexMap, false, nil
	}

	// merge path: idempotency — skip if this slot was already merged.
	markerKey := restoreMarkerKey(sess.Name)
	if restoreDeps.SessionOption(r.cfg.SocketPath, sess.Name, markerKey) != "" {
		r.step += sessionStepCount(sess)
		if !r.emit(ProgressEvent{
			Step:    r.step,
			Message: fmt.Sprintf("skipping session %s (already restored)", sess.Name),
			Kind:    "info",
			ID:      sess.Name,
		}) {
			return nil, true, r.ctx.Err()
		}
		return nil, true, nil
	}

	existingIndices, err := restoreDeps.ExistingWindowIndices(r.cfg.SocketPath, sess.Name)
	if err != nil {
		return nil, false, sendError(r.ctx, r.ch, "listing windows for session %s: %w", sess.Name, err)
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

	r.step++
	if !r.emit(ProgressEvent{
		Step:    r.step,
		Message: fmt.Sprintf("merging into session %s...", sess.Name),
		Kind:    "session",
		ID:      sess.Name,
	}) {
		return nil, false, r.ctx.Err()
	}
	return indexMap, false, nil
}

// restoreWindows creates (or renames) the windows for a session and returns the
// pane-replay wait channels accumulated from the first pane of each window.
func (r *restoreRun) restoreWindows(sess Session, indexMap map[int]int, merge bool) ([]string, error) {
	var replayWaitChannels []string
	var winIDs []string
	for _, win := range sess.Windows {
		targetIdx := indexMap[win.Index]
		winTarget := fmt.Sprintf("%s:%d", sess.Name, targetIdx)
		winIDs = append(winIDs, winTarget)

		if !merge && win.Index == 0 {
			// first window of a new session is auto-created; rename it
			if err := restoreDeps.RenameWindow(r.cfg.SocketPath, winTarget, win.Name); err != nil {
				return nil, sendError(r.ctx, r.ch, "renaming window %s: %w", winTarget, err)
			}
		} else {
			winDir := ""
			winCmd := ""
			if len(win.Panes) > 0 {
				winDir = win.Panes[0].WorkingDir
				winCmd = r.lookupPaneCmd(sess.Name, win.Index, win.Panes[0].Index)
				if winCmd != "" {
					replayWaitChannels = append(replayWaitChannels, paneReplayWaitChannel(sess.Name, win.Index, win.Panes[0].Index))
				}
			}
			if err := restoreDeps.CreateWindow(tmux.WindowSpec{
				SocketPath: r.cfg.SocketPath,
				Session:    sess.Name,
				Index:      targetIdx,
				Name:       win.Name,
				Dir:        winDir,
				Command:    winCmd,
			}); err != nil {
				return nil, sendError(r.ctx, r.ch, "creating window %s: %w", winTarget, err)
			}
		}
	}
	if len(winIDs) > 0 {
		r.step += len(winIDs)
		if !r.emit(ProgressEvent{
			Step:    r.step,
			Message: fmt.Sprintf("restoring windows for session %s: %s", sess.Name, strings.Join(winIDs, " ")),
			Kind:    "window",
		}) {
			return nil, r.ctx.Err()
		}
	}
	return replayWaitChannels, nil
}

// splitPanes creates the non-first panes for each window via split-window and
// returns the pane-replay wait channels for panes with saved content.
func (r *restoreRun) splitPanes(sess Session, indexMap map[int]int) ([]string, error) {
	var replayWaitChannels []string
	var paneIDs []string
	for _, win := range sess.Windows {
		targetIdx := indexMap[win.Index]
		for _, pane := range win.Panes {
			if pane.Index == 0 {
				continue
			}
			paneTarget := fmt.Sprintf("%s:%d", sess.Name, targetIdx)
			paneCmd := r.lookupPaneCmd(sess.Name, win.Index, pane.Index)
			if paneCmd != "" {
				replayWaitChannels = append(replayWaitChannels, paneReplayWaitChannel(sess.Name, win.Index, pane.Index))
			}
			if err := restoreDeps.SplitPane(tmux.PaneSpec{
				SocketPath: r.cfg.SocketPath,
				Target:     paneTarget,
				Dir:        pane.WorkingDir,
				Command:    paneCmd,
			}); err != nil {
				return nil, sendError(r.ctx, r.ch, "splitting pane %s.%d: %w", paneTarget, pane.Index, err)
			}
			paneIDs = append(paneIDs, fmt.Sprintf("%s.%d", paneTarget, pane.Index))
		}
	}
	if len(paneIDs) > 0 {
		r.step += len(paneIDs)
		if !r.emit(ProgressEvent{
			Step:    r.step,
			Message: fmt.Sprintf("splitting panes for session %s: %s", sess.Name, strings.Join(paneIDs, " ")),
			Kind:    "pane",
		}) {
			return nil, r.ctx.Err()
		}
	}
	return replayWaitChannels, nil
}

// finalizeSession applies layouts, waits for pane replays, selects active panes
// and the active window, emits the finalize event, and records the idempotency
// marker.
func (r *restoreRun) finalizeSession(sess Session, indexMap map[int]int, replayWaitChannels []string) error {
	for _, win := range sess.Windows {
		targetIdx := indexMap[win.Index]
		winTarget := fmt.Sprintf("%s:%d", sess.Name, targetIdx)
		r.step++
		if err := restoreDeps.SelectLayoutTarget(r.cfg.SocketPath, winTarget, selectableLayout(win.Layout)); err != nil {
			return sendError(r.ctx, r.ch, "applying layout for %s: %w", winTarget, err)
		}
	}

	for _, channel := range replayWaitChannels {
		if err := restoreDeps.WaitFor(r.ctx, r.cfg.SocketPath, channel); err != nil {
			return sendError(r.ctx, r.ch, "waiting for pane replay %s: %w", channel, err)
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
		r.step++
		if err := restoreDeps.SelectPane(r.cfg.SocketPath, paneTarget); err != nil {
			return sendError(r.ctx, r.ch, "selecting active pane %s: %w", paneTarget, err)
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
	r.step++
	if err := restoreDeps.SelectWindow(r.cfg.SocketPath, activeWindowTarget); err != nil {
		return sendError(r.ctx, r.ch, "selecting active window %s: %w", activeWindowTarget, err)
	}

	if !r.emit(ProgressEvent{
		Step:    r.step,
		Message: fmt.Sprintf("finalizing session %s...", sess.Name),
		Kind:    "info",
	}) {
		return r.ctx.Err()
	}

	// mark restored sessions so re-running the same restore is idempotent
	markerKey := restoreMarkerKey(sess.Name)
	if err := restoreDeps.SetSessionOption(r.cfg.SocketPath, sess.Name, markerKey, "1"); err != nil {
		return sendError(r.ctx, r.ch, "setting restore marker for session %s: %w", sess.Name, err)
	}
	return nil
}

// switchClient restores the saved client session and emits the final
// pre-done progress event.
func (r *restoreRun) switchClient(sf *SaveFile) error {
	r.step++
	if sf.ClientSession != "" {
		if err := restoreDeps.SwitchClient(r.cfg.SocketPath, r.cfg.ClientID, sf.ClientSession); err != nil {
			return sendError(r.ctx, r.ch, "switching client to session %s: %w", sf.ClientSession, err)
		}
	}
	if !r.emit(ProgressEvent{
		Step:    r.step,
		Message: fmt.Sprintf("switched client to session %s %s", sf.ClientSession, greenCheck),
		Kind:    "info",
	}) {
		return r.ctx.Err()
	}
	return nil
}

// sessionStepCount returns the number of progress steps a single session
// contributes to a restore. It is the single source of truth shared by
// computeRestoreTotal and the skip-already-merged branch so the two can never
// drift out of lockstep.
//
// Per session the steps are:
//   - 1   create or skip the session
//   - per window: 1 create/rename window + 1 select-layout + one per non-first pane (split-window)
//   - per window: 1 select active pane
//   - 1   select active window
func sessionStepCount(sess Session) int {
	steps := 1 // create or skip session
	for _, win := range sess.Windows {
		steps++ // create or rename window
		for _, pane := range win.Panes {
			if pane.Index != 0 {
				steps++ // split-window
			}
		}
		steps++ // select-layout
	}
	steps += len(sess.Windows) // select active pane per window
	steps++                    // select active window
	return steps
}

// computeRestoreTotal computes the total number of work units for a restore.
// Pane content restore is handled at creation time (via startup commands),
// so there are no separate send-pane-contents steps.
func computeRestoreTotal(sf *SaveFile) int {
	total := 0
	for _, sess := range sf.Sessions {
		total += sessionStepCount(sess)
	}

	total++ // switch client

	return total
}
