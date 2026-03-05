package gotmuxcc

import (
	"errors"
	"fmt"
)

func (q *query) sessionVars() *query {
	return q.vars(
		varSessionActivity,
		varSessionAlerts,
		varSessionAttached,
		varSessionAttachedList,
		varSessionCreated,
		varSessionFormat,
		varSessionGroup,
		varSessionGroupAttached,
		varSessionGroupAttachedList,
		varSessionGroupList,
		varSessionGroupManyAttached,
		varSessionGroupSize,
		varSessionGrouped,
		varSessionId,
		varSessionLastAttached,
		varSessionManyAttached,
		varSessionMarked,
		varSessionName,
		varSessionPath,
		varSessionStack,
		varSessionWindows,
	)
}

func (r queryResult) toSession(t *Tmux) *Session {
	session := &Session{
		Activity:          r.get(varSessionActivity),
		Alerts:            r.get(varSessionAlerts),
		Attached:          atoi(r.get(varSessionAttached)),
		AttachedList:      parseList(r.get(varSessionAttachedList)),
		Created:           r.get(varSessionCreated),
		Format:            isOne(r.get(varSessionFormat)),
		Group:             r.get(varSessionGroup),
		GroupAttached:     atoi(r.get(varSessionGroupAttached)),
		GroupAttachedList: parseList(r.get(varSessionGroupAttachedList)),
		GroupList:         parseList(r.get(varSessionGroupList)),
		GroupManyAttached: isOne(r.get(varSessionGroupManyAttached)),
		GroupSize:         atoi(r.get(varSessionGroupSize)),
		Grouped:           isOne(r.get(varSessionGrouped)),
		Id:                r.get(varSessionId),
		LastAttached:      r.get(varSessionLastAttached),
		ManyAttached:      isOne(r.get(varSessionManyAttached)),
		Marked:            isOne(r.get(varSessionMarked)),
		Name:              r.get(varSessionName),
		Path:              r.get(varSessionPath),
		Stack:             r.get(varSessionStack),
		Windows:           atoi(r.get(varSessionWindows)),
		tmux:              t,
	}
	return session
}

// ListSessions returns all tmux sessions.
func (t *Tmux) ListSessions() ([]*Session, error) {
	output, err := t.query().
		cmd("list-sessions").
		sessionVars().
		run()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	results := output.collect()
	sessions := make([]*Session, 0, len(results))
	for _, item := range results {
		sessions = append(sessions, item.toSession(t))
	}

	return sessions, nil
}

// HasSession returns true if the session exists.
func (t *Tmux) HasSession(name string) bool {
	_, err := t.query().
		cmd("has-session").
		fargs("-t", name).
		run()
	return err == nil
}

// GetSessionByName retrieves a session by its name.
func (t *Tmux) GetSessionByName(name string) (*Session, error) {
	sessions, err := t.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to get session by name: %w", err)
	}

	for _, session := range sessions {
		if session.Name == name {
			return session, nil
		}
	}

	return nil, nil
}

// Session is an alias for GetSessionByName.
func (t *Tmux) Session(name string) (*Session, error) {
	return t.GetSessionByName(name)
}

// SessionOptions customises new session creation (defined in types.go).

// NewSession creates a new tmux session.
func (t *Tmux) NewSession(op *SessionOptions) (*Session, error) {
	q := t.query().
		cmd("new-session").
		fargs("-d", "-P").
		sessionVars()

	if op != nil {
		if op.Name != "" {
			if !checkSessionName(op.Name) {
				return nil, errors.New("invalid tmux session name")
			}
			q.fargs("-s", op.Name)
		}
		if op.StartDirectory != "" {
			q.fargs("-c", op.StartDirectory)
		}
		if op.Width != 0 {
			q.fargs("-x", fmt.Sprintf("%d", op.Width))
		}
		if op.Height != 0 {
			q.fargs("-y", fmt.Sprintf("%d", op.Height))
		}
		if op.ShellCommand != "" {
			q.pargs(fmt.Sprintf("'%s'", op.ShellCommand))
		}
	}

	output, err := q.run()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	session := output.one().toSession(t)
	return session, nil
}

