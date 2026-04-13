package shquote

import (
	"strings"
	"testing"
)

func TestFieldsSingleQuoted(t *testing.T) {
	got := Fields("set-option -g status-left 'test'")
	want := []string{"set-option", "-g", "status-left", "test"}
	assertTokens(t, got, want)
}

func TestFieldsDoubleQuoted(t *testing.T) {
	got := Fields(`set-option -g status-left "test "`)
	want := []string{"set-option", "-g", "status-left", "test "}
	assertTokens(t, got, want)
}

func TestFieldsDoubleQuotedWithSpaces(t *testing.T) {
	got := Fields(`set-option -g status-left "hello world"`)
	want := []string{"set-option", "-g", "status-left", "hello world"}
	assertTokens(t, got, want)
}

func TestFieldsBackslashEscape(t *testing.T) {
	got := Fields(`set-option -g status-left hello\ world`)
	want := []string{"set-option", "-g", "status-left", "hello world"}
	assertTokens(t, got, want)
}

func TestFieldsBackslashInsideDoubleQuotes(t *testing.T) {
	got := Fields(`set-option -g status-left "hello \"world\""`)
	want := []string{"set-option", "-g", "status-left", `hello "world"`}
	assertTokens(t, got, want)
}

func TestFieldsSingleQuotePreservesBackslash(t *testing.T) {
	got := Fields(`set-option -g status-left 'hello\nworld'`)
	want := []string{"set-option", "-g", "status-left", `hello\nworld`}
	assertTokens(t, got, want)
}

func TestFieldsEmptyQuotedString(t *testing.T) {
	got := Fields(`set-option -g status-left ''`)
	want := []string{"set-option", "-g", "status-left", ""}
	assertTokens(t, got, want)
}

func TestFieldsMixedQuotes(t *testing.T) {
	got := Fields(`set-option -g status-left "hello"' world'`)
	want := []string{"set-option", "-g", "status-left", "hello world"}
	assertTokens(t, got, want)
}

func TestFieldsNoQuotes(t *testing.T) {
	got := Fields("set-option -g mouse on")
	want := []string{"set-option", "-g", "mouse", "on"}
	assertTokens(t, got, want)
}

func TestFieldsSingleQuotedLeadingSpace(t *testing.T) {
	got := Fields("set-option -g status-left ' hello'")
	want := []string{"set-option", "-g", "status-left", " hello"}
	assertTokens(t, got, want)
}

func TestFieldsSingleQuotedTrailingSpace(t *testing.T) {
	got := Fields("set-option -g status-left 'hello '")
	want := []string{"set-option", "-g", "status-left", "hello "}
	assertTokens(t, got, want)
}

func TestFieldsSingleQuotedBothSpaces(t *testing.T) {
	got := Fields("set-option -g status-left ' hello '")
	want := []string{"set-option", "-g", "status-left", " hello "}
	assertTokens(t, got, want)
}

func TestFieldsDoubleQuotedLeadingSpace(t *testing.T) {
	got := Fields(`set-option -g status-left " hello"`)
	want := []string{"set-option", "-g", "status-left", " hello"}
	assertTokens(t, got, want)
}

func TestFieldsDoubleQuotedTrailingSpace(t *testing.T) {
	got := Fields(`set-option -g status-left "hello "`)
	want := []string{"set-option", "-g", "status-left", "hello "}
	assertTokens(t, got, want)
}

func TestFieldsDoubleQuotedBothSpaces(t *testing.T) {
	got := Fields(`set-option -g status-left " hello "`)
	want := []string{"set-option", "-g", "status-left", " hello "}
	assertTokens(t, got, want)
}

func TestFieldsQuotedOnlySpaces(t *testing.T) {
	got := Fields("set-option -g status-left '   '")
	want := []string{"set-option", "-g", "status-left", "   "}
	assertTokens(t, got, want)
}

func TestFieldsEmpty(t *testing.T) {
	got := Fields("")
	if len(got) != 0 {
		t.Fatalf("expected 0 tokens, got %d: %v", len(got), got)
	}
}

func TestFieldsWhitespaceOnly(t *testing.T) {
	got := Fields("   ")
	if len(got) != 0 {
		t.Fatalf("expected 0 tokens, got %d: %v", len(got), got)
	}
}

func TestTrailingSpaceAfterToken(t *testing.T) {
	if !TrailingSpace("set-option ") {
		t.Fatal("expected trailing space")
	}
}

func TestTrailingSpaceNoSpace(t *testing.T) {
	if TrailingSpace("set-option") {
		t.Fatal("expected no trailing space")
	}
}

func TestTrailingSpaceAfterQuotedToken(t *testing.T) {
	if !TrailingSpace(`set-option "hello" `) {
		t.Fatal("expected trailing space after quoted token")
	}
}

func TestTrailingSpaceInsideQuote(t *testing.T) {
	if TrailingSpace(`set-option "hello `) {
		t.Fatal("expected no trailing space inside open quote")
	}
}

func TestTrailingSpaceEmpty(t *testing.T) {
	if !TrailingSpace("") {
		t.Fatal("expected trailing space for empty input")
	}
}

func assertTokens(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d tokens %v, want %d %v", len(got), got, len(want), want)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
}
