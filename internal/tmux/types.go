package tmux

import (
	"context"
	"os/exec"
	"slices"
	"sync"

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
	Layout     string
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
	Path      string
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
	Path     string
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
	ID() string
	Rename(string) error
	Detach() error
	Kill() error
}

var (
	defaultSessionFormat = "#S: #{session_windows} windows#{?session_attached, (attached),}"

	clientMu     sync.Mutex
	cachedClient tmuxClient
	cachedSocket string

	newTmux = func(socketPath string) (tmuxClient, error) {
		clientMu.Lock()
		defer clientMu.Unlock()
		if cachedClient != nil && cachedSocket == socketPath {
			return cachedClient, nil
		}
		if cachedClient != nil {
			cachedClient.Close()
		}
		var c tmuxClient
		var err error
		if socketPath != "" {
			c, err = gotmux.NewTmux(socketPath)
		} else {
			c, err = gotmux.DefaultTmux()
		}
		if err != nil {
			return nil, err
		}
		cachedClient = newTracedTmuxClient(socketPath, c)
		cachedSocket = socketPath
		configureControlClient(cachedClient)
		return cachedClient, nil
	}

	runExecCommand = func(name string, args ...string) commander {
		return tracedCommander{
			name: name,
			args: slices.Clone(args),
			cmd:  realCommander{cmd: exec.Command(name, args...)},
		}
	}

	// runExecCommandContext is the cancellable counterpart of runExecCommand,
	// used where a tmux exec must honour a context deadline / cancellation
	// (notably WaitFor, where a never-signaled channel would otherwise block
	// forever). Swapped in tests.
	runExecCommandContext = func(ctx context.Context, name string, args ...string) commander {
		return tracedCommander{
			name: name,
			args: slices.Clone(args),
			cmd:  realCommander{cmd: exec.CommandContext(ctx, name, args...)},
		}
	}

	// configureControlClient applies one-time setup to a freshly established
	// control-mode client. It suppresses %output notifications via
	// `refresh-client -f no-output`: this app never consumes pane-output events
	// (previews use request/response capture-pane; the backend polls list-*),
	// and during a restore the output buffered for an otherwise-idle control
	// client can stall tmux's draining of pane PTYs (flow control), blocking
	// content replay. Best-effort — an older tmux that rejects the flag must
	// not fail the connection.
	configureControlClient = func(c tmuxClient) {
		if c == nil {
			return
		}
		_ = c.SetControlFlags("no-output")
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
	// Pane operations (control-mode).
	RenamePane(target, title string) error
	SwapPanes(first, second string) error
	MovePane(source, target string) error
	BreakPane(source, destination string) error
	JoinPane(source, target string) error
	SelectPane(target string) error
	CapturePane(target string, op *gotmux.CaptureOptions) (string, error)
	// Window operations (control-mode).
	UnlinkWindow(target string) error
	LinkWindow(source, targetSession string) error
	MoveWindowToSession(source, targetSession string) error
	SwapWindows(first, second string) error
	SelectWindow(target string) error
	SelectLayout(target string, layout string) error
	SplitWindow(target string, op *gotmux.SplitWindowOptions) error
	// Option queries (control-mode).
	GlobalOption(key string) (string, error)
	Options(target, level string) ([]*gotmux.Option, error)
	// SetControlFlags sets control-mode client flags via `refresh-client -f`
	// (e.g. "no-output" to suppress %output notifications).
	SetControlFlags(flags string) error
	// Display and custom-format queries (control-mode).
	DisplayMessage(target, format string) (string, error)
	ListSessionsFormat(format string) ([]string, error)
	ListWindowsFormat(target, filter, format string) ([]string, error)
	ListPanesFormat(target, filter, format string) ([]string, error)
	// Raw command for operations that have no explicit target
	// (e.g. resize-pane without a pane ID,
	// kill-pane without a Tmux-level method).
	Command(parts ...string) (string, error)
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

func (h *realSessionHandle) ID() string {
	return h.session.Id
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

// Shutdown closes the cached control-mode connection, if any.
// Call this at application exit to avoid leaking tmux -C processes.
func Shutdown() {
	clientMu.Lock()
	if cachedClient != nil {
		cachedClient.Close()
		cachedClient = nil
		cachedSocket = ""
	}
	clientMu.Unlock()
	// Drop memoized lookups too — callers reaching for Shutdown expect a
	// fully clean slate before reconnecting (tests, in particular, swap
	// sockets/sessions and would otherwise read stale cache entries).
	resetCaches()
}
