package menu

import "testing"

func TestCommandActionReturnsPromptMsgWithTrailingSpace(t *testing.T) {
	item := Item{ID: "display-message", Label: "display-message"}
	cmd := CommandAction(Context{}, item)
	if cmd == nil {
		t.Fatalf("expected command")
	}
	msg := cmd()
	prompt, ok := msg.(CommandPromptMsg)
	if !ok {
		t.Fatalf("expected CommandPromptMsg, got %T", msg)
	}
	expected := "display-message "
	if prompt.Command != expected {
		t.Fatalf("expected command %q, got %q", expected, prompt.Command)
	}
	if prompt.Label != item.Label {
		t.Fatalf("expected label %q, got %q", item.Label, prompt.Label)
	}
}
