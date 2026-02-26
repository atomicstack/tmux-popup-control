package tmux

import (
	"errors"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func withStubTmux(t *testing.T, fn func(string) (tmuxClient, error)) {
	t.Helper()
	prev := newTmux
	prevClient := cachedClient
	prevSocket := cachedSocket
	cachedClient = nil
	cachedSocket = ""
	newTmux = fn
	t.Cleanup(func() {
		newTmux = prev
		cachedClient = prevClient
		cachedSocket = prevSocket
	})
}

type stubWindowHandle struct {
	selectCalls int
	selectErr   error
	renameArgs  []string
	renameErr   error
	killCalls   int
	killErr     error
}

func (s *stubWindowHandle) Select() error {
	s.selectCalls++
	return s.selectErr
}

func (s *stubWindowHandle) Rename(name string) error {
	s.renameArgs = append(s.renameArgs, name)
	return s.renameErr
}

func (s *stubWindowHandle) Kill() error {
	s.killCalls++
	return s.killErr
}

type stubSessionHandle struct {
	renameArgs  []string
	renameErr   error
	detachCalls int
	detachErr   error
	killCalls   int
	killErr     error
}

func (s *stubSessionHandle) Rename(name string) error {
	s.renameArgs = append(s.renameArgs, name)
	return s.renameErr
}

func (s *stubSessionHandle) Detach() error {
	s.detachCalls++
	return s.detachErr
}

func (s *stubSessionHandle) Kill() error {
	s.killCalls++
	return s.killErr
}

type fakeClient struct {
	sessions    []*gotmux.Session
	sessionsErr error
	windows     []*gotmux.Window
	windowsErr  error
	panes       []*gotmux.Pane
	panesErr    error
	clients     []*gotmux.Client
	clientsErr  error
	switchErr   error
	switchCalls int
	lastSwitchOpts *gotmux.SwitchClientOptions
	getSessions    map[string]*gotmux.Session
	newErr         error
	windowHandles  map[string]windowHandle
	sessionHandles map[string]sessionHandle

	// Pane operations.
	renamePaneCalls [][]string
	renamePaneErr   error
	swapPanesCalls  [][]string
	swapPanesErr    error
	movePaneCalls   [][]string
	movePaneErr     error
	breakPaneCalls  [][]string
	breakPaneErr    error
	joinPaneCalls   int
	joinPaneErr     error
	selectPaneCalls []string
	selectPaneErr   error
	capturePaneFn   func(target string, op *gotmux.CaptureOptions) (string, error)

	// Window operations.
	unlinkWindowCalls []string
	unlinkWindowErr   error
	linkWindowCalls   int
	linkWindowErr     error
	moveWindowCalls   int
	moveWindowErr     error
	swapWindowsCalls  int
	swapWindowsErr    error
	selectWindowCalls []string
	selectWindowErr   error

	// Display and format queries.
	displayMessageFn       func(target, format string) (string, error)
	listSessionsFormatLines []string
	listSessionsFormatErr   error
	listWindowsFormatLines  []string
	listWindowsFormatErr    error
	listPanesFormatLines    []string
	listPanesFormatErr      error

	// Raw command.
	commandCalls  [][]string
	commandOutput string
	commandErr    error
}

func (f *fakeClient) ListSessions() ([]*gotmux.Session, error) {
	if f.sessionsErr != nil {
		return nil, f.sessionsErr
	}
	return f.sessions, nil
}

func (f *fakeClient) ListAllWindows() ([]*gotmux.Window, error) {
	if f.windowsErr != nil {
		return nil, f.windowsErr
	}
	return f.windows, nil
}

func (f *fakeClient) ListAllPanes() ([]*gotmux.Pane, error) {
	if f.panesErr != nil {
		return nil, f.panesErr
	}
	return f.panes, nil
}

func (f *fakeClient) ListClients() ([]*gotmux.Client, error) {
	if f.clientsErr != nil {
		return nil, f.clientsErr
	}
	return f.clients, nil
}

func (f *fakeClient) SwitchClient(opts *gotmux.SwitchClientOptions) error {
	f.switchCalls++
	if opts != nil {
		cp := *opts
		f.lastSwitchOpts = &cp
	} else {
		f.lastSwitchOpts = nil
	}
	return f.switchErr
}

func (f *fakeClient) GetSessionByName(name string) (*gotmux.Session, error) {
	if f.getSessions != nil {
		if s, ok := f.getSessions[name]; ok {
			return s, nil
		}
		return nil, nil
	}
	for _, s := range f.sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, nil
}

func (f *fakeClient) NewSession(opts *gotmux.SessionOptions) (*gotmux.Session, error) {
	if f.newErr != nil {
		return nil, f.newErr
	}
	name := ""
	if opts != nil {
		name = opts.Name
	}
	return &gotmux.Session{Name: name}, nil
}

func (f *fakeClient) KillServer() error { return nil }

func (f *fakeClient) Close() error { return nil }

// Pane operations.

func (f *fakeClient) RenamePane(target, title string) error {
	f.renamePaneCalls = append(f.renamePaneCalls, []string{target, title})
	return f.renamePaneErr
}

func (f *fakeClient) SwapPanes(first, second string) error {
	f.swapPanesCalls = append(f.swapPanesCalls, []string{first, second})
	return f.swapPanesErr
}

func (f *fakeClient) MovePane(source, target string) error {
	f.movePaneCalls = append(f.movePaneCalls, []string{source, target})
	return f.movePaneErr
}

func (f *fakeClient) BreakPane(source, destination string) error {
	f.breakPaneCalls = append(f.breakPaneCalls, []string{source, destination})
	return f.breakPaneErr
}

func (f *fakeClient) JoinPane(source, target string) error {
	f.joinPaneCalls++
	return f.joinPaneErr
}

func (f *fakeClient) SelectPane(target string) error {
	f.selectPaneCalls = append(f.selectPaneCalls, target)
	return f.selectPaneErr
}

func (f *fakeClient) CapturePane(target string, op *gotmux.CaptureOptions) (string, error) {
	if f.capturePaneFn != nil {
		return f.capturePaneFn(target, op)
	}
	return "", nil
}

// Window operations.

func (f *fakeClient) UnlinkWindow(target string) error {
	f.unlinkWindowCalls = append(f.unlinkWindowCalls, target)
	return f.unlinkWindowErr
}

func (f *fakeClient) LinkWindow(source, targetSession string) error {
	f.linkWindowCalls++
	return f.linkWindowErr
}

func (f *fakeClient) MoveWindowToSession(source, targetSession string) error {
	f.moveWindowCalls++
	return f.moveWindowErr
}

func (f *fakeClient) SwapWindows(first, second string) error {
	f.swapWindowsCalls++
	return f.swapWindowsErr
}

func (f *fakeClient) SelectWindow(target string) error {
	f.selectWindowCalls = append(f.selectWindowCalls, target)
	return f.selectWindowErr
}

// Display and format queries.

func (f *fakeClient) DisplayMessage(target, format string) (string, error) {
	if f.displayMessageFn != nil {
		return f.displayMessageFn(target, format)
	}
	return "", nil
}

func (f *fakeClient) ListSessionsFormat(format string) ([]string, error) {
	if f.listSessionsFormatErr != nil {
		return nil, f.listSessionsFormatErr
	}
	return f.listSessionsFormatLines, nil
}

func (f *fakeClient) ListWindowsFormat(target, filter, format string) ([]string, error) {
	if f.listWindowsFormatErr != nil {
		return nil, f.listWindowsFormatErr
	}
	return f.listWindowsFormatLines, nil
}

func (f *fakeClient) ListPanesFormat(target, filter, format string) ([]string, error) {
	if f.listPanesFormatErr != nil {
		return nil, f.listPanesFormatErr
	}
	return f.listPanesFormatLines, nil
}

// Raw command.

func (f *fakeClient) Command(parts ...string) (string, error) {
	cp := make([]string, len(parts))
	copy(cp, parts)
	f.commandCalls = append(f.commandCalls, cp)
	return f.commandOutput, f.commandErr
}

func (f *fakeClient) useWindowHandles(t *testing.T, handles map[string]*stubWindowHandle) {
	t.Helper()
	f.windowHandles = make(map[string]windowHandle, len(handles))
	for id, handle := range handles {
		f.windowHandles[id] = handle
	}
	prevFactory := newWindowHandle
	newWindowHandle = func(w *gotmux.Window) windowHandle {
		if w != nil {
			if h, ok := f.windowHandles[w.Id]; ok {
				return h
			}
		}
		return prevFactory(w)
	}
	t.Cleanup(func() {
		newWindowHandle = prevFactory
		f.windowHandles = nil
	})
}

func (f *fakeClient) useSessionHandles(t *testing.T, handles map[string]*stubSessionHandle) {
	t.Helper()
	f.sessionHandles = make(map[string]sessionHandle, len(handles))
	for name, handle := range handles {
		f.sessionHandles[name] = handle
	}
	prevFactory := newSessionHandle
	newSessionHandle = func(s *gotmux.Session) sessionHandle {
		if s != nil {
			if h, ok := f.sessionHandles[s.Name]; ok {
				return h
			}
		}
		return prevFactory(s)
	}
	t.Cleanup(func() {
		newSessionHandle = prevFactory
		f.sessionHandles = nil
	})
}

func TestBaseArgs(t *testing.T) {
	t.Run("empty socket", func(t *testing.T) {
		args := baseArgs("")
		if len(args) != 0 {
			t.Fatalf("expected empty args, got %v", args)
		}
	})
	t.Run("with socket", func(t *testing.T) {
		args := baseArgs("/tmp/socket")
		if len(args) != 2 || args[0] != "-S" || args[1] != "/tmp/socket" {
			t.Fatalf("unexpected args %v", args)
		}
	})
}

func TestDefaultLabelForSession(t *testing.T) {
	session := &gotmux.Session{Name: "dev", Windows: 1, Attached: 0}
	if got := defaultLabelForSession(session); got != "dev: 1 window" {
		t.Fatalf("unexpected label %q", got)
	}
	session.Windows = 3
	session.Attached = 1
	if got := defaultLabelForSession(session); got != "dev: 3 windows (attached)" {
		t.Fatalf("unexpected label for plural %q", got)
	}
}

func TestResolveSocketPath(t *testing.T) {
	t.Run("flag wins", func(t *testing.T) {
		got, err := ResolveSocketPath("/tmp/flag")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/tmp/flag" {
			t.Fatalf("expected /tmp/flag, got %q", got)
		}
	})
	t.Run("env overrides", func(t *testing.T) {
		t.Setenv("TMUX_POPUP_SOCKET", "/tmp/env")
		got, err := ResolveSocketPath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/tmp/env" {
			t.Fatalf("expected /tmp/env, got %q", got)
		}
	})
	t.Run("tmux env fallback", func(t *testing.T) {
		t.Setenv("TMUX_POPUP_SOCKET", "")
		t.Setenv("TMUX", "/tmp/socket,123,0")
		got, err := ResolveSocketPath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/tmp/socket" {
			t.Fatalf("expected /tmp/socket, got %q", got)
		}
	})
	t.Run("default path", func(t *testing.T) {
		t.Setenv("TMUX_POPUP_SOCKET", "")
		t.Setenv("TMUX", "")
		t.Setenv("TMUX_TMPDIR", "/tmp")
		u, _ := user.Current()
		got, err := ResolveSocketPath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join("/tmp", "tmux-"+u.Uid, "default")
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

func TestFetchSessions(t *testing.T) {
	fake := &fakeClient{
		sessions: []*gotmux.Session{
			{Name: "dev", Windows: 2, Attached: 1, AttachedList: []string{"tty1"}},
		},
		clients: []*gotmux.Client{
			{Session: "dev"},
		},
		listSessionsFormatLines: []string{"dev\tcustom label"},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	t.Setenv("TMUX_PANE", "")
	snap, err := FetchSessions("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Current != "dev" {
		t.Fatalf("expected current dev, got %q", snap.Current)
	}
	if len(snap.Sessions) != 1 {
		t.Fatalf("expected single session, got %d", len(snap.Sessions))
	}
	if snap.Sessions[0].Label != "custom label" {
		t.Fatalf("expected custom label, got %q", snap.Sessions[0].Label)
	}
	if !snap.Sessions[0].Attached {
		t.Fatalf("expected attached session")
	}
}

func TestFetchSessionsControlModeClientNotCounted(t *testing.T) {
	// A control-mode client (gotmuxcc itself) attached to session "aaa" must
	// not cause "aaa" to appear as attached in the session switch menu.
	// Only non-control-mode clients should count.
	fake := &fakeClient{
		sessions: []*gotmux.Session{
			{Name: "aaa", Windows: 1, Attached: 1}, // inflated by gotmuxcc
			{Name: "zzz", Windows: 2, Attached: 1},
		},
		clients: []*gotmux.Client{
			{Session: "aaa", ControlMode: true},  // gotmuxcc itself — should be ignored
			{Session: "zzz", ControlMode: false}, // real terminal client
		},
		listSessionsFormatLines: []string{"aaa\taaa", "zzz\tzzz"},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "1")
	t.Setenv("TMUX_PANE", "")
	snap, err := FetchSessions("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range snap.Sessions {
		switch s.Name {
		case "aaa":
			if s.Attached {
				t.Fatalf("expected aaa NOT attached (only control-mode client), got Attached=true")
			}
		case "zzz":
			if !s.Attached {
				t.Fatalf("expected zzz attached (has real client), got Attached=false")
			}
			if len(s.Clients) != 1 {
				t.Fatalf("expected 1 real client for zzz, got %d", len(s.Clients))
			}
		}
	}
}

func TestFetchSessionsCurrentFromTmuxPane(t *testing.T) {
	// When multiple clients are attached to different sessions, currentSessionName
	// should use TMUX_PANE → display-message to identify the launching client's
	// session rather than blindly picking the first entry from ListClients.
	fake := &fakeClient{
		sessions: []*gotmux.Session{
			{Name: "work", Windows: 1, Attached: 1},
			{Name: "dev", Windows: 2, Attached: 1},
		},
		clients: []*gotmux.Client{
			{Session: "work"}, // first client — wrong session
			{Session: "dev"},
		},
		listSessionsFormatLines: []string{"work\twork", "dev\tdev"},
		displayMessageFn: func(target, format string) (string, error) {
			if target == "%5" && format == "#{session_name}" {
				return "dev", nil
			}
			return "", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_PANE", "%5")
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	snap, err := FetchSessions("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Current != "dev" {
		t.Fatalf("expected current=dev (from TMUX_PANE), got %q", snap.Current)
	}
	for _, s := range snap.Sessions {
		switch s.Name {
		case "dev":
			if !s.Current {
				t.Fatalf("expected dev session to be marked current")
			}
		case "work":
			if s.Current {
				t.Fatalf("expected work session NOT to be marked current")
			}
		}
	}
}

func TestFetchSessionsPropagatesError(t *testing.T) {
	fake := &fakeClient{sessionsErr: errors.New("boom")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if _, err := FetchSessions(""); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestFetchWindowLinesParsesOutput(t *testing.T) {
	fake := &fakeClient{
		listWindowsFormatLines: []string{
			" @1\tdev:0\tdev:0 main",
			"%2\tdev:1\tcustom label ",
		},
	}
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FORMAT", "#{window_name}")
	lines, err := fetchWindowLines(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected two lines, got %d", len(lines))
	}
	if lines[0].windowID != "@1" || lines[0].label != "dev:0 main" {
		t.Fatalf("unexpected first line %#v", lines[0])
	}
	if lines[1].label != "custom label" {
		t.Fatalf("unexpected second line %#v", lines[1])
	}
}

func TestFetchWindowLinesFallsBackOnError(t *testing.T) {
	fake := &fakeClient{listWindowsFormatErr: errors.New("boom")}
	if _, err := fetchWindowLines(fake); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFallbackWindowLines(t *testing.T) {
	windows := []*gotmux.Window{
		{Id: "@1", Index: 0, Name: "main", ActiveSessionsList: []string{"dev"}},
		{Id: "%2", Index: 1, Name: "logs", LinkedSessionsList: []string{"dev"}},
	}
	lines := fallbackWindowLines(windows)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].displayID != "dev:0" {
		t.Fatalf("unexpected display id %q", lines[0].displayID)
	}
	if !strings.Contains(lines[1].label, "logs") {
		t.Fatalf("expected label to contain logs, got %q", lines[1].label)
	}
}

func TestFetchPaneLinesParsesOutput(t *testing.T) {
	fake := &fakeClient{
		listPanesFormatLines: []string{
			"%0\tdev:0.0\tlabel\tdev\tmain\t0\t0\t1",
			"%1\tdev:0.1\t\tdev\tmain\t0\t1\t0",
		},
	}
	lines, err := fetchPaneLines(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !lines[0].current || lines[1].current {
		t.Fatalf("current flags wrong: %#v", lines)
	}
	if lines[1].label != "dev:0.1" {
		t.Fatalf("expected fallback label, got %q", lines[1].label)
	}
}

func TestFetchPaneLinesError(t *testing.T) {
	fake := &fakeClient{listPanesFormatErr: errors.New("boom")}
	if _, err := fetchPaneLines(fake); err == nil {
		t.Fatalf("expected error")
	}
}

func TestFallbackPaneLines(t *testing.T) {
	panes := []*gotmux.Pane{{Id: "%0", Index: 3}}
	lines := fallbackPaneLines(panes)
	if len(lines) != 1 {
		t.Fatalf("expected one line")
	}
	if lines[0].label != "%0" || lines[0].paneIndex != 3 {
		t.Fatalf("unexpected fallback %#v", lines[0])
	}
}

func TestRenamePaneValidation(t *testing.T) {
	if err := RenamePane("", "", ""); err == nil {
		t.Fatalf("expected error for missing target")
	}
	if err := RenamePane("", " %0 ", "  "); err == nil {
		t.Fatalf("expected error for missing title")
	}
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := RenamePane("sock", " %0 ", " new "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.renamePaneCalls) != 1 ||
		fake.renamePaneCalls[0][0] != "%0" ||
		fake.renamePaneCalls[0][1] != "new" {
		t.Fatalf("unexpected rename pane calls %#v", fake.renamePaneCalls)
	}
}

func TestKillPanesSkipsBlank(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	err := KillPanes("sock", []string{"  ", "%0", "\t%1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 2 {
		t.Fatalf("expected 2 command calls, got %d", len(fake.commandCalls))
	}
	for _, call := range fake.commandCalls {
		if len(call) < 3 || call[0] != "kill-pane" || call[1] != "-t" {
			t.Fatalf("unexpected command call %#v", call)
		}
	}
	targets := []string{fake.commandCalls[0][2], fake.commandCalls[1][2]}
	if !containsArg(targets, "%0") || !containsArg(targets, "%1") {
		t.Fatalf("expected %%0 and %%1 targets, got %v", targets)
	}
}

func TestSwapPanesValidation(t *testing.T) {
	if err := SwapPanes("", " ", "%1"); err == nil {
		t.Fatalf("expected error for missing first")
	}
	if err := SwapPanes("", "%0", ""); err == nil {
		t.Fatalf("expected error for missing second")
	}
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SwapPanes("sock", "%0", "%1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.swapPanesCalls) != 1 ||
		fake.swapPanesCalls[0][0] != "%0" ||
		fake.swapPanesCalls[0][1] != "%1" {
		t.Fatalf("unexpected swap panes calls %#v", fake.swapPanesCalls)
	}
}

func TestMovePaneAllowsOptionalTarget(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := MovePane("sock", "%0", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.movePaneCalls) != 1 ||
		fake.movePaneCalls[0][0] != "%0" ||
		fake.movePaneCalls[0][1] != "" {
		t.Fatalf("unexpected move pane calls for empty target %#v", fake.movePaneCalls)
	}
	fake.movePaneCalls = nil
	if err := MovePane("sock", "%0", "%1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.movePaneCalls) != 1 || fake.movePaneCalls[0][1] != "%1" {
		t.Fatalf("unexpected move pane calls with target %#v", fake.movePaneCalls)
	}
}

func TestBreakPaneValidation(t *testing.T) {
	if err := BreakPane("", " ", ""); err == nil {
		t.Fatalf("expected error for missing source")
	}
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := BreakPane("sock", "%0", "%1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.breakPaneCalls) != 1 ||
		fake.breakPaneCalls[0][0] != "%0" ||
		fake.breakPaneCalls[0][1] != "%1" {
		t.Fatalf("unexpected break pane calls %#v", fake.breakPaneCalls)
	}
}

func TestSelectLayoutValidation(t *testing.T) {
	if err := SelectLayout("", "  "); err == nil {
		t.Fatalf("expected error")
	}
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SelectLayout("", "even-horizontal"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 ||
		len(fake.commandCalls[0]) < 1 ||
		fake.commandCalls[0][0] != "select-layout" {
		t.Fatalf("unexpected command calls %#v", fake.commandCalls)
	}
}

func TestResizePaneValidation(t *testing.T) {
	if err := ResizePane("", "left", 0); err == nil {
		t.Fatalf("expected error for amount")
	}
	if err := ResizePane("", "weird", 1); err == nil {
		t.Fatalf("expected error for direction")
	}
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := ResizePane("sock", "up", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 ||
		fake.commandCalls[0][0] != "resize-pane" ||
		fake.commandCalls[0][1] != "-U" ||
		fake.commandCalls[0][2] != "3" {
		t.Fatalf("unexpected command calls %#v", fake.commandCalls)
	}
}

func TestUnlinkWindowsSkipsEmpty(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	err := UnlinkWindows("sock", []string{"", " dev:1 "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.unlinkWindowCalls) != 1 || fake.unlinkWindowCalls[0] != "dev:1" {
		t.Fatalf("expected one unlink call for dev:1, got %#v", fake.unlinkWindowCalls)
	}
}

func TestLinkMoveSwapWindows(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := LinkWindow("", "src", "dst"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := MoveWindow("", "src", "dst"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SwapWindows("", "a", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.linkWindowCalls != 1 || fake.moveWindowCalls != 1 || fake.swapWindowsCalls != 1 {
		t.Fatalf("expected one call each, got link=%d move=%d swap=%d",
			fake.linkWindowCalls, fake.moveWindowCalls, fake.swapWindowsCalls)
	}

	fake2 := &fakeClient{linkWindowErr: errors.New("boom")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake2, nil })
	if err := LinkWindow("", "src", "dst"); err == nil || !strings.Contains(err.Error(), "failed to link window") {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestFetchSessionLabelsFallback(t *testing.T) {
	fake := &fakeClient{listSessionsFormatErr: errors.New("boom")}
	labels := fetchSessionLabels(fake, "")
	if len(labels) != 0 {
		t.Fatalf("expected empty map, got %#v", labels)
	}
}

func containsArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}

func TestFetchWindowsUsesFallbackLines(t *testing.T) {
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@1", Index: 0, Name: "main", Active: true, ActiveSessionsList: []string{"dev"}},
			{Id: "%2", Index: 1, Name: "logs", Active: false, ActiveSessionsList: []string{"dev"}},
		},
		clients:              []*gotmux.Client{{Session: "dev"}},
		listWindowsFormatErr: errors.New("boom"),
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "1")
	snap, err := FetchWindows("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !snap.IncludeCurrent {
		t.Fatalf("expected include current")
	}
	if snap.CurrentID != "dev:0" {
		t.Fatalf("expected current id dev:0, got %q", snap.CurrentID)
	}
	if len(snap.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(snap.Windows))
	}
	if !strings.Contains(snap.Windows[0].Label, "main") {
		t.Fatalf("expected label with main, got %q", snap.Windows[0].Label)
	}
}

func TestFetchPanesParsesOutput(t *testing.T) {
	fake := &fakeClient{
		panes: []*gotmux.Pane{
			{Id: "%0", Title: "top", CurrentCommand: "vim", Width: 80, Height: 20, Active: true},
			{Id: "%1", Title: "tail", CurrentCommand: "tail", Width: 80, Height: 20, Active: false},
		},
		listPanesFormatLines: []string{
			"%0\tdev:0.0\tlabel0\tdev\tmain\t0\t0\t1",
			"%1\tdev:0.1\t\tdev\tmain\t0\t1\t0",
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
	snap, err := FetchPanes("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Panes) != 2 {
		t.Fatalf("expected 2 panes, got %d", len(snap.Panes))
	}
	if snap.CurrentID != "dev:0.0" {
		t.Fatalf("expected current pane id dev:0.0, got %q", snap.CurrentID)
	}
	if snap.CurrentWindow != "dev:0" {
		t.Fatalf("expected current window dev:0, got %q", snap.CurrentWindow)
	}
	if snap.Panes[0].Label != "label0" {
		t.Fatalf("expected label0, got %q", snap.Panes[0].Label)
	}
	if snap.Panes[0].Title != "top" {
		t.Fatalf("expected pane title top, got %q", snap.Panes[0].Title)
	}
}

func TestSelectWindowRunsCommand(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SelectWindow("", "main:1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.selectWindowCalls) != 1 || fake.selectWindowCalls[0] != "main:1" {
		t.Fatalf("unexpected select window calls %#v", fake.selectWindowCalls)
	}
}

func TestSelectWindowPropagatesError(t *testing.T) {
	fake := &fakeClient{selectWindowErr: errors.New("boom")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SelectWindow("", "main:1"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestKillWindowRunsCommand(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillWindow("", "@1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 || strings.Join(fake.commandCalls[0], " ") != "kill-window -t @1" {
		t.Fatalf("unexpected command calls: %v", fake.commandCalls)
	}
}

func TestKillWindowsSkipsBlankAndRunsCommands(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillWindows("", []string{"  ", " @1 ", "@2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 2 {
		t.Fatalf("expected 2 command calls, got %d: %v", len(fake.commandCalls), fake.commandCalls)
	}
	if strings.Join(fake.commandCalls[0], " ") != "kill-window -t @1" {
		t.Fatalf("unexpected first call: %v", fake.commandCalls[0])
	}
	if strings.Join(fake.commandCalls[1], " ") != "kill-window -t @2" {
		t.Fatalf("unexpected second call: %v", fake.commandCalls[1])
	}
}

func TestKillWindowsCommandError(t *testing.T) {
	fake := &fakeClient{commandErr: fmt.Errorf("tmux error")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillWindows("", []string{"@1"}); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestRenameSessionValidation(t *testing.T) {
	if err := RenameSession("", " ", "new"); err == nil || !strings.Contains(err.Error(), "session target required") {
		t.Fatalf("expected target error, got %v", err)
	}
}

func TestRenameSessionUsesHandle(t *testing.T) {
	handle := &stubSessionHandle{}
	fake := &fakeClient{
		getSessions: map[string]*gotmux.Session{
			"dev": {Name: "dev"},
		},
	}
	fake.useSessionHandles(t, map[string]*stubSessionHandle{"dev": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := RenameSession("", " dev ", "new"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(handle.renameArgs) != 1 || handle.renameArgs[0] != "new" {
		t.Fatalf("unexpected rename args %#v", handle.renameArgs)
	}
}

func TestDetachSessionsUsesHandles(t *testing.T) {
	handle := &stubSessionHandle{}
	fake := &fakeClient{
		getSessions: map[string]*gotmux.Session{
			"dev": {Name: "dev"},
		},
		clients: []*gotmux.Client{
			{Session: "dev"},
		},
	}
	fake.useSessionHandles(t, map[string]*stubSessionHandle{"dev": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := DetachSessions("", []string{" ", " dev "}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.detachCalls != 1 {
		t.Fatalf("expected detach call, got %d", handle.detachCalls)
	}
}

func TestDetachSessionsPropagatesError(t *testing.T) {
	handle := &stubSessionHandle{detachErr: errors.New("boom")}
	fake := &fakeClient{
		getSessions: map[string]*gotmux.Session{
			"dev": {Name: "dev"},
		},
		clients: []*gotmux.Client{
			{Session: "dev"},
		},
	}
	fake.useSessionHandles(t, map[string]*stubSessionHandle{"dev": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := DetachSessions("", []string{"dev"}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestKillSessionsUsesHandles(t *testing.T) {
	handle := &stubSessionHandle{}
	fake := &fakeClient{
		getSessions: map[string]*gotmux.Session{
			"dev": {Name: "dev"},
		},
	}
	fake.useSessionHandles(t, map[string]*stubSessionHandle{"dev": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillSessions("", []string{" dev "}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.killCalls != 1 {
		t.Fatalf("expected kill call, got %d", handle.killCalls)
	}
}

func TestKillSessionsPropagatesError(t *testing.T) {
	handle := &stubSessionHandle{killErr: errors.New("boom")}
	fake := &fakeClient{
		getSessions: map[string]*gotmux.Session{
			"dev": {Name: "dev"},
		},
	}
	fake.useSessionHandles(t, map[string]*stubSessionHandle{"dev": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillSessions("", []string{"dev"}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestSwitchPaneValidatesTarget(t *testing.T) {
	if err := SwitchPane("", "", "dev"); err == nil || !strings.Contains(err.Error(), "invalid pane target") {
		t.Fatalf("expected validation error, got %v", err)
	}
	if err := SwitchPane("", "", "dev:0"); err == nil || !strings.Contains(err.Error(), "invalid pane target") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSwitchPaneRunsCommands(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SwitchPane("sock", "/dev/ttys009", "dev:0.%0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastSwitchOpts == nil ||
		fake.lastSwitchOpts.TargetSession != "dev" ||
		fake.lastSwitchOpts.TargetClient != "/dev/ttys009" {
		t.Fatalf("unexpected switch opts %#v", fake.lastSwitchOpts)
	}
	if len(fake.selectWindowCalls) != 1 || fake.selectWindowCalls[0] != "dev:0" {
		t.Fatalf("unexpected select window calls %#v", fake.selectWindowCalls)
	}
	if len(fake.selectPaneCalls) != 1 || fake.selectPaneCalls[0] != "dev:0.%0" {
		t.Fatalf("unexpected select pane calls %#v", fake.selectPaneCalls)
	}
}

func TestSwitchPaneSkipsInvalidClientID(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SwitchPane("sock", "[shells] O:zsh, pane 0", "dev:0.%0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastSwitchOpts == nil {
		t.Fatal("expected switch to be called")
	}
	if fake.lastSwitchOpts.TargetClient != "" {
		t.Fatalf("expected empty TargetClient, got %q", fake.lastSwitchOpts.TargetClient)
	}
}

func TestSwitchClientTargetsRequestedClient(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SwitchClient("", "/dev/ttys004", "dev"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastSwitchOpts == nil ||
		fake.lastSwitchOpts.TargetClient != "/dev/ttys004" ||
		fake.lastSwitchOpts.TargetSession != "dev" {
		t.Fatalf("unexpected switch opts %#v", fake.lastSwitchOpts)
	}
}

func TestSwitchClientSkipsInvalidClientID(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	// Garbage client ID (status-line text) should be silently dropped.
	if err := SwitchClient("", "[shells] O:zsh, current pane 0", "dev"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastSwitchOpts == nil {
		t.Fatal("expected switch to be called")
	}
	if fake.lastSwitchOpts.TargetClient != "" {
		t.Fatalf("expected empty TargetClient, got %q", fake.lastSwitchOpts.TargetClient)
	}
	if fake.lastSwitchOpts.TargetSession != "dev" {
		t.Fatalf("expected TargetSession=dev, got %q", fake.lastSwitchOpts.TargetSession)
	}
}

func TestSwitchClientSkipsEmptyClientID(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SwitchClient("", "", "dev"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.lastSwitchOpts == nil {
		t.Fatal("expected switch to be called")
	}
	if fake.lastSwitchOpts.TargetClient != "" {
		t.Fatalf("expected empty TargetClient, got %q", fake.lastSwitchOpts.TargetClient)
	}
}

func TestCurrentClientIDReturnsNonControlModeClient(t *testing.T) {
	fake := &fakeClient{
		displayMessageFn: func(target, format string) (string, error) {
			if format == "#{session_name}" {
				return "main-session", nil
			}
			return "", fmt.Errorf("unexpected format")
		},
		clients: []*gotmux.Client{
			{Name: "client-12345", ControlMode: true, Session: "main-session"},
			{Name: "/dev/ttys004", ControlMode: false, Session: "main-session"},
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_PANE", "%5")

	got := CurrentClientID("/tmp/test.sock")
	if got != "/dev/ttys004" {
		t.Fatalf("expected /dev/ttys004, got %q", got)
	}
}

func TestCurrentClientIDSkipsControlModeOnly(t *testing.T) {
	fake := &fakeClient{
		displayMessageFn: func(target, format string) (string, error) {
			return "my-session", nil
		},
		clients: []*gotmux.Client{
			{Name: "client-99999", ControlMode: true, Session: "my-session"},
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_PANE", "%0")

	got := CurrentClientID("/tmp/test.sock")
	if got != "" {
		t.Fatalf("expected empty string when only control-mode clients exist, got %q", got)
	}
}

func TestCurrentClientIDFallsBackToTMUXEnv(t *testing.T) {
	// Inside display-popup, TMUX_PANE is empty but TMUX is set.
	fake := &fakeClient{
		displayMessageFn: func(target, format string) (string, error) {
			// $3 is the session ID extracted from TMUX env var.
			if target == "$3" && format == "#{session_name}" {
				return "popup-session", nil
			}
			return "", fmt.Errorf("unexpected target=%q format=%q", target, format)
		},
		clients: []*gotmux.Client{
			{Name: "client-11111", ControlMode: true, Session: "popup-session"},
			{Name: "/dev/ttys007", ControlMode: false, Session: "popup-session"},
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX", "/tmp/tmux-501/default,12345,3")

	got := CurrentClientID("/tmp/test.sock")
	if got != "/dev/ttys007" {
		t.Fatalf("expected /dev/ttys007 via TMUX env fallback, got %q", got)
	}
}

func TestCurrentClientIDMatchesSessionOnly(t *testing.T) {
	fake := &fakeClient{
		displayMessageFn: func(target, format string) (string, error) {
			return "session-A", nil
		},
		clients: []*gotmux.Client{
			{Name: "/dev/ttys001", ControlMode: false, Session: "session-B"},
			{Name: "/dev/ttys002", ControlMode: false, Session: "session-A"},
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	t.Setenv("TMUX_PANE", "%0")

	got := CurrentClientID("/tmp/test.sock")
	if got != "/dev/ttys002" {
		t.Fatalf("expected /dev/ttys002 (matching session), got %q", got)
	}
}

func TestIsValidClientName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid device path", "/dev/ttys004", true},
		{"valid pts path", "/dev/pts/0", true},
		{"internal client name", "client-67503", true},
		{"empty", "", false},
		{"spaces", "/dev/tty with space", false},
		{"status line garbage", "[shells] O:zsh, current pane 0", false},
		{"tab character", "/dev/tty\t1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidClientName(tt.input)
			if got != tt.want {
				t.Errorf("isValidClientName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestListCommands(t *testing.T) {
	expected := "attach-session (attach)\nbind-key (bind)"
	fake := &fakeClient{commandOutput: expected}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	out, err := ListCommands("/tmp/test.sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
	if len(fake.commandCalls) != 1 || fake.commandCalls[0][0] != "list-commands" {
		t.Fatalf("unexpected command calls: %v", fake.commandCalls)
	}
}

func TestListCommandsError(t *testing.T) {
	fake := &fakeClient{commandErr: fmt.Errorf("connection failed")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if _, err := ListCommands(""); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestListKeys(t *testing.T) {
	expected := "bind-key -T prefix d detach-client"
	fake := &fakeClient{commandOutput: expected}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	out, err := ListKeys("/tmp/test.sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
	if len(fake.commandCalls) != 1 || fake.commandCalls[0][0] != "list-keys" {
		t.Fatalf("unexpected command calls: %v", fake.commandCalls)
	}
}

func TestListKeysError(t *testing.T) {
	fake := &fakeClient{commandErr: fmt.Errorf("connection failed")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if _, err := ListKeys(""); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestShutdownClosesClient(t *testing.T) {
	fake := &fakeClient{}

	prevClient := cachedClient
	prevSocket := cachedSocket
	cachedClient = fake
	cachedSocket = "/tmp/test"
	t.Cleanup(func() {
		cachedClient = prevClient
		cachedSocket = prevSocket
	})

	Shutdown()
	if cachedClient != nil {
		t.Fatalf("expected cachedClient to be nil after Shutdown")
	}
	if cachedSocket != "" {
		t.Fatalf("expected cachedSocket to be empty after Shutdown")
	}
}

func TestNewTmuxCachesConnection(t *testing.T) {
	callCount := 0
	fake := &fakeClient{}
	prev := newTmux
	prevClient := cachedClient
	prevSocket := cachedSocket
	cachedClient = nil
	cachedSocket = ""
	newTmux = func(socketPath string) (tmuxClient, error) {
		clientMu.Lock()
		defer clientMu.Unlock()
		if cachedClient != nil && cachedSocket == socketPath {
			return cachedClient, nil
		}
		callCount++
		cachedClient = fake
		cachedSocket = socketPath
		return fake, nil
	}
	t.Cleanup(func() {
		newTmux = prev
		cachedClient = prevClient
		cachedSocket = prevSocket
	})

	c1, err := newTmux("/tmp/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c2, err := newTmux("/tmp/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c1 != c2 {
		t.Fatalf("expected same client instance from cache")
	}
	if callCount != 1 {
		t.Fatalf("expected newTmux factory called once, got %d", callCount)
	}
}

