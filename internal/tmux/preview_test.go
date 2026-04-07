package tmux

import (
	"fmt"
	"strings"
	"testing"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

func TestPanePreviewLimitsLines(t *testing.T) {
	var buf strings.Builder
	for i := range 60 {
		fmt.Fprintf(&buf, "line %03d\n", i)
	}
	captured := buf.String()
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if target != "%3" {
				t.Errorf("unexpected target %q", target)
			}
			if op == nil {
				t.Fatalf("expected capture options")
			}
			if op.StartLine != "0" || op.EndLine != "" {
				t.Errorf("unexpected options %#v", op)
			}
			return captured, nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			if target != "%3" {
				t.Errorf("unexpected display target %q", target)
			}
			if format != "#{cursor_x},#{cursor_y},#{pane_height}" {
				t.Errorf("unexpected display format %q", format)
			}
			return "0,5,60", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(data.Lines) != panePreviewDefaultLines {
		t.Fatalf("expected %d lines, got %d", panePreviewDefaultLines, len(data.Lines))
	}
	if data.CursorVisible {
		t.Fatalf("expected hidden cursor for trimmed-out row, got %+v", data)
	}
}

func TestPanePreviewEmpty(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			return "", nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			return "0,0,24", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	data, err := PanePreview("", "%4")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(data.Lines) != 1 || data.Lines[0] != "" {
		t.Fatalf("expected blank preview line, got %#v", data.Lines)
	}
	if !data.CursorVisible || data.CursorX != 0 || data.CursorY != 0 {
		t.Fatalf("expected cursor at origin for empty pane, got %+v", data)
	}
}

func TestPanePreviewReturnsVisibleCursorCoordinates(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if op.StartLine != "0" || op.EndLine != "" {
				t.Fatalf("unexpected options %#v", op)
			}
			return "zero\none\ntwo\nthree\n", nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			if format != "#{cursor_x},#{cursor_y},#{pane_height}" {
				t.Fatalf("unexpected format %q", format)
			}
			return "2,3,4", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if !data.CursorVisible || data.CursorX != 2 || data.CursorY != 3 {
		t.Fatalf("unexpected cursor data: %+v", data)
	}
	if len(data.Lines) != 4 {
		t.Fatalf("expected 4 preview lines, got %d", len(data.Lines))
	}
}

func TestPanePreviewHidesCursorWhenOutsideVisibleTail(t *testing.T) {
	var buf strings.Builder
	for i := range 60 {
		fmt.Fprintf(&buf, "line %03d\n", i)
	}
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if op.StartLine != "0" || op.EndLine != "" {
				t.Fatalf("unexpected options %#v", op)
			}
			return buf.String(), nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			return "1,2,60", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if data.CursorVisible {
		t.Fatalf("expected hidden cursor, got %+v", data)
	}
}

func TestPanePreviewMapsCursorWithinTrimmedVisiblePane(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if op.StartLine != "0" || op.EndLine != "" {
				t.Fatalf("unexpected options %#v", op)
			}
			var buf strings.Builder
			for i := range 60 {
				fmt.Fprintf(&buf, "row %02d\n", i)
			}
			return buf.String(), nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			return "1,55,60", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if !data.CursorVisible {
		t.Fatalf("expected visible cursor, got %+v", data)
	}
	if data.CursorY != 35 {
		t.Fatalf("expected cursor within trimmed preview tail, got %+v", data)
	}
}

func TestPanePreviewPreservesBlankRowsUpToCursor(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if op.StartLine != "0" || op.EndLine != "" {
				t.Fatalf("unexpected options %#v", op)
			}
			return "abcd\n\n\n", nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			return "4,2,4", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if !data.CursorVisible || data.CursorY != 2 || data.CursorX != 4 {
		t.Fatalf("expected visible cursor on preserved blank row, got %+v", data)
	}
	if len(data.Lines) != 3 {
		t.Fatalf("expected blank rows kept through cursor, got %+v", data)
	}
}

func TestPanePreviewKeepsTopVisibleContentInSparseTallPane(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if op.StartLine != "0" || op.EndLine != "" {
				t.Fatalf("unexpected options %#v", op)
			}
			return "L00\nL01\nL02\nL03\nL04\nL05\nL06\nL07\nL08\nL09\n", nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			return "0,0,50", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(data.Lines) != 10 {
		t.Fatalf("expected sparse top content to remain visible, got %+v", data)
	}
	if data.Lines[0] != "L00" || data.Lines[9] != "L09" {
		t.Fatalf("unexpected visible content %+v", data)
	}
	if !data.CursorVisible || data.CursorY != 0 {
		t.Fatalf("expected cursor on first visible line, got %+v", data)
	}
}

func TestPanePreviewUsesVisibleRowsNotHistoryTail(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if op.StartLine != "0" || op.EndLine != "" {
				t.Fatalf("unexpected options %#v", op)
			}
			var buf strings.Builder
			for i := range 50 {
				fmt.Fprintf(&buf, "visible %02d\n", i)
			}
			return buf.String(), nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			return "3,49,50", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(data.Lines) != panePreviewDefaultLines {
		t.Fatalf("expected %d visible rows retained, got %+v", panePreviewDefaultLines, data)
	}
	if data.Lines[0] != "visible 10" || data.Lines[len(data.Lines)-1] != "visible 49" {
		t.Fatalf("expected bottom 40 visible rows, got %+v", data)
	}
	if !data.CursorVisible || data.CursorY != panePreviewDefaultLines-1 {
		t.Fatalf("expected cursor on last retained visible row, got %+v", data)
	}
}

func TestPanePreviewKeepsSingleVisibleLineWithBlankRowsBelow(t *testing.T) {
	fake := &fakeClient{
		capturePaneFn: func(target string, op *gotmux.CaptureOptions) (string, error) {
			if op.StartLine != "0" || op.EndLine != "" {
				t.Fatalf("unexpected options %#v", op)
			}
			var buf strings.Builder
			buf.WriteString("only-line\n")
			for range 64 {
				buf.WriteString("\n")
			}
			return buf.String(), nil
		},
		displayMessageFn: func(target, format string) (string, error) {
			return "8,0,65", nil
		},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	data, err := PanePreview("", "%3")
	if err != nil {
		t.Fatalf("PanePreview returned error: %v", err)
	}
	if len(data.Lines) != 1 || data.Lines[0] != "only-line" {
		t.Fatalf("expected single visible line retained, got %+v", data)
	}
	if !data.CursorVisible || data.CursorY != 0 || data.CursorX != 8 {
		t.Fatalf("expected cursor on retained line, got %+v", data)
	}
}
