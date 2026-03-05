package gotmuxcc

import (
	"strconv"
)

func (q *query) serverVars() *query {
	return q.vars(
		varPid,
		varSocketPath,
		varStartTime,
		varUid,
		varUser,
		varVersion,
	)
}

func (q queryResult) toServer(t *Tmux) *Server {
	pid, _ := strconv.Atoi(q.get(varPid))
	socketPath := q.get(varSocketPath)
	socket, _ := newSocket(socketPath)
	startTime := q.get(varStartTime)
	uid := q.get(varUid)
	user := q.get(varUser)
	version := q.get(varVersion)

	return &Server{
		Pid:       int32(pid),
		Socket:    socket,
		StartTime: startTime,
		Uid:       uid,
		User:      user,
		Version:   version,
		tmux:      t,
	}
}

// GetServerInformation retrieves tmux server details.
func (t *Tmux) GetServerInformation() (*Server, error) {
	output, err := t.query().
		cmd("display-message").
		serverVars().
		run()
	if err != nil {
		return nil, err
	}

	server := output.one().toServer(t)
	return server, nil
}
