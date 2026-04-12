package shquote

import "testing"

func TestQuoteEscapesSingleQuotes(t *testing.T) {
	got := Quote("space's here")
	want := "'space'\\''s here'"
	if got != want {
		t.Fatalf("Quote() = %q, want %q", got, want)
	}
}

func TestJoinCommandQuotesEachArgument(t *testing.T) {
	got := JoinCommand("tmux", "display-message", "it's fine")
	want := "'tmux' 'display-message' 'it'\\''s fine'"
	if got != want {
		t.Fatalf("JoinCommand() = %q, want %q", got, want)
	}
}
