package cmdhelp

import (
	"strings"
	"testing"
)

func TestMoveWindowHelpIncludesExpectedSummaryAndFlags(t *testing.T) {
	help, ok := Commands()["move-window"]
	if !ok {
		t.Fatal("expected move-window help")
	}
	if !strings.Contains(help.Summary, "move a window") {
		t.Fatalf("unexpected move-window summary: %q", help.Summary)
	}
	want := []string{"-a", "-b", "-d", "-k", "-r", "-s", "-t"}
	if len(help.Args) != len(want) {
		t.Fatalf("expected %d move-window args, got %d", len(want), len(help.Args))
	}
	for i, name := range want {
		if help.Args[i].Name != name {
			t.Fatalf("arg %d: got %q want %q", i, help.Args[i].Name, name)
		}
		if help.Args[i].Description == "" {
			t.Fatalf("arg %d (%s) should have a description", i, name)
		}
	}
}

func TestCommandsCoverCatalog(t *testing.T) {
	commands := Commands()
	if len(commands) < 90 {
		t.Fatalf("expected the full tmux command set, got %d entries", len(commands))
	}
	for name, help := range commands {
		if help.Summary == "" {
			t.Errorf("command %q has no summary", name)
		}
		for _, arg := range help.Args {
			if !strings.HasPrefix(arg.Name, "-") {
				t.Errorf("command %q: expected flag args only, got %q", name, arg.Name)
			}
			if arg.Description == "" {
				t.Errorf("command %q: flag %q has no description", name, arg.Name)
			}
		}
	}
	// Commands added by the tmux floating-pane work must be present now that
	// help comes from the embedded catalog rather than a generated snapshot.
	for _, name := range []string{"new-pane", "switch-mode"} {
		if _, ok := commands[name]; !ok {
			t.Errorf("expected %q help to be available", name)
		}
	}
}

func TestCommandsIsStable(t *testing.T) {
	first := Commands()
	second := Commands()
	if len(first) != len(second) {
		t.Fatalf("Commands() returned differing sizes: %d then %d", len(first), len(second))
	}
	if len(first) == 0 {
		t.Fatal("Commands() returned an empty map")
	}
}
