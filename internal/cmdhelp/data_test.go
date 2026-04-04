package cmdhelp

import "testing"

func TestMoveWindowHelpExists(t *testing.T) {
	help, ok := Commands["move-window"]
	if !ok {
		t.Fatal("expected move-window help")
	}
	if help.Summary == "" {
		t.Fatal("expected move-window summary")
	}
}
