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

	if err := CreateSession("", "mysession", "/tmp/work", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.newSessionOptsCalls) != 1 {
		t.Fatalf("expected 1 NewSession call, got %d", len(fake.newSessionOptsCalls))
	}
	opts := fake.newSessionOptsCalls[0]
	if opts == nil {
		t.Fatal("expected non-nil options")
	}
	if opts.Name != "mysession" {
		t.Errorf("expected name %q, got %q", "mysession", opts.Name)
	}
	if opts.StartDirectory != "/tmp/work" {
		t.Errorf("expected dir %q, got %q", "/tmp/work", opts.StartDirectory)
	}
	if opts.ShellCommand != "" {
		t.Errorf("expected empty ShellCommand, got %q", opts.ShellCommand)
	}
}

func TestCreateSessionWithCommand(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	cmd := "cat /tmp/pane.txt; exec bash"
	if err := CreateSession("", "mysession", "/tmp", cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	opts := fake.newSessionOptsCalls[0]
	if opts.ShellCommand != cmd {
		t.Errorf("expected ShellCommand %q, got %q", cmd, opts.ShellCommand)
	}
}

func TestCreateSessionError(t *testing.T) {
	wantErr := errors.New("new-session failed")
	fake := &fakeClient{newErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := CreateSession("", "fail", "/tmp", "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestCreateSessionClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := CreateSession("", "name", "/dir", "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- CreateWindow ---

func TestCreateWindowSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := CreateWindow("", "main", 2, "editor", "/home/user", ""); err != nil {
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
	if err := CreateWindow("", "main", 2, "editor", "/home/user", cmd); err != nil {
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

	err := CreateWindow("", "main", 0, "w", "/", "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestCreateWindowClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := CreateWindow("", "s", 1, "w", "/", "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- SplitPane ---

func TestSplitPaneSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := SplitPane("", "main:1.0", "/var/log", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.splitWindowCalls) != 1 {
		t.Fatalf("expected 1 SplitWindow call, got %d", len(fake.splitWindowCalls))
	}
	call := fake.splitWindowCalls[0]
	if call.target != "main:1.0" {
		t.Errorf("expected target %q, got %q", "main:1.0", call.target)
	}
	if call.opts == nil {
		t.Fatal("expected non-nil options")
	}
	if call.opts.StartDirectory != "/var/log" {
		t.Errorf("expected dir %q, got %q", "/var/log", call.opts.StartDirectory)
	}
	if !call.opts.Detached {
		t.Error("expected Detached=true")
	}
	if call.opts.ShellCommand != "" {
		t.Errorf("expected empty ShellCommand, got %q", call.opts.ShellCommand)
	}
}

func TestSplitPaneWithCommand(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	cmd := "cat /tmp/pane.txt; exec bash"
	if err := SplitPane("", "main:1.0", "/var/log", cmd); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	call := fake.splitWindowCalls[0]
	if call.opts.ShellCommand != cmd {
		t.Errorf("expected ShellCommand %q, got %q", cmd, call.opts.ShellCommand)
	}
}

func TestSplitPaneError(t *testing.T) {
	wantErr := errors.New("split-window failed")
	fake := &fakeClient{splitWindowErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := SplitPane("", "main:1.0", "/tmp", "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSplitPaneClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := SplitPane("", "t", "/", "")
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
