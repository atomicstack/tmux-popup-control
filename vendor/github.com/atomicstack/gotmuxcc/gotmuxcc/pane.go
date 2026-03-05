package gotmuxcc

import "fmt"

func (q *query) paneVars() *query {
	return q.vars(
		varPaneActive,
		varPaneAtBottom,
		varPaneAtLeft,
		varPaneAtRight,
		varPaneAtTop,
		varPaneBg,
		varPaneBottom,
		varPaneCurrentCommand,
		varPaneCurrentPath,
		varPaneDead,
		varPaneDeadSignal,
		varPaneDeadStatus,
		varPaneDeadTime,
		varPaneFg,
		varPaneFormat,
		varPaneHeight,
		varPaneId,
		varPaneInMode,
		varPaneIndex,
		varPaneInputOff,
		varPaneLast,
		varPaneLeft,
		varPaneMarked,
		varPaneMarkedSet,
		varPaneMode,
		varPanePath,
		varPanePid,
		varPanePipe,
		varPaneRight,
		varPaneSearchString,
		varPaneSessionName,
		varPaneStartCommand,
		varPaneStartPath,
		varPaneSynchronized,
		varPaneTabs,
		varPaneTitle,
		varPaneTop,
		varPaneTty,
		varPaneUnseenChanges,
		varPaneWidth,
		varPaneWindowIndex,
	)
}

// CapturePane runs the tmux capture-pane command against the provided target.
func (t *Tmux) CapturePane(target string, op *CaptureOptions) (string, error) {
	q := t.query().cmd("capture-pane")
	if target != "" {
		q.fargs("-t", target)
	}
	q.fargs("-p")

	if op != nil {
		if op.EscTxtNBgAttr {
			q.fargs("-e")
		}
		if op.EscNonPrintables {
			q.fargs("-C")
		}
		if op.IgnoreTrailing {
			q.fargs("-T")
		}
		if op.PreserveTrailing {
			q.fargs("-N")
		}
		if op.PreserveAndJoin {
			q.fargs("-J")
		}
		if op.StartLine != "" {
			q.fargs("-S", op.StartLine)
		}
		if op.EndLine != "" {
			q.fargs("-E", op.EndLine)
		}
	}

	output, err := q.run()
	if err != nil {
		return "", fmt.Errorf("failed to capture pane: %w", err)
	}

	return output.raw(), nil
}

func (r queryResult) toPane(t *Tmux) *Pane {
	return &Pane{
		Active:         isOne(r.get(varPaneActive)),
		AtBottom:       isOne(r.get(varPaneAtBottom)),
		AtLeft:         isOne(r.get(varPaneAtLeft)),
		AtRight:        isOne(r.get(varPaneAtRight)),
		AtTop:          isOne(r.get(varPaneAtTop)),
		Bg:             r.get(varPaneBg),
		Bottom:         r.get(varPaneBottom),
		CurrentCommand: r.get(varPaneCurrentCommand),
		CurrentPath:    r.get(varPaneCurrentPath),
		Dead:           isOne(r.get(varPaneDead)),
		DeadSignal:     atoi(r.get(varPaneDeadSignal)),
		DeadStatus:     atoi(r.get(varPaneDeadStatus)),
		DeadTime:       r.get(varPaneDeadTime),
		Fg:             r.get(varPaneFg),
		Format:         isOne(r.get(varPaneFormat)),
		Height:         atoi(r.get(varPaneHeight)),
		Id:             r.get(varPaneId),
		InMode:         isOne(r.get(varPaneInMode)),
		Index:          atoi(r.get(varPaneIndex)),
		InputOff:       isOne(r.get(varPaneInputOff)),
		Last:           isOne(r.get(varPaneLast)),
		Left:           r.get(varPaneLeft),
		Marked:         isOne(r.get(varPaneMarked)),
		MarkedSet:      isOne(r.get(varPaneMarkedSet)),
		Mode:           r.get(varPaneMode),
		Path:           r.get(varPanePath),
		Pid:            atoi32(r.get(varPanePid)),
		Pipe:           isOne(r.get(varPanePipe)),
		Right:          r.get(varPaneRight),
		SearchString:   r.get(varPaneSearchString),
		SessionName:    r.get(varPaneSessionName),
		StartCommand:   r.get(varPaneStartCommand),
		StartPath:      r.get(varPaneStartPath),
		Synchronized:   isOne(r.get(varPaneSynchronized)),
		Tabs:           r.get(varPaneTabs),
		Title:          r.get(varPaneTitle),
		Top:            r.get(varPaneTop),
		Tty:            r.get(varPaneTty),
		UnseenChanges:  isOne(r.get(varPaneUnseenChanges)),
		Width:          atoi(r.get(varPaneWidth)),
		WindowIndex:    atoi(r.get(varPaneWindowIndex)),
		tmux:           t,
	}
}

// ListPanes lists panes within a session.
func (s *Session) ListPanes() ([]*Pane, error) {
	output, err := s.tmux.query().
		cmd("list-panes").
		fargs("-s", "-t", s.Name).
		paneVars().
		run()
	if err != nil {
		return nil, fmt.Errorf("failed to list panes: %w", err)
	}

	results := output.collect()
	panes := make([]*Pane, 0, len(results))
	for _, result := range results {
		panes = append(panes, result.toPane(s.tmux))
	}
	return panes, nil
}

// GetPaneByIndex returns a pane within a window by index.
func (w *Window) GetPaneByIndex(idx int) (*Pane, error) {
	panes, err := w.ListPanes()
	if err != nil {
		return nil, fmt.Errorf("failed to get pane by index: %w", err)
	}
	for _, pane := range panes {
		if pane.Index == idx {
			return pane, nil
		}
	}
	return nil, nil
}

