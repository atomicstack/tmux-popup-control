package tmux

import (
	"slices"
	"testing"
)

func TestParseSGRColourSpec(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"\x1b[38;2;154;205;50m", "#9acd32", true},
		{"\x1b[38;2;154;205;50m\n", "#9acd32", true},
		{"\x1b[48;2;108;166;205m", "#6ca6cd", true},
		{"\x1b[38;5;33m", "33", true},
		{"\x1b[48;5;7m", "7", true},
		{"\x1b[32m", "2", true},
		{"\x1b[92m", "10", true},
		{"\x1b[44m", "4", true},
		{"\x1b[105m", "13", true},
		{"\x1b[1;38;5;33m", "33", true},
		{"\x1b[39m", "", false},
		{"\x1b[0m", "", false},
		{"", "", false},
		{"garbage", "", false},
		{"\x1b[38;2;1;2m", "", false},
	}
	for _, tc := range cases {
		got, ok := parseSGRColourSpec(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("parseSGRColourSpec(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestIsThemeColourName(t *testing.T) {
	for _, name := range []string{
		"themeblack", "themewhite", "themelightgrey", "themedarkgrey", "themegreen",
		"themeyellow", "themered", "themeblue", "themecyan", "thememagenta", "ThemeBlue",
	} {
		if !IsThemeColourName(name) {
			t.Errorf("IsThemeColourName(%q) = false; want true", name)
		}
	}
	for _, name := range []string{"", "red", "colour33", "theme", "themepurple", "thememagenta2"} {
		if IsThemeColourName(name) {
			t.Errorf("IsThemeColourName(%q) = true; want false", name)
		}
	}
}

func TestResolveThemeColourQueriesClientAndCaches(t *testing.T) {
	fake := &fakeClient{commandOutput: "\x1b[38;2;154;205;50m\n"}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })

	spec, ok := ResolveThemeColour("sock", "/dev/ttys004", "themegreen")
	if !ok || spec != "#9acd32" {
		t.Fatalf("ResolveThemeColour = %q, %v; want #9acd32, true", spec, ok)
	}
	want := []string{"display-message", "-c", "/dev/ttys004", "-p", "#{c/f:themegreen}"}
	if len(fake.commandCalls) != 1 || !slices.Equal(fake.commandCalls[0], want) {
		t.Fatalf("command calls = %q; want [%q]", fake.commandCalls, want)
	}

	// Second lookup for the same (socket, client, name) is served from cache.
	if spec, ok := ResolveThemeColour("sock", "/dev/ttys004", "ThemeGreen"); !ok || spec != "#9acd32" {
		t.Fatalf("cached ResolveThemeColour = %q, %v; want #9acd32, true", spec, ok)
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected cache hit, got %d command calls", len(fake.commandCalls))
	}

	// A different client is a different cache entry.
	if _, ok := ResolveThemeColour("sock", "/dev/ttys005", "themegreen"); !ok {
		t.Fatal("expected lookup for second client to succeed")
	}
	if len(fake.commandCalls) != 2 {
		t.Fatalf("expected a fresh command for the second client, got %d calls", len(fake.commandCalls))
	}
}

func TestResolveThemeColourIgnoresNonThemeNames(t *testing.T) {
	fake := &fakeClient{commandOutput: "\x1b[31m"}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	for _, name := range []string{"", "red", "colour33", "#ff0000", "#{?a,b,c}"} {
		if spec, ok := ResolveThemeColour("sock", "/dev/ttys004", name); ok {
			t.Errorf("ResolveThemeColour(%q) = %q, true; want false", name, spec)
		}
	}
	if len(fake.commandCalls) != 0 {
		t.Fatalf("non-theme names must not reach tmux, got %q", fake.commandCalls)
	}
}

func TestResolveThemeColourCachesEmptyOutput(t *testing.T) {
	fake := &fakeClient{commandOutput: "\n"}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	if _, ok := ResolveThemeColour("sock", "/dev/ttys004", "themered"); ok {
		t.Fatal("expected empty output to resolve as no colour")
	}
	if _, ok := ResolveThemeColour("sock", "/dev/ttys004", "themered"); ok {
		t.Fatal("expected cached negative result")
	}
	if len(fake.commandCalls) != 1 {
		t.Fatalf("expected the negative result to be cached, got %d calls", len(fake.commandCalls))
	}
}

func TestResolveThemeColourWithoutClientOmitsFlag(t *testing.T) {
	fake := &fakeClient{commandOutput: "\x1b[32m"}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	spec, ok := ResolveThemeColour("sock", "", "themegreen")
	if !ok || spec != "2" {
		t.Fatalf("ResolveThemeColour = %q, %v; want 2, true", spec, ok)
	}
	want := []string{"display-message", "-p", "#{c/f:themegreen}"}
	if len(fake.commandCalls) != 1 || !slices.Equal(fake.commandCalls[0], want) {
		t.Fatalf("command calls = %q; want [%q]", fake.commandCalls, want)
	}
}
