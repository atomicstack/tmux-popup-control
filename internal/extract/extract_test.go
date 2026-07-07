package extract

import (
	"reflect"
	"testing"
)

func texts(toks []Token) []string {
	out := make([]string, len(toks))
	for i, t := range toks {
		out[i] = t.Text
	}
	return out
}

func TestExtractWord(t *testing.T) {
	// reverse order: last-on-screen first; min length 5 drops "make" and "cd".
	got := texts(Extract("please make build\ncd internal", Word))
	want := []string{"internal", "build", "please"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("word = %v, want %v", got, want)
	}
}

func TestExtractURL(t *testing.T) {
	got := texts(Extract("see https://example.com/download here", URL))
	want := []string{"https://example.com/download"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("url = %v, want %v", got, want)
	}
}

func TestExtractPath(t *testing.T) {
	got := texts(Extract("edit internal/menu/registry.go now", Path))
	want := []string{"internal/menu/registry.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("path = %v, want %v", got, want)
	}
}

func TestExtractPathExcludesSpeeds(t *testing.T) {
	// Tokens must be long enough (≥5 runes) to survive minLen so that only
	// the exclude regex drops them. "12345k/s" (8 runes) matches [kmgKMG]/s$
	// and "123456/654321" (13 runes) matches ^\d+/\d+$.
	got := texts(Extract("rate 12345k/s page 123456/654321", Path))
	if len(got) != 0 {
		t.Fatalf("path = %v, want empty (excluded)", got)
	}
}

func TestExtractDedup(t *testing.T) {
	// "beta" is only 4 runes so min length 5 drops it too, leaving the two
	// "alpha" occurrences deduped down to one.
	got := texts(Extract("alpha alpha beta", Word))
	want := []string{"alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dedup = %v, want %v", got, want)
	}
}
