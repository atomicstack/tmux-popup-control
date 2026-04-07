package ui

import (
	"strings"
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/charmbracelet/x/ansi"
)

func TestViewDisplaysPreviewBlock(t *testing.T) {
	m := NewModel(ModelConfig{})
	lvl := newLevel("session:switch", "Sessions", []menu.Item{{ID: "dev", Label: "Dev"}}, nil)
	m.stack = []*level{lvl}
	m.preview["session:switch"] = &previewData{
		target: "dev",
		label:  "Dev Session",
		lines:  []string{"pane-1", "pane-2"},
		seq:    1,
	}
	view := m.View().Content
	if !strings.Contains(view, "Preview: Dev Session") {
		t.Fatalf("expected preview title in view, got:\n%s", view)
	}
	if !strings.Contains(view, "pane-2") {
		t.Fatalf("expected preview body in view, got:\n%s", view)
	}
}

func TestViewOverlaysCompletionAbovePrompt(t *testing.T) {
	m := NewModel(ModelConfig{Width: 60, Height: 12})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}
	lvl := newLevel("command", "command", []menu.Item{
		{ID: "kill-session", Label: "kill-session [-aC] [-t target-session]"},
	}, node)
	lvl.SetFilter("kill-session -t ", len([]rune("kill-session -t ")))
	m.stack = []*level{lvl}
	m.completion = newCompletionState(CompletionOptions{
		Items:     []string{"main", "work"},
		ArgType:   "target-session",
		TypeLabel: "target-session",
		AnchorCol: 18,
	})

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "main") {
		t.Fatalf("expected completion overlay in view, got:\n%s", view)
	}
	if !strings.Contains(view, "» kill-session -t ") {
		t.Fatalf("expected prompt to remain visible, got:\n%s", view)
	}
}

func TestViewOverlaysCompletionBelowPromptWhenInsufficientRoomAbove(t *testing.T) {
	m := NewModel(ModelConfig{Width: 60, Height: 12})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}
	lvl := newLevel("command", "command", []menu.Item{
		{ID: "move-window", Label: "move-window [-dr] [-s src-window] [-t dst-window]"},
	}, node)
	lvl.SetFilter("move-window -t ", len([]rune("move-window -t ")))
	m.stack = []*level{lvl}
	m.completion = newCompletionState(CompletionOptions{
		Items: []string{
			"main:0",
			"main:1",
			"work:0",
			"work:1",
			"scratch:0",
		},
		ArgType:   "dst-window",
		TypeLabel: "dst-window",
		AnchorCol: 18,
	})

	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	promptIdx := -1
	dropdownIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "» move-window -t ") {
			promptIdx = i
		}
		if strings.Contains(line, "main:0") {
			dropdownIdx = i
			break
		}
	}
	if promptIdx == -1 {
		t.Fatalf("expected prompt line in view, got:\n%s", strings.Join(lines, "\n"))
	}
	if dropdownIdx == -1 {
		t.Fatalf("expected dropdown line in view, got:\n%s", strings.Join(lines, "\n"))
	}
	if dropdownIdx <= promptIdx {
		t.Fatalf("expected dropdown below prompt when space above is insufficient, got:\n%s", strings.Join(lines, "\n"))
	}
}

func TestViewShowsCommandSummaryBelowPrompt(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 16})
	node, ok := m.registry.Find("command")
	if !ok {
		t.Fatal("expected command node")
	}
	lvl := newLevel("command", "command", []menu.Item{
		{ID: "move-window", Label: "move-window [-adpr] [-s src-window] [-t dst-window]"},
	}, node)
	lvl.SetFilter("move-window", len([]rune("move-window")))
	m.stack = []*level{lvl}

	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	promptIdx := -1
	summaryIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "» move-window") {
			promptIdx = i
		}
		if strings.Contains(line, "move a window to a new index") {
			summaryIdx = i
		}
	}
	if promptIdx == -1 {
		t.Fatalf("expected prompt line in view, got:\n%s", strings.Join(lines, "\n"))
	}
	if summaryIdx == -1 {
		t.Fatalf("expected summary line in view, got:\n%s", strings.Join(lines, "\n"))
	}
	if summaryIdx <= promptIdx {
		t.Fatalf("expected summary below prompt, got:\n%s", strings.Join(lines, "\n"))
	}
}

