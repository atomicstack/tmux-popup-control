package tmux

import (
	"errors"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	gotmux "github.com/GianlucaP106/gotmux/gotmux"
)

type stubCommander struct {
	runErr       error
	output       []byte
	outputErr    error
	runCalled    *bool
	outputCalled *bool
}

func (s *stubCommander) Run() error {
	if s.runCalled != nil {
		*s.runCalled = true
	}
	return s.runErr
}

func (s *stubCommander) Output() ([]byte, error) {
	if s.outputCalled != nil {
		*s.outputCalled = true
	}
	return s.output, s.outputErr
}

func withStubCommander(t *testing.T, fn func(name string, args ...string) commander) {
	t.Helper()
	prev := runExecCommand
	runExecCommand = fn
	t.Cleanup(func() { runExecCommand = prev })
}

func withStubTmux(t *testing.T, fn func(string) (tmuxClient, error)) {
	t.Helper()
	prev := newTmux
	newTmux = fn
	t.Cleanup(func() { newTmux = prev })
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
	sessions       []*gotmux.Session
	sessionsErr    error
	windows        []*gotmux.Window
	windowsErr     error
	panes          []*gotmux.Pane
	panesErr       error
	clients        []*gotmux.Client
	clientsErr     error
	switchErr      error
	switchCalls    int
	getSessions    map[string]*gotmux.Session
	newErr         error
	windowHandles  map[string]windowHandle
	sessionHandles map[string]sessionHandle
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

func (f *fakeClient) SwitchClient(*gotmux.SwitchClientOptions) error {
	f.switchCalls++
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
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommander(t, func(_ string, args ...string) commander {
		if containsArg(args, "list-sessions") {
			return &stubCommander{output: []byte("dev\tcustom label\n")}
		}
		return &stubCommander{output: []byte{}, outputErr: fmt.Errorf("unexpected command: %v", args)}
	})
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_FORMAT", "")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "")
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

func TestFetchSessionsPropagatesError(t *testing.T) {
	fake := &fakeClient{sessionsErr: errors.New("boom")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommander(t, func(string, ...string) commander { return &stubCommander{} })
	if _, err := FetchSessions(""); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestFetchWindowLinesParsesOutput(t *testing.T) {
	output := " @1\tdev:0\tdev:0 main\n%2\tdev:1\tcustom label "
	var cmdName string
	var cmdArgs []string
	withStubCommander(t, func(name string, args ...string) commander {
		cmdName = name
		cmdArgs = args
		return &stubCommander{output: []byte(output)}
	})
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FILTER", "")
	t.Setenv("TMUX_POPUP_CONTROL_WINDOW_FORMAT", "#{window_name}")
	lines, err := fetchWindowLines("sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmdName != "tmux" {
		t.Fatalf("expected tmux command, got %q", cmdName)
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
	if !strings.Contains(strings.Join(cmdArgs, " "), "-S sock") {
		t.Fatalf("expected socket arg in %v", cmdArgs)
	}
}

func TestFetchWindowLinesFallsBackOnError(t *testing.T) {
	withStubCommander(t, func(string, ...string) commander {
		return &stubCommander{outputErr: errors.New("boom")}
	})
	if _, err := fetchWindowLines(""); err == nil {
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
	out := "%0\tdev:0.0\tlabel\tdev\tmain\t0\t0\t1\n%1\tdev:0.1\t\tdev\tmain\t0\t1\t0"
	withStubCommander(t, func(string, ...string) commander {
		return &stubCommander{output: []byte(out)}
	})
	lines, err := fetchPaneLines("")
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
	withStubCommander(t, func(string, ...string) commander {
		return &stubCommander{outputErr: errors.New("boom")}
	})
	if _, err := fetchPaneLines(""); err == nil {
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
	var captured []string
	withStubCommander(t, func(name string, args ...string) commander {
		captured = append([]string{name}, args...)
		return &stubCommander{runCalled: new(bool)}
	})
	if err := RenamePane("sock", " %0 ", " new "); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.Join(captured, " "), "-S sock rename-pane -t %0 new") {
		t.Fatalf("unexpected args %v", captured)
	}
}

func TestKillPanesSkipsBlank(t *testing.T) {
	var mu sync.Mutex
	var calls [][]string
	withStubCommander(t, func(name string, args ...string) commander {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, append([]string{name}, args...))
		return &stubCommander{runCalled: new(bool)}
	})
	err := KillPanes("sock", []string{"  ", "%0", "\t%1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(calls))
	}
	for _, call := range calls {
		if call[0] != "tmux" {
			t.Fatalf("unexpected binary %v", call)
		}
		if !strings.Contains(strings.Join(call[1:], " "), "-S sock kill-pane") {
			t.Fatalf("unexpected args %v", call[1:])
		}
	}
}

func TestSwapPanesValidation(t *testing.T) {
	if err := SwapPanes("", " ", "%1"); err == nil {
		t.Fatalf("expected error for missing first")
	}
	if err := SwapPanes("", "%0", ""); err == nil {
		t.Fatalf("expected error for missing second")
	}
	var captured []string
	withStubCommander(t, func(name string, args ...string) commander {
		captured = append([]string{name}, args...)
		return &stubCommander{}
	})
	if err := SwapPanes("sock", "%0", "%1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.Join(captured, " "), "swap-pane -s %0 -t %1") {
		t.Fatalf("unexpected args %v", captured)
	}
}

func TestMovePaneAllowsOptionalTarget(t *testing.T) {
	var captured []string
	withStubCommander(t, func(name string, args ...string) commander {
		captured = append([]string{name}, args...)
		return &stubCommander{}
	})
	if err := MovePane("sock", "%0", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(strings.Join(captured, " "), "-t") {
		t.Fatalf("did not expect target: %v", captured)
	}
	withStubCommander(t, func(name string, args ...string) commander {
		captured = append([]string{name}, args...)
		return &stubCommander{}
	})
	if err := MovePane("sock", "%0", "%1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.Join(captured, " "), "-t %1") {
		t.Fatalf("expected target in args %v", captured)
	}
}

func TestBreakPaneValidation(t *testing.T) {
	if err := BreakPane("", " ", ""); err == nil {
		t.Fatalf("expected error for missing source")
	}
	var captured []string
	withStubCommander(t, func(name string, args ...string) commander {
		captured = append([]string{name}, args...)
		return &stubCommander{}
	})
	if err := BreakPane("sock", "%0", "%1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.Join(captured, " "), "-s %0 -t %1") {
		t.Fatalf("unexpected args %v", captured)
	}
}

func TestSelectLayoutValidation(t *testing.T) {
	if err := SelectLayout("", "  "); err == nil {
		t.Fatalf("expected error")
	}
	withStubCommander(t, func(string, ...string) commander { return &stubCommander{} })
	if err := SelectLayout("", "even-horizontal"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResizePaneValidation(t *testing.T) {
	if err := ResizePane("", "left", 0); err == nil {
		t.Fatalf("expected error for amount")
	}
	if err := ResizePane("", "weird", 1); err == nil {
		t.Fatalf("expected error for direction")
	}
	var captured []string
	withStubCommander(t, func(name string, args ...string) commander {
		captured = append([]string{name}, args...)
		return &stubCommander{}
	})
	if err := ResizePane("sock", "up", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(strings.Join(captured, " "), "resize-pane -U 3") {
		t.Fatalf("unexpected args %v", captured)
	}
}

func TestUnlinkWindowsSkipsEmpty(t *testing.T) {
	var calls int
	withStubCommander(t, func(string, ...string) commander {
		calls++
		return &stubCommander{}
	})
	err := UnlinkWindows("sock", []string{"", " dev:1 "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one command, got %d", calls)
	}
}

func TestLinkMoveSwapWindows(t *testing.T) {
	withStubCommander(t, func(string, ...string) commander { return &stubCommander{} })
	if err := LinkWindow("", "src", "dst"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := MoveWindow("", "src", "dst"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := SwapWindows("", "a", "b"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	withStubCommander(t, func(string, ...string) commander {
		return &stubCommander{runErr: errors.New("boom")}
	})
	if err := LinkWindow("", "src", "dst"); err == nil || !strings.Contains(err.Error(), "failed to link window") {
		t.Fatalf("expected wrapped error")
	}
}

func TestFetchSessionLabelsFallback(t *testing.T) {
	withStubCommander(t, func(string, ...string) commander {
		return &stubCommander{outputErr: errors.New("boom")}
	})
	labels := fetchSessionLabels("", "")
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
		clients: []*gotmux.Client{{Session: "dev"}},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommander(t, func(string, ...string) commander {
		return &stubCommander{outputErr: errors.New("boom")}
	})
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
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommander(t, func(_ string, args ...string) commander {
		if containsArg(args, "list-panes") {
			out := "%0\tdev:0.0\tlabel0\tdev\tmain\t0\t0\t1\n%1\tdev:0.1\t\tdev\tmain\t0\t1\t0"
			return &stubCommander{output: []byte(out)}
		}
		return &stubCommander{outputErr: fmt.Errorf("unexpected command %v", args)}
	})
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

func TestSelectWindowUsesHandle(t *testing.T) {
	handle := &stubWindowHandle{}
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@1"},
		},
	}
	fake.useWindowHandles(t, map[string]*stubWindowHandle{"@1": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SelectWindow("", "@1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.selectCalls != 1 {
		t.Fatalf("expected select call, got %d", handle.selectCalls)
	}
}

func TestSelectWindowPropagatesHandleError(t *testing.T) {
	handle := &stubWindowHandle{selectErr: errors.New("boom")}
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@1"},
		},
	}
	fake.useWindowHandles(t, map[string]*stubWindowHandle{"@1": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := SelectWindow("", "@1"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestKillWindowUsesHandle(t *testing.T) {
	handle := &stubWindowHandle{}
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@1"},
		},
	}
	fake.useWindowHandles(t, map[string]*stubWindowHandle{"@1": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillWindow("", "@1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle.killCalls != 1 {
		t.Fatalf("expected kill call, got %d", handle.killCalls)
	}
}

func TestKillWindowsSkipsBlankAndUsesHandles(t *testing.T) {
	handleA := &stubWindowHandle{}
	handleB := &stubWindowHandle{}
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@1"},
			{Id: "@2"},
		},
	}
	fake.useWindowHandles(t, map[string]*stubWindowHandle{
		"@1": handleA,
		"@2": handleB,
	})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillWindows("", []string{"  ", " @1 ", "@2"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handleA.killCalls != 1 || handleB.killCalls != 1 {
		t.Fatalf("expected kill calls, got %d and %d", handleA.killCalls, handleB.killCalls)
	}
}

func TestKillWindowsMissingTarget(t *testing.T) {
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@1"},
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if err := KillWindows("", []string{"@2"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found error, got %v", err)
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
	if err := SwitchPane("", "dev"); err == nil || !strings.Contains(err.Error(), "invalid pane target") {
		t.Fatalf("expected validation error, got %v", err)
	}
	if err := SwitchPane("", "dev:0"); err == nil || !strings.Contains(err.Error(), "invalid pane target") {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSwitchPaneRunsCommands(t *testing.T) {
	handle := &stubWindowHandle{}
	fake := &fakeClient{
		windows: []*gotmux.Window{
			{Id: "@1", Index: 0, ActiveSessionsList: []string{"dev"}},
		},
	}
	fake.useWindowHandles(t, map[string]*stubWindowHandle{"@1": handle})
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	var runCalls [][]string
	withStubCommander(t, func(name string, args ...string) commander {
		runCalls = append(runCalls, append([]string{name}, args...))
		return &stubCommander{}
	})

	if err := SwitchPane("sock", "dev:0.%0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.switchCalls != 1 {
		t.Fatalf("expected switch client call, got %d", fake.switchCalls)
	}
	if handle.selectCalls != 1 {
		t.Fatalf("expected select call, got %d", handle.selectCalls)
	}
	foundSelectPane := false
	for _, call := range runCalls {
		if call[0] == "tmux" && containsArg(call[1:], "select-pane") {
			foundSelectPane = true
			if !strings.Contains(strings.Join(call[1:], " "), "-S sock") {
				t.Fatalf("expected socket arg in %v", call)
			}
		}
	}
	if !foundSelectPane {
		t.Fatalf("expected select-pane command, calls: %#v", runCalls)
	}
}
