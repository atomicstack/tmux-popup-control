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

func TestExtractQuote(t *testing.T) {
	got := texts(Extract(`run "hello world" now`, Quote))
	want := []string{`"hello world"`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quote = %v, want %v", got, want)
	}
}

func TestExtractSQuote(t *testing.T) {
	got := texts(Extract("run 'hello world' now", SQuote))
	want := []string{"'hello world'"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("s-quote = %v, want %v", got, want)
	}
}

func TestExtractLine(t *testing.T) {
	got := texts(Extract("  first line  \nx\nsecond line", Line))
	// "x" dropped (len<5); reverse order.
	want := []string{"second line", "first line"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("line = %v, want %v", got, want)
	}
}

func TestExtractAllUnionExcludesWordAndLine(t *testing.T) {
	got := texts(Extract(`open https://example.com path internal/menu/x.go "quoted val"`, All))
	// contains url, path, quote — but not the bare word "open"/"path".
	assertContains(t, got, "https://example.com")
	assertContains(t, got, "internal/menu/x.go")
	assertContains(t, got, `"quoted val"`)
	for _, tok := range got {
		if tok == "open" || tok == "path" {
			t.Fatalf("All must exclude bare words, got %q", tok)
		}
	}
}

func assertContains(t *testing.T, hay []string, needle string) {
	t.Helper()
	for _, h := range hay {
		if h == needle {
			return
		}
	}
	t.Fatalf("expected %q in %v", needle, hay)
}

func assertNotContains(t *testing.T, hay []string, needle string) {
	t.Helper()
	for _, h := range hay {
		if h == needle {
			t.Fatalf("expected %q NOT in %v", needle, hay)
		}
	}
}

func TestExtractHostSchemeStripsPath(t *testing.T) {
	got := texts(Extract("clone https://github.com/a/b now", Host))
	want := []string{"github.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %v, want %v", got, want)
	}
}

func TestExtractHostSchemeStripsPort(t *testing.T) {
	got := texts(Extract("open http://example.com:8080/p", Host))
	want := []string{"example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %v, want %v", got, want)
	}
}

func TestExtractHostSCPForm(t *testing.T) {
	got := texts(Extract("remote git@github.com:a/b.git", Host))
	want := []string{"github.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %v, want %v", got, want)
	}
}

func TestExtractHostGitScheme(t *testing.T) {
	got := texts(Extract("run git://git.example.org/repo", Host))
	want := []string{"git.example.org"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %v, want %v", got, want)
	}
}

func TestExtractHostFTPScheme(t *testing.T) {
	got := texts(Extract("get ftp://files.example.org/", Host))
	want := []string{"files.example.org"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %v, want %v", got, want)
	}
}

func TestExtractHostDropsShortHost(t *testing.T) {
	// derived host "host" is 4 runes, below defaultMinLength, so it's dropped.
	got := texts(Extract("connect ssh://user@host:22/x", Host))
	if len(got) != 0 {
		t.Fatalf("host = %v, want empty (dropped, <5 runes)", got)
	}
}

func TestExtractHostProseEmpty(t *testing.T) {
	got := texts(Extract("just some plain prose here", Host))
	if len(got) != 0 {
		t.Fatalf("host = %v, want empty", got)
	}
}

func TestExtractHostDedupAcrossForms(t *testing.T) {
	got := texts(Extract("a https://github.com/a and git@github.com:x.git", Host))
	want := []string{"github.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %v, want %v", got, want)
	}
}

func TestExtractHostReverseOrder(t *testing.T) {
	got := texts(Extract("see https://alpha.example.com then https://beta.example.com", Host))
	want := []string{"beta.example.com", "alpha.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("host = %v, want %v", got, want)
	}
}

func TestExtractQuotedDouble(t *testing.T) {
	got := texts(Extract(`say "hello world" now`, Quoted))
	want := []string{"hello world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quoted = %v, want %v", got, want)
	}
}

func TestExtractQuotedSingle(t *testing.T) {
	got := texts(Extract(`path '/etc/hosts' here`, Quoted))
	want := []string{"/etc/hosts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quoted = %v, want %v", got, want)
	}
}

func TestExtractQuotedDropsShortInner(t *testing.T) {
	// inner "hi" is 2 runes, below defaultMinLength, so it's dropped.
	got := texts(Extract(`a "hi" b`, Quoted))
	if len(got) != 0 {
		t.Fatalf("quoted = %v, want empty (dropped, <5 runes)", got)
	}
}

func TestExtractAllExcludesHostAndQuoted(t *testing.T) {
	got := texts(Extract(`open https://github.com/x path a/b.go "quoted value"`, All))
	assertContains(t, got, "https://github.com/x")
	assertContains(t, got, `"quoted value"`)
	assertNotContains(t, got, "github.com")
	assertNotContains(t, got, "quoted value")
}
