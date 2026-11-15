package contentsearch

import "testing"

func TestFirst(t *testing.T) {
	text := "alpha one\n  Beta TWO  \ngamma three\nbeta again"
	tests := []struct {
		name     string
		query    string
		wantOK   bool
		wantLine int
		wantText string
	}{
		{"exact case", "gamma", true, 2, "gamma three"},
		{"case-insensitive query", "BETA two", true, 1, "  Beta TWO  "},
		{"first of several lines", "beta", true, 1, "  Beta TWO  "},
		{"substring inside a word", "amm", true, 2, "gamma three"},
		{"no match", "delta", false, 0, ""},
		{"empty query", "", false, 0, ""},
		{"blank query", "   ", false, 0, ""},
		{"query never spans lines", "one\n", false, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := First(text, tt.query)
			if ok != tt.wantOK {
				t.Fatalf("First(%q) ok = %v, want %v", tt.query, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Line != tt.wantLine || got.Text != tt.wantText {
				t.Fatalf("First(%q) = line %d %q, want line %d %q", tt.query, got.Line, got.Text, tt.wantLine, tt.wantText)
			}
		})
	}
}

func TestFirstKeepsInnerSpaces(t *testing.T) {
	if _, ok := First("foobar", "foo bar"); ok {
		t.Fatal("a query with a space must not match text without one")
	}
	if _, ok := First("say foo bar now", "foo bar"); !ok {
		t.Fatal("expected a match on the spaced phrase")
	}
}

func TestSpans(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		query string
		want  [][2]int
	}{
		{"single", "hello world", "world", [][2]int{{6, 11}}},
		{"case fold", "Hello HELLO hello", "hello", [][2]int{{0, 5}, {6, 11}, {12, 17}}},
		{"non-overlapping", "aaaa", "aa", [][2]int{{0, 2}, {2, 4}}},
		{"multibyte before the match", "naïve café bar", "bar", [][2]int{{13, 16}}},
		{"multibyte in the match", "the CAFÉ is open", "café", [][2]int{{4, 9}}},
		{"none", "hello", "xyz", nil},
		{"blank query", "hello", " ", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Spans(tt.line, tt.query)
			if len(got) != len(tt.want) {
				t.Fatalf("Spans(%q, %q) = %v, want %v", tt.line, tt.query, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("Spans(%q, %q) = %v, want %v", tt.line, tt.query, got, tt.want)
				}
			}
		})
	}
}

func TestSnippet(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		query string
		lead  int
		want  string
	}{
		{"trims and collapses whitespace", "  foo \t  bar   baz ", "bar", 20, "foo bar baz"},
		{"short lead is kept whole", "error: disk full", "disk", 20, "error: disk full"},
		{"long lead is cut so the match shows", "0123456789abcdefghij needle tail", "needle", 5, "…ghij needle tail"},
		{"no match returns the cleaned line", "  plain   line ", "zzz", 5, "plain line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Snippet(tt.line, tt.query, tt.lead); got != tt.want {
				t.Fatalf("Snippet(%q, %q, %d) = %q, want %q", tt.line, tt.query, tt.lead, got, tt.want)
			}
		})
	}
}
