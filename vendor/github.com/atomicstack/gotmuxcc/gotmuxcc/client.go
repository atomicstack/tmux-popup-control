package gotmuxcc

import "fmt"

func (q *query) clientVars() *query {
	return q.vars(
		varClientActivity,
		varClientCellHeight,
		varClientCellWidth,
		varClientControlMode,
		varClientCreated,
		varClientDiscarded,
		varClientFlags,
		varClientHeight,
		varClientKeyTable,
		varClientLastSession,
		varClientName,
		varClientPid,
		varClientPrefix,
		varClientReadonly,
		varClientSession,
		varClientTermname,
		varClientTermfeatures,
		varClientTermtype,
		varClientTty,
		varClientUid,
		varClientUser,
		varClientUtf8,
		varClientWidth,
		varClientWritten,
	)
}

func (q queryResult) toClient(t *Tmux) *Client {
	client := &Client{
		Activity:     q.get(varClientActivity),
		CellHeight:   atoi(q.get(varClientCellHeight)),
		CellWidth:    atoi(q.get(varClientCellWidth)),
		ControlMode:  isOne(q.get(varClientControlMode)),
		Created:      q.get(varClientCreated),
		Discarded:    q.get(varClientDiscarded),
		Flags:        q.get(varClientFlags),
		Height:       atoi(q.get(varClientHeight)),
		KeyTable:     q.get(varClientKeyTable),
		LastSession:  q.get(varClientLastSession),
		Name:         q.get(varClientName),
		Pid:          atoi32(q.get(varClientPid)),
		Prefix:       isOne(q.get(varClientPrefix)),
		Readonly:     isOne(q.get(varClientReadonly)),
		Session:      q.get(varClientSession),
		Termname:     q.get(varClientTermname),
		Termfeatures: q.get(varClientTermfeatures),
		Termtype:     q.get(varClientTermtype),
		Tty:          q.get(varClientTty),
		Uid:          atoi32(q.get(varClientUid)),
		User:         q.get(varClientUser),
		Utf8:         isOne(q.get(varClientUtf8)),
		Width:        atoi(q.get(varClientWidth)),
		Written:      q.get(varClientWritten),
		tmux:         t,
	}

	return client
}

// ListClients enumerates tmux clients.
func (t *Tmux) ListClients() ([]*Client, error) {
	output, err := t.query().
		cmd("list-clients").
		clientVars().
		run()
	if err != nil {
		return nil, fmt.Errorf("failed to list clients: %w", err)
	}

	results := output.collect()
	clients := make([]*Client, 0, len(results))
	for _, entry := range results {
		clients = append(clients, entry.toClient(t))
	}

	return clients, nil
}

// GetClientByTty retrieves a client by tty identifier.
func (t *Tmux) GetClientByTty(tty string) (*Client, error) {
	clients, err := t.ListClients()
	if err != nil {
		return nil, err
	}
	for _, client := range clients {
		if client.Tty == tty {
			return client, nil
		}
	}
	return nil, nil
}

// GetSession retrieves the session this client is attached to.
func (c *Client) GetSession() (*Session, error) {
	return c.tmux.GetSessionByName(c.Session)
}
