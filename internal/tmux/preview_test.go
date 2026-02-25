package tmux

import (
	"fmt"
	"strings"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func TestPanePreviewLimitsLines(t *testing.T) {
	var buf strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&buf, "line %03d\n", i)
	}
	captured := buf.String()
	wantStartLine := fmt.Sprintf("-%d", panePreviewDefaultLines)
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if target != "%3" {
				t.Errorf("unexpected target %q", target)
			}
			if op == nil || op.StartLine != wantStartLine {
				t.Errorf("unexpected options %#v", op)
			}
			return captured, nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	lines, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(lines) != panePreviewDefaultLines {
		t.Fatalf("expected %d lines, got %d", panePreviewDefaultLines, len(lines))
	}
}

func TestPanePreviewEmpty(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			return "", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	lines, err := PanePreview("", "%4")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(lines) != 1 || lines[0] != "(pane is empty)" {
		t.Fatalf("expected placeholder line, got %#v", lines)
	}
}
