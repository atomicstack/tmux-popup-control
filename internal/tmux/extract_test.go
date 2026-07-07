package tmux

import (
	"reflect"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func TestOriginPaneID(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "  %7 ")
	if got := OriginPaneID(); got != "%7" {
		t.Fatalf("OriginPaneID() = %q, want %q", got, "%7")
	}

	t.Setenv("TMUX_POPUP_CONTROL_PANE_ID", "")
	if got := OriginPaneID(); got != "" {
		t.Fatalf("OriginPaneID() unset = %q, want empty", got)
	}
}

func TestInsertTextIssuesSetAndPaste(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := InsertText("/sock", "%3", "hello"); err != nil {
		t.Fatalf("InsertText: %v", err)
	}

	want := [][]string{
		{"set-buffer", "--", "hello"},
		{"paste-buffer", "-p", "-t", "%3"},
	}
	if !reflect.DeepEqual(fake.commandCalls, want) {
		t.Fatalf("commandCalls = %#v, want %#v", fake.commandCalls, want)
	}
}

func TestCopyTextSetsBufferOnly(t *testing.T) {
	fake := &fakeClient{}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if err := CopyText("/sock", "world"); err != nil {
		t.Fatalf("CopyText: %v", err)
	}

	want := [][]string{
		{"set-buffer", "--", "world"},
	}
	if !reflect.DeepEqual(fake.commandCalls, want) {
		t.Fatalf("commandCalls = %#v, want %#v", fake.commandCalls, want)
	}
}

func TestCaptureVisible(t *testing.T) {
	fake := &fakeClient{}
	var gotTarget string
	var gotOp *gotmux.CaptureOptions
	fake.capturePaneFn = func(target string, op *gotmux.CaptureOptions) (string, error) {
		gotTarget = target
		gotOp = op
		return "canned output", nil
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	got, err := CaptureVisible("/sock", "%3")
	if err != nil {
		t.Fatalf("CaptureVisible: %v", err)
	}
	if got != "canned output" {
		t.Fatalf("CaptureVisible() = %q, want %q", got, "canned output")
	}
	if gotTarget != "%3" {
		t.Fatalf("CapturePane target = %q, want %q", gotTarget, "%3")
	}
	if gotOp == nil || !gotOp.PreserveAndJoin {
		t.Fatalf("CapturePane options = %#v, want PreserveAndJoin=true (-J, visible screen only)", gotOp)
	}
	if gotOp.StartLine != "" || gotOp.EndLine != "" {
		t.Fatalf("CapturePane options requested scrollback (StartLine=%q EndLine=%q), want visible screen only", gotOp.StartLine, gotOp.EndLine)
	}
}
