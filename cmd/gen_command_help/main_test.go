package main

import (
	"strings"
	"testing"
)

func TestParseCommandSummaryMarkdown(t *testing.T) {
	input := strings.TrimSpace(`
command: move-window
command-summary: move a window to another position
command-args:
-r renumber windows
-a insert after target window
-s source window
-t destination window
`)

	commands, err := parseCommandSummary(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	help := commands["move-window"]
	if help.Summary != "move a window to another position" {
		t.Fatalf("expected summary, got %q", help.Summary)
	}
	if len(help.Args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(help.Args))
	}
	if help.Args[2].Name != "-s" || help.Args[2].Description != "source window" {
		t.Fatalf("unexpected arg help: %+v", help.Args[2])
	}
}

func TestRenderDataFileIsDeterministic(t *testing.T) {
	commands := map[string]commandHelp{
		"move-window": {
			Summary: "move a window to another position",
			Args: []argHelp{
				{Name: "-a", Description: "insert after target window"},
				{Name: "-t", Description: "destination window"},
			},
		},
		"attach-session": {
			Summary: "attach or switch to a session",
			Args: []argHelp{
				{Name: "-t", Description: "target session"},
			},
		},
	}

	rendered, err := renderDataFile(commands)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := string(rendered)
	attachIdx := strings.Index(text, "\"attach-session\"")
	moveIdx := strings.Index(text, "\"move-window\"")
	if attachIdx == -1 || moveIdx == -1 {
		t.Fatalf("expected rendered commands in output, got:\n%s", text)
	}
	if attachIdx >= moveIdx {
		t.Fatalf("expected deterministic command ordering, got:\n%s", text)
	}
}
