package tmux

import (
	"fmt"
	"strings"
	"testing"
)

func TestSessionPreview(t *testing.T) {
	var recorded []string
	withStubCommander(t, func(name string, args ...string) commander {
		recorded = append([]string{name}, args...)
		return &stubCommander{output: []byte(" * 0: main\n   1: logs\n")}
	})
	lines, err := SessionPreview("sock", "dev")
	if err != nil {
		t.Fatalf("SessionPreview returned error: %v", err)
	}
	wantArgs := []string{"tmux", "-S", "sock", "list-windows", "-t", "dev", "-F", sessionPreviewFormat}
	if strings.Join(recorded, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("unexpected command args: %v", recorded)
	}
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "* 0: main") || !strings.HasSuffix(lines[1], "1: logs") {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestSessionPreviewNoWindows(t *testing.T) {
	withStubCommander(t, func(name string, args ...string) commander {
		return &stubCommander{output: []byte("\n")}
	})
	lines, err := SessionPreview("", "dev")
	if err != nil {
		t.Fatalf("SessionPreview returned error: %v", err)
	}
	if len(lines) != 1 || lines[0] != "(no windows)" {
		t.Fatalf("expected fallback message, got %#v", lines)
	}
}

func TestWindowPreview(t *testing.T) {
	var recorded []string
	withStubCommander(t, func(name string, args ...string) commander {
		recorded = append([]string{name}, args...)
		return &stubCommander{output: []byte(" * 0: editor (nvim)\n   1: shell (zsh)\n")}
	})
	lines, err := WindowPreview("sock", "dev:1")
	if err != nil {
		t.Fatalf("WindowPreview returned error: %v", err)
	}
	wantArgs := []string{"tmux", "-S", "sock", "list-panes", "-t", "dev:1", "-F", windowPreviewFormat}
	if strings.Join(recorded, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("unexpected command args: %v", recorded)
	}
	if len(lines) != 2 || !strings.HasSuffix(lines[0], "* 0: editor (nvim)") {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestPanePreviewLimitsLines(t *testing.T) {
	var recorded []string
	withStubCommander(t, func(name string, args ...string) commander {
		recorded = append([]string{name}, args...)
		var buf strings.Builder
		for i := 0; i < 100; i++ {
			fmt.Fprintf(&buf, "line %03d\n", i)
		}
		return &stubCommander{output: []byte(buf.String())}
	})
	lines, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	wantArgs := []string{"tmux", "capture-pane", "-ep", "-S", "-40", "-t", "%3"}
	if strings.Join(recorded, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("unexpected command args: %v", recorded)
	}
	if len(lines) != panePreviewDefaultLines {
		t.Fatalf("expected %d lines, got %d", panePreviewDefaultLines, len(lines))
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
