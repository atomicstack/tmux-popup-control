package gotmuxcc

import (
	"errors"
	"fmt"
	"strings"
)

func (q *query) windowVars() *query {
	return q.vars(
		varWindowActive,
		varWindowActiveClients,
		varWindowActiveClientsList,
		varWindowActiveSessions,
		varWindowActiveSessionsList,
		varWindowActivity,
		varWindowActivityFlag,
		varWindowBellFlag,
		varWindowBigger,
		varWindowCellHeight,
		varWindowCellWidth,
		varWindowEndFlag,
		varWindowFlags,
		varWindowFormat,
		varWindowHeight,
		varWindowId,
		varWindowIndex,
		varWindowLastFlag,
		varWindowLayout,
		varWindowLinked,
		varWindowLinkedSessions,
		varWindowLinkedSessionsList,
		varWindowMarkedFlag,
		varWindowName,
		varWindowOffsetX,
		varWindowOffsetY,
		varWindowPanes,
		varWindowRawFlags,
		varWindowSilenceFlag,
		varWindowStackIndex,
		varWindowStartFlag,
		varWindowVisibleLayout,
		varWindowWidth,
		varWindowZoomedFlag,
		varSessionName,
	)
}

func (r queryResult) toWindow(t *Tmux) *Window {
	window := &Window{
		Active:             isOne(r.get(varWindowActive)),
		ActiveClients:      atoi(r.get(varWindowActiveClients)),
		ActiveClientsList:  parseList(r.get(varWindowActiveClientsList)),
		ActiveSessions:     atoi(r.get(varWindowActiveSessions)),
		ActiveSessionsList: parseList(r.get(varWindowActiveSessionsList)),
		Activity:           r.get(varWindowActivity),
		ActivityFlag:       isOne(r.get(varWindowActivityFlag)),
		BellFlag:           isOne(r.get(varWindowBellFlag)),
		Bigger:             isOne(r.get(varWindowBigger)),
		CellHeight:         atoi(r.get(varWindowCellHeight)),
		CellWidth:          atoi(r.get(varWindowCellWidth)),
		EndFlag:            isOne(r.get(varWindowEndFlag)),
		Flags:              r.get(varWindowFlags),
		Format:             isOne(r.get(varWindowFormat)),
		Height:             atoi(r.get(varWindowHeight)),
		Id:                 r.get(varWindowId),
		Index:              atoi(r.get(varWindowIndex)),
		LastFlag:           isOne(r.get(varWindowLastFlag)),
		Layout:             r.get(varWindowLayout),
		Linked:             isOne(r.get(varWindowLinked)),
		LinkedSessions:     atoi(r.get(varWindowLinkedSessions)),
		LinkedSessionsList: parseList(r.get(varWindowLinkedSessionsList)),
		MarkedFlag:         isOne(r.get(varWindowMarkedFlag)),
		Name:               r.get(varWindowName),
		Session:            r.get(varSessionName),
		OffsetX:            atoi(r.get(varWindowOffsetX)),
		OffsetY:            atoi(r.get(varWindowOffsetY)),
		Panes:              atoi(r.get(varWindowPanes)),
		RawFlags:           r.get(varWindowRawFlags),
		SilenceFlag:        atoi(r.get(varWindowSilenceFlag)),
		StackIndex:         atoi(r.get(varWindowStackIndex)),
		StartFlag:          isOne(r.get(varWindowStartFlag)),
		VisibleLayout:      r.get(varWindowVisibleLayout),
		Width:              atoi(r.get(varWindowWidth)),
		ZoomedFlag:         isOne(r.get(varWindowZoomedFlag)),
		tmux:               t,
	}
	return window
}

// ListPanes returns the panes in this window.
func (w *Window) ListPanes() ([]*Pane, error) {
	output, err := w.tmux.query().
		cmd("list-panes").
		fargs("-t", w.Id).
		paneVars().
		run()
	if err != nil {
		return nil, fmt.Errorf("failed to list panes: %w", err)
	}

	panes := make([]*Pane, 0)
	for _, result := range output.collect() {
		panes = append(panes, result.toPane(w.tmux))
	}
	return panes, nil
}

