package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func TestTrimCaptureOutput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"clean", "line1\nline2\n", "line1\nline2\n"},
		{"trailing blank lines", "line1\nline2\n\n\n\n", "line1\nline2\n"},
		{"mid-line whitespace preserved", "line1   \nline2\t\n", "line1   \nline2\n"},
		{"trailing blanks stripped", "a  \nb\n  \n\n", "a  \nb\n"},
		{"empty", "", ""},
		{"only blanks", "\n\n\n", ""},
		{"no trailing newline", "line1\nline2", "line1\nline2\n"},
		{"many trailing newlines", "prompt $ cmd\n" + strings.Repeat("\n", 50), "prompt $ cmd\n"},
		{"scrollback with blank tail", "line1\nline2\nprompt »\n" + strings.Repeat("\n", 100), "line1\nline2\nprompt »\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimCaptureOutput(tt.input)
			if got != tt.expect {
				t.Errorf("trimCaptureOutput(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestCapturePaneToFile(t *testing.T) {
	captured := "line1\nline2\nline3"
	var gotTarget string
	var gotOpts *gotmux.CaptureOptions

	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			gotTarget = target
			gotOpts = op
			return captured, nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	dir := t.TempDir()
	outPath := filepath.Join(dir, "capture.log")

	err := CapturePaneToFile("sock", "%3", outPath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotTarget != "%3" {
		t.Errorf("target = %q, want %%3", gotTarget)
	}
	if gotOpts.StartLine != "-" {
		t.Errorf("StartLine = %q, want \"-\"", gotOpts.StartLine)
	}
	if gotOpts.EscTxtNBgAttr {
		t.Error("EscTxtNBgAttr should be false when escSeqs=false")
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	expected := "line1\nline2\nline3\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestCapturePaneToFileTrimsTrailingNewlines(t *testing.T) {
	// Simulate realistic tmux capture-pane output: content followed by dozens
	// of blank lines (tmux pads to the scrollback height).
	rawOutput := "prompt $ ls\nfile1  file2  file3\nprompt $\n" + strings.Repeat("\n", 80)
	fake := &fakeClient{
		capturePaneFn: func(_ string, _ *gotmux.CaptureOptions) (string, error) {
			return rawOutput, nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	dir := t.TempDir()
	outPath := filepath.Join(dir, "capture.log")
	err := CapturePaneToFile("sock", "%0", outPath, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	expected := "prompt $ ls\nfile1  file2  file3\nprompt $\n"
	if string(data) != expected {
		t.Errorf("file has %d bytes, want %d\ngot:  %q\nwant: %q",
			len(data), len(expected), string(data), expected)
	}
}

func TestCapturePaneToFileWithEscSeqs(t *testing.T) {
	var gotOpts *gotmux.CaptureOptions
	fake := &fakeClient{
		capturePaneFn: func(_ string, op *gotmux.CaptureOptions) (string, error) {
			gotOpts = op
			return "content", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	dir := t.TempDir()
	outPath := filepath.Join(dir, "esc.log")
	err := CapturePaneToFile("sock", "%1", outPath, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gotOpts.EscTxtNBgAttr {
		t.Error("EscTxtNBgAttr should be true when escSeqs=true")
	}
}

func TestExpandFormat(t *testing.T) {
	fake := &fakeClient{
		displayMessageFn: func(target, format string) (string, error) {
			return "expanded:" + format, nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	result, err := ExpandFormat("sock", "%3", "#{pane_id}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "expanded:#{pane_id}" {
		t.Errorf("result = %q, want %q", result, "expanded:#{pane_id}")
	}
}