func TestViewShowsCommandOutputScreen(t *testing.T) {
	m := NewModel(ModelConfig{Width: 60, Height: 8})
	m.mode = ModeCommandOutput
	m.commandOutputTitle = "list-keys"
	m.commandOutputLines = []string{
		"bind-key -T root C-b send-prefix",
		"bind-key -T root C-o rotate-window",
	}

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "output") {
		t.Fatalf("expected output header, got:\n%s", view)
	}
	if !strings.Contains(view, "list-keys") {
		t.Fatalf("expected command title, got:\n%s", view)
	}
	if !strings.Contains(view, "bind-key -T root C-b send-prefix") {
		t.Fatalf("expected output body, got:\n%s", view)
	}
	if !strings.Contains(view, "esc back") {
		t.Fatalf("expected output footer, got:\n%s", view)
	}
}

func TestViewDisplaysPreviewCursorBlock(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 16})
	lvl := newLevel("pane:switch", "Panes", []menu.Item{{ID: "dev:1.0", Label: "Pane"}}, nil)
	m.stack = []*level{lvl}
	m.preview["pane:switch"] = &previewData{
		target:        "dev:1.0",
		lines:         []string{"abcd", "wxyz"},
		rawANSI:       false,
		cursorVisible: true,
		cursorX:       2,
		cursorY:       1,
		seq:           1,
	}

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "wx█z") {
		t.Fatalf("expected cursor block overlay in preview, got:\n%s", view)
	}
}

func TestViewOmitsPreviewCursorBlockWhenNotVisible(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 16})
	lvl := newLevel("pane:switch", "Panes", []menu.Item{{ID: "dev:1.0", Label: "Pane"}}, nil)
	m.stack = []*level{lvl}
	m.preview["pane:switch"] = &previewData{
		target:        "dev:1.0",
		lines:         []string{"abcd"},
		rawANSI:       false,
		cursorVisible: false,
		seq:           1,
	}

	view := ansi.Strip(m.View().Content)
	if strings.Contains(view, "█") {
		t.Fatalf("did not expect cursor block, got:\n%s", view)
	}
}

func TestViewUsesStyledLabelForNormalMenuItems(t *testing.T) {
	m := NewModel(ModelConfig{})
	lvl := newLevel("session:restore-from", "restore-from", []menu.Item{
		{ID: "plain", Label: "plain-label", StyledLabel: "\x1b[38;5;33mstyled-label\x1b[39m"},
	}, nil)
	m.stack = []*level{lvl}

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "styled-label") {
		t.Fatalf("expected styled label content in view, got:\n%s", view)
	}
	if strings.Contains(view, "plain-label") {
		t.Fatalf("did not expect plain label when StyledLabel is present, got:\n%s", view)
	}
}

func TestViewDoesNotEllipsizeStyledLabelThatFitsVisibly(t *testing.T) {
	m := NewModel(ModelConfig{Width: 10, Height: 6})
	lvl := newLevel("session:restore-from", "restore-from", []menu.Item{
		{ID: "short", Label: "short", StyledLabel: "\x1b[38;5;33mshort\x1b[39m"},
	}, nil)
	m.stack = []*level{lvl}

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "short") {
		t.Fatalf("expected styled label content in view, got:\n%s", view)
	}
	if strings.Contains(view, "▌ …") {
		t.Fatalf("did not expect restore row to be ellipsized, got:\n%s", view)
	}
}

func TestViewDisplaysPreviewCursorBlockPastLineEnd(t *testing.T) {
	m := NewModel(ModelConfig{Width: 80, Height: 16})
	lvl := newLevel("pane:switch", "Panes", []menu.Item{{ID: "dev:1.0", Label: "Pane"}}, nil)
	m.stack = []*level{lvl}
	m.preview["pane:switch"] = &previewData{
		target:        "dev:1.0",
		lines:         []string{"abcd"},
		rawANSI:       false,
		cursorVisible: true,
		cursorX:       4,
		cursorY:       0,
		seq:           1,
	}

	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "abcd█") {
		t.Fatalf("expected cursor block after trimmed line end, got:\n%s", view)
	}
}
