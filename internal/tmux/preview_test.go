package tmux

import (
	"fmt"
	"strings"
	"testing"
)

func TestPanePreviewLimitsLines(t *testing.T) {
	var buf strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&buf, "line %03d\n", i)
	}
	captured := buf.String()
	wantTarget := "%3"
	var gotArgs []string
	withStubCommander(t, func(name string, args ...string) commander {
		gotArgs = args
		return &stubCommander{output: []byte(captured)}
	})
	lines, err := PanePreview("", wantTarget)
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(lines) != panePreviewDefaultLines {
		t.Fatalf("expected %d lines, got %d", panePreviewDefaultLines, len(lines))
	}
	// Verify target is passed to the exec command.
	found := false
	for _, arg := range gotArgs {
		if arg == wantTarget {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected target %q in exec args %v", wantTarget, gotArgs)
	}
}

func TestPanePreviewEmpty(t *testing.T) {
	withStubCommander(t, func(name string, args ...string) commander {
		return &stubCommander{output: []byte("")}
	})
	lines, err := PanePreview("", "%4")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(lines) != 1 || lines[0] != "(pane is empty)" {
		t.Fatalf("expected placeholder line, got %#v", lines)
	}
}
