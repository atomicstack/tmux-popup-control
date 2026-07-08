package clipboard

import (
	"errors"
	"testing"
)

// errBoom is a sentinel error stub runners return to simulate a failed
// clipboard tool invocation.
var errBoom = errors.New("boom")

// stubCall records a single invocation of the stub runner.
type stubCall struct {
	name  string
	stdin string
	args  []string
}

// newStubRunner returns a runner that records every call in calls and
// resolves errFor(name) to decide whether the call succeeds or fails.
func newStubRunner(calls *[]stubCall, errFor map[string]error) runner {
	return func(name string, stdin string, args ...string) error {
		*calls = append(*calls, stubCall{name: name, stdin: stdin, args: args})
		return errFor[name]
	}
}

func TestCopyForOSDarwinUsesPbcopy(t *testing.T) {
	var calls []stubCall
	stub := newStubRunner(&calls, nil)

	if err := copyForOS("darwin", "hello", stub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].name != "pbcopy" {
		t.Fatalf("name = %q, want pbcopy", calls[0].name)
	}
	if calls[0].stdin != "hello" {
		t.Fatalf("stdin = %q, want hello", calls[0].stdin)
	}
	if len(calls[0].args) != 0 {
		t.Fatalf("args = %v, want none", calls[0].args)
	}
}

func TestCopyForOSWindowsUsesClip(t *testing.T) {
	var calls []stubCall
	stub := newStubRunner(&calls, nil)

	if err := copyForOS("windows", "hi", stub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].name != "clip" {
		t.Fatalf("name = %q, want clip", calls[0].name)
	}
	if calls[0].stdin != "hi" {
		t.Fatalf("stdin = %q, want hi", calls[0].stdin)
	}
}

func TestCopyForOSLinuxFallsThroughToXsel(t *testing.T) {
	var calls []stubCall
	stub := newStubRunner(&calls, map[string]error{
		"wl-copy": errBoom,
		"xclip":   errBoom,
	})

	if err := copyForOS("linux", "text", stub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("calls = %d, want 3, got %+v", len(calls), calls)
	}
	if calls[0].name != "wl-copy" || calls[1].name != "xclip" || calls[2].name != "xsel" {
		t.Fatalf("call order = %+v, want wl-copy, xclip, xsel", calls)
	}
	last := calls[2]
	if got := last.args; len(got) != 2 || got[0] != "--clipboard" || got[1] != "--input" {
		t.Fatalf("xsel args = %v, want [--clipboard --input]", got)
	}
	if last.stdin != "text" {
		t.Fatalf("xsel stdin = %q, want text", last.stdin)
	}
}

func TestCopyForOSLinuxAllFailReturnsError(t *testing.T) {
	var calls []stubCall
	stub := newStubRunner(&calls, map[string]error{
		"wl-copy": errBoom,
		"xclip":   errBoom,
		"xsel":    errBoom,
	})

	if err := copyForOS("linux", "text", stub); err == nil {
		t.Fatal("expected an error when every tool fails")
	}
}

func TestCopyForOSLinuxFirstToolWins(t *testing.T) {
	var calls []stubCall
	stub := newStubRunner(&calls, nil)

	if err := copyForOS("linux", "text", stub); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1 (only wl-copy attempted), got %+v", len(calls), calls)
	}
	if calls[0].name != "wl-copy" {
		t.Fatalf("name = %q, want wl-copy", calls[0].name)
	}
	if calls[0].args != nil {
		t.Fatalf("args = %v, want nil", calls[0].args)
	}
}

func TestCopyForOSUnknownReturnsError(t *testing.T) {
	var calls []stubCall
	stub := newStubRunner(&calls, nil)

	if err := copyForOS("plan9", "x", stub); err == nil {
		t.Fatal("expected an error for an unsupported os")
	}
	if len(calls) != 0 {
		t.Fatalf("calls = %d, want 0 (unsupported os must not invoke the stub)", len(calls))
	}
}