// Kill terminates the window.
func (w *Window) Kill() error {
	_, err := w.tmux.query().
		cmd("kill-window").
		fargs("-t", w.Id).
		run()
	if err != nil {
		return fmt.Errorf("failed to kill window: %w", err)
	}
	return nil
}

// Rename changes the window's name.
func (w *Window) Rename(newName string) error {
	_, err := w.tmux.query().
		cmd("rename-window").
		fargs("-t", w.Id).
		pargs(newName).
		run()
	if err != nil {
		return fmt.Errorf("failed to rename window: %w", err)
	}
	return nil
}

// Select activates this window.
func (w *Window) Select() error {
	_, err := w.tmux.query().
		cmd("select-window").
		fargs("-t", w.Id).
		run()
	if err != nil {
		return fmt.Errorf("failed to select window: %w", err)
	}
	return nil
}

// SelectLayout changes window layout.
func (w *Window) SelectLayout(layout WindowLayout) error {
	_, err := w.tmux.query().
		cmd("select-layout").
		fargs("-t", w.Id).
		pargs(string(layout)).
		run()
	if err != nil {
		return fmt.Errorf("failed to select layout: %w", err)
	}
	return nil
}

// Move moves this window to another session/index.
func (w *Window) Move(targetSession string, targetIdx int) error {
	_, err := w.tmux.query().
		cmd("move-window").
		fargs("-s", w.Id).
		fargs("-t", fmt.Sprintf("%s:%d", targetSession, targetIdx)).
		run()
	if err != nil {
		return fmt.Errorf("failed to move window: %w", err)
	}
	return nil
}

// Unlink unlinks this window from the session. If the window is linked to
// multiple sessions it is removed from the current one; -k kills it even
// if it is the last link.
func (w *Window) Unlink() error {
	_, err := w.tmux.query().
		cmd("unlink-window").
		fargs("-k", "-t", w.Id).
		run()
	if err != nil {
		return fmt.Errorf("failed to unlink window: %w", err)
	}
	return nil
}

// Link links this window into targetSession, appending it after the last
// window in that session.
func (w *Window) Link(targetSession string) error {
	_, err := w.tmux.query().
		cmd("link-window").
		fargs("-a", "-s", w.Id, "-t", targetSession).
		run()
	if err != nil {
		return fmt.Errorf("failed to link window: %w", err)
	}
	return nil
}

// MoveToSession moves this window to targetSession, appending it after
// the last window. Unlike Move(), this uses -a for append semantics.
func (w *Window) MoveToSession(targetSession string) error {
	_, err := w.tmux.query().
		cmd("move-window").
		fargs("-a", "-s", w.Id, "-t", targetSession).
		run()
	if err != nil {
		return fmt.Errorf("failed to move window to session: %w", err)
	}
	return nil
}

// Swap swaps this window with another window.
func (w *Window) Swap(target string) error {
	_, err := w.tmux.query().
		cmd("swap-window").
		fargs("-s", w.Id, "-t", target).
		run()
	if err != nil {
		return fmt.Errorf("failed to swap window: %w", err)
	}
	return nil
}

