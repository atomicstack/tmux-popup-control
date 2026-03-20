package tmux

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// --- CreateSession ---

func TestCreateSessionSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := CreateSession("", "mysession", "/tmp/work"); err != nil {
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
}

func TestCreateSessionError(t *testing.T) {
	wantErr := errors.New("new-session failed")
	fake := &fakeClient{newErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := CreateSession("", "fail", "/tmp")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestCreateSessionClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := CreateSession("", "name", "/dir")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- CreateWindow ---

func TestCreateWindowSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := CreateWindow("", "main", 2, "editor", "/home/user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(fake.commandCalls))
	}
	got := strings.Join(fake.commandCalls[0], " ")
	want := "new-window -t main:2 -n editor -c /home/user -d"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestCreateWindowError(t *testing.T) {
	wantErr := errors.New("new-window failed")
	fake := &fakeClient{commandErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := CreateWindow("", "main", 0, "w", "/")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestCreateWindowClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := CreateWindow("", "s", 1, "w", "/")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

// --- SplitPane ---

func TestSplitPaneSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := SplitPane("", "main:1.0", "/var/log"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(fake.commandCalls))
	}
	got := strings.Join(fake.commandCalls[0], " ")
	want := "split-window -t main:1.0 -c /var/log -d"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSplitPaneError(t *testing.T) {
	wantErr := errors.New("split-window failed")
	fake := &fakeClient{commandErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := SplitPane("", "main:1.0", "/tmp")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestSplitPaneClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := SplitPane("", "t", "/")
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
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(fake.commandCalls))
	}
	got := strings.Join(fake.commandCalls[0], " ")
	want := "select-layout -t work:3 tiled"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSelectLayoutTargetError(t *testing.T) {
	wantErr := errors.New("select-layout failed")
	fake := &fakeClient{commandErr: wantErr}
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
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(fake.commandCalls))
	}
	got := strings.Join(fake.commandCalls[0], " ")
	want := "select-pane -t work:1.0"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestSelectPaneRestoreError(t *testing.T) {
	wantErr := errors.New("select-pane failed")
	fake := &fakeClient{commandErr: wantErr}
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

// --- SendPaneContents ---

func TestSendPaneContentsSuccess(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := SendPaneContents("", "main:0.0", "hello world"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fake.commandCalls) != 2 {
		t.Fatalf("expected 2 command calls, got %d: %v", len(fake.commandCalls), fake.commandCalls)
	}

	// first call: load-buffer <tmpfile>
	if len(fake.commandCalls[0]) < 2 || fake.commandCalls[0][0] != "load-buffer" {
		t.Errorf("expected first command to be load-buffer, got %v", fake.commandCalls[0])
	}
	tmpPath := fake.commandCalls[0][1]

	// second call: paste-buffer -t <target> -d
	got := strings.Join(fake.commandCalls[1], " ")
	want := "paste-buffer -t main:0.0 -d"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	// verify temp file was removed
	if _, err := os.Stat(tmpPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("temp file %q was not cleaned up", tmpPath)
	}
}

func TestSendPaneContentsCleanupOnLoadBufferError(t *testing.T) {
	wantErr := errors.New("load-buffer failed")
	fake := &fakeClient{commandErr: wantErr}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	err := SendPaneContents("", "main:0.0", "data")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}

	// temp file should be cleaned up even on error
	if len(fake.commandCalls) >= 1 && fake.commandCalls[0][0] == "load-buffer" {
		tmpPath := fake.commandCalls[0][1]
		if _, statErr := os.Stat(tmpPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("temp file %q was not cleaned up after load-buffer error", tmpPath)
		}
	}
}

func TestSendPaneContentsClientError(t *testing.T) {
	wantErr := errors.New("connect failed")
	withStubTmux(t, func(string) (tmuxClient, error) { return nil, wantErr })
	err := SendPaneContents("", "t", "x")
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
