package tmux

import (
	"os"
	"path/filepath"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

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
	if string(data) != captured {
		t.Errorf("file content = %q, want %q", string(data), captured)
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