// New creates a session with default options.
func (t *Tmux) New() (*Session, error) {
	return t.NewSession(nil)
}

// DetachClientOptions control detach behaviour (defined in types.go).

// DetachClient detaches the specified client or session.
func (t *Tmux) DetachClient(op *DetachClientOptions) error {
	q := t.query().
		cmd("detach-client")

	if op != nil {
		if op.TargetClient != "" {
			q.fargs("-t", op.TargetClient)
		} else if op.TargetSession != "" {
			q.fargs("-s", op.TargetSession)
		}
	}

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to detach client: %w", err)
	}

	return nil
}

// SwitchClientOptions control switch behaviour (defined in types.go).

// SwitchClient switches a client to a target session.
func (t *Tmux) SwitchClient(op *SwitchClientOptions) error {
	q := t.query().
		cmd("switch-client")

	if op != nil {
		if op.TargetClient != "" {
			q.fargs("-c", op.TargetClient)
		}
		if op.TargetSession != "" {
			q.fargs("-t", op.TargetSession)
		}
	}

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to switch client: %w", err)
	}

	return nil
}

// KillServer terminates the tmux server.
func (t *Tmux) KillServer() error {
	if _, err := t.query().cmd("kill-server").run(); err != nil {
		return fmt.Errorf("failed to kill server: %w", err)
	}
	return nil
}

// ListClients returns clients attached to this session.
func (s *Session) ListClients() ([]*Client, error) {
	clients, err := s.tmux.ListClients()
	if err != nil {
		return nil, err
	}

	filtered := make([]*Client, 0, len(clients))
	for _, client := range clients {
		if client.Session == s.Name {
			filtered = append(filtered, client)
		}
	}

	return filtered, nil
}

// AttachSessionOptions customise attachment (defined in types.go).

// AttachSession attaches the control client to this session.
func (s *Session) AttachSession(op *AttachSessionOptions) error {
	q := s.tmux.query().
		cmd("attach-session").
		fargs("-t", s.Name)

	if op != nil {
		if op.DetachClients {
			q.fargs("-d")
		}
		if op.WorkingDir != "" {
			q.fargs("-c", op.WorkingDir)
		}
	}

	if _, err := q.run(); err != nil {
		return fmt.Errorf("failed to attach session: %w", err)
	}
	return nil
}

// Attach attaches with default options.
func (s *Session) Attach() error {
	return s.AttachSession(nil)
}

// Detach detaches all clients from this session.
func (s *Session) Detach() error {
	_, err := s.tmux.query().
		cmd("detach-client").
		fargs("-s", s.Name).
		run()
	if err != nil {
		return fmt.Errorf("failed to detach session: %w", err)
	}
	return nil
}

// Kill terminates the session.
func (s *Session) Kill() error {
	_, err := s.tmux.query().
		cmd("kill-session").
		fargs("-t", s.Name).
		run()
	if err != nil {
		return fmt.Errorf("failed to kill session: %w", err)
	}
	return nil
}

// Rename renames the session.
func (s *Session) Rename(name string) error {
	_, err := s.tmux.query().
		cmd("rename-session").
		fargs("-t", s.Name).
		pargs(name).
		run()
	if err != nil {
		return fmt.Errorf("failed to rename session: %w", err)
	}
	return nil
}

// SetOption sets a session-scoped option.
func (s *Session) SetOption(key, value string) error {
	return s.tmux.SetOption(s.Name, key, value, "")
}

// Option retrieves a session option value.
func (s *Session) Option(key string) (*Option, error) {
	return s.tmux.Option(s.Name, key, "")
}

// Options lists all session options.
func (s *Session) Options() ([]*Option, error) {
	return s.tmux.Options(s.Name, "")
}

// DeleteOption removes a session option.
func (s *Session) DeleteOption(key string) error {
	return s.tmux.DeleteOption(s.Name, key, "")
}