// SendKeys sends keys to the pane.
func (p *Pane) SendKeys(line string) error {
	_, err := p.tmux.query().
		cmd("send-keys").
		fargs("-t", p.Id).
		pargs(line).
		run()
	if err != nil {
		return fmt.Errorf("failed to send keys: %w", err)
	}
	return nil
}

// Kill terminates the pane.
func (p *Pane) Kill() error {
	_, err := p.tmux.query().
		cmd("kill-pane").
		fargs("-t", p.Id).
		run()
	if err != nil {
		return fmt.Errorf("failed to kill pane: %w", err)
	}
	return nil
}

// SelectPane selects the pane with optional target position.
func (p *Pane) SelectPane(op *SelectPaneOptions) error {
	q := p.tmux.query().
		cmd("select-pane").
		fargs("-t", p.Id)

	if op != nil && op.TargetPosition != "" {
		q.fargs(string(op.TargetPosition))
	}

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to select pane: %w", err)
	}
	return nil
}

// Select selects the pane with defaults.
func (p *Pane) Select() error {
	return p.SelectPane(nil)
}

// SplitWindow splits the pane into another pane.
func (p *Pane) SplitWindow(op *SplitWindowOptions) error {
	q := p.tmux.query().
		cmd("split-window").
		fargs("-t", p.Id)

	if op != nil {
		if op.SplitDirection != "" {
			q.fargs(string(op.SplitDirection))
		}
		if op.StartDirectory != "" {
			q.fargs("-c", op.StartDirectory)
		}
		if op.ShellCommand != "" {
			q.pargs(fmt.Sprintf("'%s'", op.ShellCommand))
		}
	}

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to split pane: %w", err)
	}
	return nil
}

// Split splits with default options.
func (p *Pane) Split() error {
	return p.SplitWindow(nil)
}

// ChooseTree enters choose-tree mode for this pane.
func (p *Pane) ChooseTree(op *ChooseTreeOptions) error {
	q := p.tmux.query().
		cmd("choose-tree").
		fargs("-t", p.Id)

	if op != nil {
		if op.SessionsCollapsed {
			q.fargs("-s")
		}
		if op.WindowsCollapsed {
			q.fargs("-w")
		}
	}

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to enter choose-tree: %w", err)
	}
	return nil
}

// Rename sets the pane's title via select-pane -T.
func (p *Pane) Rename(title string) error {
	_, err := p.tmux.query().
		cmd("select-pane").
		fargs("-t", p.Id, "-T", title).
		run()
	if err != nil {
		return fmt.Errorf("failed to rename pane: %w", err)
	}
	return nil
}

// Swap swaps this pane with another pane.
func (p *Pane) Swap(target string) error {
	_, err := p.tmux.query().
		cmd("swap-pane").
		fargs("-s", p.Id, "-t", target).
		run()
	if err != nil {
		return fmt.Errorf("failed to swap pane: %w", err)
	}
	return nil
}

// Move moves this pane to another target (window or pane position).
// If target is empty, tmux uses the default target.
func (p *Pane) Move(target string) error {
	q := p.tmux.query().
		cmd("move-pane").
		fargs("-s", p.Id)
	if target != "" {
		q.fargs("-t", target)
	}
	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to move pane: %w", err)
	}
	return nil
}

// Break breaks this pane out into a new window.
// If destination is empty, the pane becomes a window in the current session.
func (p *Pane) Break(destination string) error {
	q := p.tmux.query().
		cmd("break-pane").
		fargs("-s", p.Id)
	if destination != "" {
		q.fargs("-t", destination)
	}
	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to break pane: %w", err)
	}
	return nil
}

// Join moves this pane into the target window as a split.
// This is equivalent to join-pane -s <this> -t <target>.
func (p *Pane) Join(target string) error {
	_, err := p.tmux.query().
		cmd("join-pane").
		fargs("-s", p.Id, "-t", target).
		run()
	if err != nil {
		return fmt.Errorf("failed to join pane: %w", err)
	}
	return nil
}

// Resize resizes the pane in the given direction by amount cells.
func (p *Pane) Resize(direction ResizeDirection, amount int) error {
	_, err := p.tmux.query().
		cmd("resize-pane").
		fargs("-t", p.Id, string(direction), fmt.Sprintf("%d", amount)).
		run()
	if err != nil {
		return fmt.Errorf("failed to resize pane: %w", err)
	}
	return nil
}

// CapturePane captures pane output with options.
func (p *Pane) CapturePane(op *CaptureOptions) (string, error) {
	return p.tmux.CapturePane(p.Id, op)
}

// Capture captures pane output with default escapes.
func (p *Pane) Capture() (string, error) {
	return p.tmux.CapturePane(p.Id, &CaptureOptions{EscTxtNBgAttr: true})
}

// SetOption sets a pane-scoped option.
func (p *Pane) SetOption(key, value string) error {
	return p.tmux.SetOption(p.Id, key, value, "-p")
}

// Option retrieves a pane option.
func (p *Pane) Option(key string) (*Option, error) {
	return p.tmux.Option(p.Id, key, "-p")
}

// Options lists all pane options.
func (p *Pane) Options() ([]*Option, error) {
	return p.tmux.Options(p.Id, "-p")
}

// DeleteOption removes a pane option.
func (p *Pane) DeleteOption(key string) error {
	return p.tmux.DeleteOption(p.Id, key, "-p")
}
