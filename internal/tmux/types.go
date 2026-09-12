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
	SessionID  string
	Layout     string
	Zoomed     bool
}

// PaneRef identifies a pane by tmux ids so it can be addressed regardless of
// what the session or window is called. PaneID is required; SessionID and
// WindowID are used when known.
type PaneRef struct {
	SessionID string
	WindowID  string
	PaneID    string
}

type Pane struct {
	ID        string
	PaneID    string
	SessionID string
	WindowID  string
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
	// Floating reports a floating pane (tmux next-3.8+). tmux keeps floating
	// panes in the ordinary pane list, so consumers that lay panes out (the
	// tree view, resurrect) need the flag to tell them apart. X, Y and Z are
	// the floating pane's position and z-index; zero for tiled panes.
	Floating bool
	X        int
	Y        int
	Z        int
}

type PaneSnapshot struct {
	Panes          []Pane
	CurrentID      string
	CurrentLabel   string
	IncludeCurrent bool
	CurrentWindow  string
}

type Session struct {
	Name string
	// ID is the tmux session id ($N). tmux next-3.8 allows ':' and '.' in
	// session names, which makes name-based targets unparseable, so every
	// tmux command must target the id.
	ID       string
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
		// Opt into the JSON (v2) layout format. tmux next-3.9 keeps sending
		// the old v1 format to control clients unless they set new-layouts,
		// and v1 strings omit floating panes entirely, so resurrect could not
		// round-trip a window that had one. Sent as a separate call so an
		// older tmux that ignores or rejects the flag cannot undo no-output.
		_ = c.SetControlFlags("new-layouts")
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
	// Context-aware list operations (control-mode). Cancellation is
	// caller-side: the command has already been written to tmux, so the
	// request stays in the router's pending queue and its reply is discarded
	// on arrival.
	ListSessionsContext(ctx context.Context) ([]*gotmux.Session, error)
	ListAllWindowsContext(ctx context.Context) ([]*gotmux.Window, error)
	ListAllPanesContext(ctx context.Context) ([]*gotmux.Pane, error)
	ListSessionsFormatContext(ctx context.Context, format string) ([]string, error)
	ListWindowsFormatContext(ctx context.Context, target, filter, format string) ([]string, error)
	ListPanesFormatContext(ctx context.Context, target, filter, format string) ([]string, error)
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