// ListLinkedSessions returns sessions linked to this window.
func (w *Window) ListLinkedSessions() ([]*Session, error) {
	sessions := make([]*Session, 0, len(w.LinkedSessionsList))
	for _, name := range w.LinkedSessionsList {
		session, err := w.tmux.GetSessionByName(name)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// ListActiveSessions returns sessions where the window is active.
func (w *Window) ListActiveSessions() ([]*Session, error) {
	sessions := make([]*Session, 0, len(w.ActiveSessionsList))
	for _, name := range w.ActiveSessionsList {
		session, err := w.tmux.GetSessionByName(name)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// ListActiveClients returns clients displaying this window.
func (w *Window) ListActiveClients() ([]*Client, error) {
	clients := make([]*Client, 0, len(w.ActiveClientsList))
	for _, tty := range w.ActiveClientsList {
		client, err := w.tmux.GetClientByTty(tty)
		if err != nil {
			return nil, err
		}
		clients = append(clients, client)
	}
	return clients, nil
}

// ListAllWindows lists all tmux windows across sessions.
func (t *Tmux) ListAllWindows() ([]*Window, error) {
	windows, directErr := t.listAllWindowsDirect()
	windowMap := make(map[string]*Window, len(windows))
	for _, w := range windows {
		windowMap[w.Id] = w
	}

	sessions, err := t.ListSessions()
	if err == nil {
		for _, session := range sessions {
			ws, serr := session.ListWindows()
			if serr != nil {
				continue
			}
			for _, w := range ws {
				if strings.TrimSpace(w.Session) == "" {
					w.Session = session.Name
				}
				if _, ok := windowMap[w.Id]; ok {
					continue
				}
				windowMap[w.Id] = w
				windows = append(windows, w)
			}
		}
	}

	if len(windows) == 0 {
		if directErr != nil {
			return nil, directErr
		}
	}
	return windows, nil
}

func (t *Tmux) listAllWindowsDirect() ([]*Window, error) {
	output, err := t.query().
		cmd("list-windows").
		fargs("-a").
		windowVars().
		run()
	if err != nil {
		return nil, fmt.Errorf("failed to list all windows: %w", err)
	}

	results := output.collect()
	windows := make([]*Window, 0, len(results))
	for _, result := range results {
		windows = append(windows, result.toWindow(t))
	}
	return windows, nil
}

// ListAllPanes lists all panes across sessions.
func (t *Tmux) ListAllPanes() ([]*Pane, error) {
	panes, directErr := t.listAllPanesDirect()
	paneMap := make(map[string]*Pane, len(panes))
	for _, p := range panes {
		paneMap[p.Id] = p
	}

	windows, err := t.ListAllWindows()
	if err == nil {
		for _, window := range windows {
			ps, serr := window.ListPanes()
			if serr != nil {
				continue
			}
			for _, p := range ps {
				if _, ok := paneMap[p.Id]; ok {
					continue
				}
				paneMap[p.Id] = p
				panes = append(panes, p)
			}
		}
	}

	if len(panes) == 0 {
		if directErr != nil {
			return nil, directErr
		}
	}
	return panes, nil
}

func (t *Tmux) listAllPanesDirect() ([]*Pane, error) {
	output, err := t.query().
		cmd("list-panes").
		fargs("-a").
		paneVars().
		run()
	if err != nil {
		return nil, fmt.Errorf("failed to list all panes: %w", err)
	}

	results := output.collect()
	panes := make([]*Pane, 0, len(results))
	for _, entry := range results {
		panes = append(panes, entry.toPane(t))
	}

	return panes, nil
}

// GetWindowById retrieves a window by its ID.
func (t *Tmux) GetWindowById(id string) (*Window, error) {
	windows, err := t.ListAllWindows()
	if err != nil {
		return nil, fmt.Errorf("failed to get window by id: %w", err)
	}

	for _, window := range windows {
		if window.Id == id {
			return window, nil
		}
	}

	return nil, nil
}

// GetPaneById retrieves a pane by its ID.
func (t *Tmux) GetPaneById(id string) (*Pane, error) {
	panes, err := t.ListAllPanes()
	if err != nil {
		return nil, fmt.Errorf("failed to get pane by id: %w", err)
	}

	for _, pane := range panes {
		if pane.Id == id {
			return pane, nil
		}
	}

	return nil, nil
}

// GetClient retrieves the first client in the server (compat helper).
func (t *Tmux) GetClient() (*Client, error) {
	clients, err := t.ListClients()
	if err != nil {
		return nil, err
	}
	if len(clients) == 0 {
		return nil, nil
	}
	return clients[0], nil
}

// ListWindows returns the windows belonging to a session.
func (s *Session) ListWindows() ([]*Window, error) {
	targets := []string{}
	if id := strings.TrimSpace(s.Id); id != "" {
		targets = append(targets, id)
	}
	if name := strings.TrimSpace(s.Name); name != "" && name != strings.TrimSpace(s.Id) {
		targets = append(targets, name)
	}

	var lastErr error
	for _, target := range targets {
		windows, err := s.listWindowsWithTarget(target)
		if err == nil {
			return windows, nil
		}
		lastErr = err
		var cmdErr *commandError
		if errors.As(err, &cmdErr) {
			continue
		}
		return nil, err
	}

	if lastErr != nil {
		var cmdErr *commandError
		if errors.As(lastErr, &cmdErr) {
			return []*Window{}, nil
		}
		return nil, lastErr
	}
	return []*Window{}, nil
}

func (s *Session) listWindowsWithTarget(target string) ([]*Window, error) {
	output, err := s.tmux.query().
		cmd("list-windows").
		fargs("-t", target).
		windowVars().
		run()
	if err != nil {
		return nil, fmt.Errorf("failed to list windows: %w", err)
	}

	results := output.collect()
	windows := make([]*Window, 0, len(results))
	for _, result := range results {
		windows = append(windows, result.toWindow(s.tmux))
	}
	return windows, nil
}

// GetWindowByName returns a window by its name within the session.
func (s *Session) GetWindowByName(name string) (*Window, error) {
	windows, err := s.ListWindows()
	if err != nil {
		return nil, fmt.Errorf("failed to get window by name: %w", err)
	}
	for _, window := range windows {
		if window.Name == name {
			return window, nil
		}
	}
	return nil, nil
}

// GetWindowByIndex returns a window by index within the session.
func (s *Session) GetWindowByIndex(idx int) (*Window, error) {
	windows, err := s.ListWindows()
	if err != nil {
		return nil, fmt.Errorf("failed to get window by index: %w", err)
	}
	for _, window := range windows {
		if window.Index == idx {
			return window, nil
		}
	}
	return nil, nil
}

// NewWindowOptions customise new-window behaviour.

// NewWindow creates a new window within the session.
func (s *Session) NewWindow(op *NewWindowOptions) (*Window, error) {
	q := s.tmux.query().
		cmd("new-window").
		fargs("-P", "-t", s.Name).
		windowVars()

	if op != nil {
		if op.StartDirectory != "" {
			q.fargs("-c", op.StartDirectory)
		}
		if op.WindowName != "" {
			q.fargs("-n", op.WindowName)
		}
		if op.DoNotAttach {
			q.fargs("-d")
		}
	}

	output, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}
	return output.one().toWindow(s.tmux), nil
}

// New creates a new window with default options.
func (s *Session) New() (*Window, error) {
	return s.NewWindow(nil)
}

// NextWindow selects the next window in the session.
func (s *Session) NextWindow() error {
	_, err := s.tmux.query().
		cmd("next-window").
		fargs("-t", s.Name).
		run()
	if err != nil {
		return fmt.Errorf("failed to select next window: %w", err)
	}
	return nil
}

// PreviousWindow selects the previous window in the session.
func (s *Session) PreviousWindow() error {
	_, err := s.tmux.query().
		cmd("previous-window").
		fargs("-t", s.Name).
		run()
	if err != nil {
		return fmt.Errorf("failed to select previous window: %w", err)
	}
	return nil
}

// UnlinkWindow unlinks a window by target string.
func (t *Tmux) UnlinkWindow(target string) error {
	_, err := t.query().
		cmd("unlink-window").
		fargs("-k", "-t", target).
		run()
	if err != nil {
		return fmt.Errorf("failed to unlink window: %w", err)
	}
	return nil
}

// LinkWindow links a source window into a target session, appending it.
func (t *Tmux) LinkWindow(source, targetSession string) error {
	_, err := t.query().
		cmd("link-window").
		fargs("-a", "-s", source, "-t", targetSession).
		run()
	if err != nil {
		return fmt.Errorf("failed to link window: %w", err)
	}
	return nil
}

// MoveWindowToSession moves a source window to a target session, appending it.
func (t *Tmux) MoveWindowToSession(source, targetSession string) error {
	_, err := t.query().
		cmd("move-window").
		fargs("-a", "-s", source, "-t", targetSession).
		run()
	if err != nil {
		return fmt.Errorf("failed to move window: %w", err)
	}
	return nil
}

// SwapWindows swaps two windows by target strings.
func (t *Tmux) SwapWindows(first, second string) error {
	_, err := t.query().
		cmd("swap-window").
		fargs("-s", first, "-t", second).
		run()
	if err != nil {
		return fmt.Errorf("failed to swap windows: %w", err)
	}
	return nil
}

// SwapPanes swaps two panes by target strings.
func (t *Tmux) SwapPanes(first, second string) error {
	_, err := t.query().
		cmd("swap-pane").
		fargs("-s", first, "-t", second).
		run()
	if err != nil {
		return fmt.Errorf("failed to swap panes: %w", err)
	}
	return nil
}

// MovePane moves a source pane to a target location.
func (t *Tmux) MovePane(source, target string) error {
	q := t.query().
		cmd("move-pane").
		fargs("-s", source)
	if target != "" {
		q.fargs("-t", target)
	}
	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to move pane: %w", err)
	}
	return nil
}

// BreakPane breaks a pane out into a new window.
func (t *Tmux) BreakPane(source, destination string) error {
	q := t.query().
		cmd("break-pane").
		fargs("-s", source)
	if destination != "" {
		q.fargs("-t", destination)
	}
	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to break pane: %w", err)
	}
	return nil
}

// JoinPane moves source pane into target window as a split.
func (t *Tmux) JoinPane(source, target string) error {
	_, err := t.query().
		cmd("join-pane").
		fargs("-s", source, "-t", target).
		run()
	if err != nil {
		return fmt.Errorf("failed to join pane: %w", err)
	}
	return nil
}

// ResizePane resizes a pane by target string.
func (t *Tmux) ResizePane(target string, direction ResizeDirection, amount int) error {
	_, err := t.query().
		cmd("resize-pane").
		fargs("-t", target, string(direction), fmt.Sprintf("%d", amount)).
		run()
	if err != nil {
		return fmt.Errorf("failed to resize pane: %w", err)
	}
	return nil
}

// RenamePane sets a pane's title via select-pane -T.
func (t *Tmux) RenamePane(target, title string) error {
	_, err := t.query().
		cmd("select-pane").
		fargs("-t", target, "-T", title).
		run()
	if err != nil {
		return fmt.Errorf("failed to rename pane: %w", err)
	}
	return nil
}

// SelectPane selects a pane by target string.
func (t *Tmux) SelectPane(target string) error {
	_, err := t.query().
		cmd("select-pane").
		fargs("-t", target).
		run()
	if err != nil {
		return fmt.Errorf("failed to select pane: %w", err)
	}
	return nil
}

// SelectWindow selects a window by target string.
func (t *Tmux) SelectWindow(target string) error {
	_, err := t.query().
		cmd("select-window").
		fargs("-t", target).
		run()
	if err != nil {
		return fmt.Errorf("failed to select window: %w", err)
	}
	return nil
}

// window-level option helpers leverage existing Option/SetOption functions.
func (w *Window) SetOption(key, value string) error {
	return w.tmux.SetOption(w.Id, key, value, "-w")
}

func (w *Window) Option(key string) (*Option, error) {
	return w.tmux.Option(w.Id, key, "-w")
}

func (w *Window) Options() ([]*Option, error) {
	return w.tmux.Options(w.Id, "-w")
}

func (w *Window) DeleteOption(key string) error {
	return w.tmux.DeleteOption(w.Id, key, "-w")
}
