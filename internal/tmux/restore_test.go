package tmux

import (
	"errors"
	"fmt"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// --- CreateSession ---

func TestCreateSessionSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := CreateSession(SessionSpec{SocketPath: "", Name: "mysession", Dir: "/tmp/work"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(fake.commandCalls))
	}
	got := fmt.Sprintf("%v", fake.commandCalls[0])
	want := "[new-session -d -s mysession -c /tmp/work]"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestCreateSessionWithCommand(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	cmd := "cat /tmp/pane.txt; exec bash"
	if err := CreateSession(SessionSpec{SocketPath: "", Name: "mysession", Dir: "/tmp", Command: cmd}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := fake.commandCalls[0]
	if got := args[len(args)-1]; got != cmd {
		t.Errorf("expected last arg %q, got %q", cmd, got)
	}
}

func TestCreateSessionError(t *testing.T) {
	wantErr := errors.New("new-session failed")
	fake := &fakeClient{commandErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := CreateSession(SessionSpec{SocketPath: "", Name: "fail", Dir: "/tmp"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestCreateSessionClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := CreateSession(SessionSpec{SocketPath: "", Name: "name", Dir: "/dir"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- CreateWindow ---

func TestCreateWindowSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := CreateWindow(WindowSpec{SocketPath: "", Session: "main", Index: 2, Name: "editor", Dir: "/home/user"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(fake.commandCalls))
	}
	got := fmt.Sprintf("%v", fake.commandCalls[0])
	want := "[new-window -t main:2 -n editor -c /home/user -d]"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestCreateWindowWithCommand(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	cmd := "cat /tmp/pane.txt; exec bash"
	if err := CreateWindow(WindowSpec{SocketPath: "", Session: "main", Index: 2, Name: "editor", Dir: "/home/user", Command: cmd}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := fake.commandCalls[0]
	last := args[len(args)-1]
	if last != cmd {
		t.Errorf("expected last arg %q, got %q", cmd, last)
	}
}

func TestCreateWindowError(t *testing.T) {
	wantErr := errors.New("new-window failed")
	fake := &fakeClient{commandErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := CreateWindow(WindowSpec{SocketPath: "", Session: "main", Index: 0, Name: "w", Dir: "/"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestCreateWindowClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := CreateWindow(WindowSpec{SocketPath: "", Session: "s", Index: 1, Name: "w", Dir: "/"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- SplitPane ---

func TestSplitPaneSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := SplitPane(PaneSpec{SocketPath: "", Target: "main:1.0", Dir: "/var/log"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(fake.commandCalls))
	}
	got := fmt.Sprintf("%v", fake.commandCalls[0])
	want := "[split-window -d -t main:1.0 -c /var/log]"
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestSplitPaneWithCommand(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	cmd := "cat /tmp/pane.txt; exec bash"
	if err := SplitPane(PaneSpec{SocketPath: "", Target: "main:1.0", Dir: "/var/log", Command: cmd}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := fake.commandCalls[0]
	if got := args[len(args)-1]; got != cmd {
		t.Errorf("expected last arg %q, got %q", cmd, got)
	}
}

func TestSplitPaneError(t *testing.T) {
	wantErr := errors.New("split-window failed")
	fake := &fakeClient{commandErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := SplitPane(PaneSpec{SocketPath: "", Target: "main:1.0", Dir: "/tmp"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSplitPaneClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := SplitPane(PaneSpec{SocketPath: "", Target: "t", Dir: "/"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- SelectLayoutTarget ---

func TestSelectLayoutTargetSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := SelectLayoutTarget("", "work:3", "tiled"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.selectLayoutCalls) != 1 {
		t.Fatalf("expected 1 SelectLayout call, got %d", len(fake.selectLayoutCalls))
	}
	call := fake.selectLayoutCalls[0]
	if call[0] != "work:3" || call[1] != "tiled" {
		t.Errorf("expected [work:3 tiled], got %v", call)
	}
}

func TestSelectLayoutTargetError(t *testing.T) {
	wantErr := errors.New("select-layout failed")
	fake := &fakeClient{selectLayoutErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := SelectLayoutTarget("", "main:1", "even-horizontal")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSelectLayoutTargetClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := SelectLayoutTarget("", "s:1", "tiled")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- SelectPane (restore) ---

func TestSelectPaneRestoreSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := SelectPane("", "work:1.0"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.selectPaneCalls) != 1 {
		t.Fatalf("expected 1 SelectPane call, got %d", len(fake.selectPaneCalls))
	}
	if fake.selectPaneCalls[0] != "work:1.0" {
		t.Errorf("expected target %q, got %q", "work:1.0", fake.selectPaneCalls[0])
	}
}

func TestSelectPaneRestoreError(t *testing.T) {
	wantErr := errors.New("select-pane failed")
	fake := &fakeClient{selectPaneErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := SelectPane("", "main:0.0")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSelectPaneRestoreClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := SelectPane("", "t")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- CapturePaneContents ---

func TestCapturePaneContentsSuccess(t *testing.T) {
	want := "line1\nline2\n"
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if target != "main:0.0" {
				return "", fmt.Errorf("unexpected target %q", target)
			}
			if op == nil || !op.PreserveTrailing {
				return "", fmt.Errorf("expected PreserveTrailing=true, got %+v", op)
			}
			if !op.EscTxtNBgAttr {
				return "", fmt.Errorf("expected EscTxtNBgAttr=true, got %+v", op)
			}
			if op.StartLine != "-" {
				return "", fmt.Errorf("expected StartLine='-', got %q", op.StartLine)
			}
			return want, nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	got, err := CapturePaneContents("", "main:0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCapturePaneContentsError(t *testing.T) {
	wantErr := errors.New("capture-pane failed")
	fake := &fakeClient{
		capturePaneFn: func(string, *gotmux.CaptureOptions) (string, error) {
			return "", wantErr
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	_, err := CapturePaneContents("", "main:0.0")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestCapturePaneContentsClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	_, err := CapturePaneContents("", "t")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- ShowOption ---

func TestShowOptionSuccess(t *testing.T) {
	fake := &fakeClient{
		globalOptionFn: func(key string) (string, error) {
			if key == "@my-option" {
				return "  myvalue  ", nil
			}
			return "", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	got := ShowOption("", "@my-option")
	if got != "myvalue" {
		t.Errorf("expected %q, got %q", "myvalue", got)
	}
}

func TestShowOptionError(t *testing.T) {
	fake := &fakeClient{
		globalOptionFn: func(string) (string, error) {
			return "", errors.New("option error")
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	got := ShowOption("", "@missing")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestShowOptionCachesResult verifies repeated lookups for the same
// (socket, option) pair only hit tmux once. Backend pollers query the
// same options on every cycle; without this cache each cycle issues
// redundant `show-options` commands that serialize through gotmuxcc and
// dominate startup latency under load.
func TestShowOptionCachesResult(t *testing.T) {
	calls := 0
	fake := &fakeClient{
		globalOptionFn: func(key string) (string, error) {
			calls++
			return "value", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	for i := range 5 {
		if got := ShowOption("/tmp/socket", "@cached-option"); got != "value" {
			t.Fatalf("call %d: got %q, want %q", i, got, "value")
		}
	}
	if calls != 1 {
		t.Errorf("expected 1 tmux call, got %d", calls)
	}

	// Different option key → new tmux call.
	_ = ShowOption("/tmp/socket", "@other-option")
	if calls != 2 {
		t.Errorf("expected 2 tmux calls after new key, got %d", calls)
	}

	// Different socket path → new tmux call.
	_ = ShowOption("/tmp/different-socket", "@cached-option")
	if calls != 3 {
		t.Errorf("expected 3 tmux calls after new socket, got %d", calls)
	}
}

// --- DefaultCommand ---

func TestDefaultCommandFromTmux(t *testing.T) {
	fake := &fakeClient{
		globalOptionFn: func(key string) (string, error) {
			if key == "default-command" {
				return "/usr/local/bin/fish", nil
			}
			return "", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	got := DefaultCommand("")
	if got != "/usr/local/bin/fish" {
		t.Errorf("expected /usr/local/bin/fish, got %q", got)
	}
}

func TestDefaultCommandFallsBackToShell(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	t.Setenv("SHELL", "/bin/zsh")
	got := DefaultCommand("")
	if got != "/bin/zsh" {
		t.Errorf("expected /bin/zsh, got %q", got)
	}
}

func TestDefaultCommandFallsBackToBinSh(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	t.Setenv("SHELL", "")
	got := DefaultCommand("")
	if got != "/bin/sh" {
		t.Errorf("expected /bin/sh, got %q", got)
	}
}
