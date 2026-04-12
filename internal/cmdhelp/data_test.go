package cmdhelp

import (
	"strings"
	"testing"
)

func TestMoveWindowHelpIncludesExpectedSummaryAndFlags(t *testing.T) {
	help, ok := Commands["move-window"]
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
