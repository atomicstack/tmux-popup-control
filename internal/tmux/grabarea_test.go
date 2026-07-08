package tmux

import (
	"errors"
	"reflect"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func TestCaptureScrollback(t *testing.T) {
	fake := &fakeClient{}
	var gotTarget string
	var gotOp *gotmux.CaptureOptions
	fake.capturePaneFn = func(target string, op *gotmux.CaptureOptions) (string, error) {
		gotTarget = target
		gotOp = op
		return "canned scrollback", nil
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	got, err := CaptureScrollback("/sock", "%3")
	if err != nil {
		t.Fatalf("CaptureScrollback: %v", err)
	}
	if got != "canned scrollback" {
		t.Fatalf("CaptureScrollback() = %q, want %q", got, "canned scrollback")
	}
	if gotTarget != "%3" {
		t.Fatalf("CapturePane target = %q, want %q", gotTarget, "%3")
	}
	if gotOp == nil || !gotOp.PreserveAndJoin {
		t.Fatalf("CapturePane options = %#v, want PreserveAndJoin=true (-J, join wrapped lines)", gotOp)
	}
	if gotOp.StartLine != "-" {
		t.Fatalf("CapturePane options StartLine = %q, want %q (full scrollback)", gotOp.StartLine, "-")
	}
}

func TestWindowPaneIDs(t *testing.T) {
	fake := &fakeClient{
		listPanesFormatLines: []string{
			"2\t%12",
			"0\t%10",
			"",
			"1\t%11",
			"  ",
			"bad-line",
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	got, err := WindowPaneIDs("/sock", "%10")
	if err != nil {
		t.Fatalf("WindowPaneIDs: %v", err)
	}
	want := []string{"%10", "%11", "%12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WindowPaneIDs() = %#v, want %#v", got, want)
	}
}

func TestWindowPaneIDsError(t *testing.T) {
	fake := &fakeClient{listPanesFormatErr: errors.New("boom")}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	if _, err := WindowPaneIDs("/sock", "%10"); err == nil {
		t.Fatalf("expected error")
	}
}
