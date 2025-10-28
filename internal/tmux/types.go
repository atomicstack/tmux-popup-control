package tmux

import (
	"os/exec"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

type Window struct {
	ID         string
	Session    string
	Index      int
	Name       string
	Active     bool
	Label      string
	Current    bool
	InternalID string
}

type Pane struct {
	ID        string
	PaneID    string
	Session   string
	Window    string
	WindowIdx int
	Index     int
	Title     string
	Command   string
	Width     int
	Height    int
	Active    bool
	Label     string
	Current   bool
}

type PaneSnapshot struct {
	Panes          []Pane
	CurrentID      string
	CurrentLabel   string
	IncludeCurrent bool
	CurrentWindow  string
}

type Session struct {
	Name     string
	Label    string
	Attached bool
	Clients  []string
	Current  bool
	Windows  int
}

type SessionSnapshot struct {
	Sessions       []Session
	Current        string
	IncludeCurrent bool
}

type WindowSnapshot struct {
	Windows        []Window
	CurrentID      string
	CurrentLabel   string
	CurrentSession string
	IncludeCurrent bool
}

type windowHandle interface {
	Select() error
	Rename(string) error
	Kill() error
}

type sessionHandle interface {
	Rename(string) error
	Detach() error
	Kill() error
}

var (
	defaultSessionFormat = "#S: #{session_windows} windows#{?session_attached, (attached),}"

	newTmux = func(socketPath string) (tmuxClient, error) {
		if socketPath != "" {
			return gotmux.NewTmux(socketPath)
		}
		return gotmux.DefaultTmux()
	}

	runExecCommand = func(name string, args ...string) commander {
		return realCommander{cmd: exec.Command(name, args...)}
	}

	newWindowHandle = func(w *gotmux.Window) windowHandle {
		if w == nil {
			return nil
		}
		return &realWindowHandle{window: w}
	}

	newSessionHandle = func(s *gotmux.Session) sessionHandle {
		if s == nil {
			return nil
		}
		return &realSessionHandle{session: s}
	}
)

type tmuxClient interface {
    ListSessions() ([]*gotmux.Session, error)
    ListAllWindows() ([]*gotmux.Window, error)
    ListAllPanes() ([]*gotmux.Pane, error)
    ListClients() ([]*gotmux.Client, error)
    SwitchClient(*gotmux.SwitchClientOptions) error
    GetSessionByName(string) (*gotmux.Session, error)
    NewSession(*gotmux.SessionOptions) (*gotmux.Session, error)
    KillServer() error
    Close() error
}

type commander interface {
	Run() error
	Output() ([]byte, error)
}

type realCommander struct {
	cmd *exec.Cmd
}

func (r realCommander) Run() error {
	return r.cmd.Run()
}

func (r realCommander) Output() ([]byte, error) {
	return r.cmd.Output()
}

type realWindowHandle struct {
	window *gotmux.Window
}

func (h *realWindowHandle) Select() error {
	return h.window.Select()
}

func (h *realWindowHandle) Rename(name string) error {
	return h.window.Rename(name)
}

func (h *realWindowHandle) Kill() error {
	return h.window.Kill()
}

type realSessionHandle struct {
	session *gotmux.Session
}

func (h *realSessionHandle) Rename(name string) error {
	return h.session.Rename(name)
}

func (h *realSessionHandle) Detach() error {
	return h.session.Detach()
}

func (h *realSessionHandle) Kill() error {
	return h.session.Kill()
}
