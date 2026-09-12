package ui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// stubThemeResolver returns a colourResolver backed by a fixed name → spec
// table, standing in for the live tmux theme lookup.
func stubThemeResolver(t *testing.T, specs map[string]string) colourResolver {
	t.Helper()
	return func(name string) (string, bool) {
		spec, ok := specs[name]
		return spec, ok
	}
}

func TestColourSpecForNameRejectsFormatStringsAndShortHex(t *testing.T) {
	for _, value := range []string{
		"#{?#{e|>=:#{client_colours},256},gray5,black}",
		"#[fg=red]",
		"#abc",
		"#12345",
		"#1234567",
		"#gggggg",
		"#",
	} {
		if spec, ok := colourSpecForName(value, nil); ok {
			t.Errorf("colourSpecForName(%q) = %q, true; want not a colour", value, spec)
		}
	}
}

func TestColourSpecForNameAcceptsSevenCharHex(t *testing.T) {
	spec, ok := colourSpecForName("#ff00AA", nil)
	if !ok || spec != "#ff00AA" {
		t.Fatalf("colourSpecForName(#ff00AA) = %q, %v; want passthrough", spec, ok)
	}
}

func TestColourSpecForNameResolvesThemeNamesViaResolver(t *testing.T) {
	resolver := stubThemeResolver(t, map[string]string{"themeblue": "#6ca6cd"})
	spec, ok := colourSpecForName("themeblue", resolver)
	if !ok || spec != "#6ca6cd" {
		t.Fatalf("colourSpecForName(themeblue) = %q, %v; want resolved spec", spec, ok)
	}
	if _, ok := colourSpecForName("themeblue", nil); ok {
		t.Fatal("expected themeblue to be unresolvable without a resolver")
	}
	calledWith := []string{}
	spy := func(name string) (string, bool) {
		calledWith = append(calledWith, name)
		return "", false
	}
	if spec, ok := colourSpecForName("red", spy); !ok || spec != "1" {
		t.Fatalf("colourSpecForName(red) = %q, %v; want basic index", spec, ok)
	}
	if _, ok := colourSpecForName("#ff0000", spy); !ok {
		t.Fatal("expected hex colour to resolve without the resolver")
	}
	if len(calledWith) != 0 {
		t.Fatalf("resolver must not be consulted for basic/hex names, got %v", calledWith)
	}
}

func TestDecorateShowOptionsLineThemeColourValue(t *testing.T) {
	resolver := stubThemeResolver(t, map[string]string{"themeblue": "#6ca6cd"})
	decorated, ok := decorateShowOptionsLine("display-panes-colour themeblue", nil, resolver)
	if !ok {
		t.Fatal("expected decoration for 'display-panes-colour themeblue'")
	}
	if !strings.Contains(decorated, "\x1b[38;2;108;166;205m") {
		t.Errorf("expected resolved theme colour on value, got %q", decorated)
	}
	if stripped := ansi.Strip(decorated); stripped != "display-panes-colour themeblue" {
		t.Errorf("stripped output changed: %q", stripped)
	}
}

func TestDecorateShowOptionsLineFormatStringValueRendersThroughBodyStyle(t *testing.T) {
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	for _, line := range []string{
		`dark-theme-black "#{?#{e|>=:#{client_colours},256},gray5,black}"`,
		`display-panes-colour #{?#{e|>=:#{client_colours},256},gray5,black}`,
	} {
		decorated, ok := decorateShowOptionsLine(line, &body, nil)
		if !ok {
			t.Fatalf("expected scope decoration for %q", line)
		}
		sp := strings.IndexByte(line, ' ')
		wantValue := body.Render(line[sp:])
		if !strings.HasSuffix(decorated, wantValue) {
			t.Errorf("format-string value must render through bodyStyle\nline: %q\n got: %q\nwant suffix: %q", line, decorated, wantValue)
		}
		if stripped := ansi.Strip(decorated); stripped != line {
			t.Errorf("stripped output changed: %q", stripped)
		}
	}
}

func TestDecorateShowOptionsLineQuotedStyleValue(t *testing.T) {
	resolver := stubThemeResolver(t, map[string]string{
		"themeblack":  "#0d0d0d",
		"themeyellow": "#b8860b",
	})
	line := `message-command-style "bg=themeblack,fg=themeyellow,#{?#{m/r:(^|#,)IS(PANE|MODE)($|#,),#{prompt_flags}},,fill=themeblack}"`
	decorated, ok := decorateShowOptionsLine(line, nil, resolver)
	if !ok {
		t.Fatal("expected decoration for quoted style value")
	}
	if !strings.Contains(decorated, "\x1b[38;2;13;13;13m") {
		t.Errorf("expected themeblack swatch on bg=, got %q", decorated)
	}
	if !strings.Contains(decorated, "\x1b[38;2;184;134;11m") {
		t.Errorf("expected themeyellow swatch on fg=, got %q", decorated)
	}
	if stripped := ansi.Strip(decorated); stripped != line {
		t.Errorf("quotes/format string must survive decoration, got %q", stripped)
	}
}

func TestSplitStyleAttrs(t *testing.T) {
	cases := []struct {
		value string
		want  []string
	}{
		{"fg=red,bold", []string{"fg=red", "bold"}},
		{"fg=red,,bold", []string{"fg=red", "", "bold"}},
		{"fg=red#,bg=blue", []string{"fg=red#,bg=blue"}},
		{
			"bg=themeblack,fg=themeyellow,#{?#{m/r:(^|#,)IS(PANE|MODE)($|#,),#{prompt_flags}},,fill=themeblack}",
			[]string{"bg=themeblack", "fg=themeyellow", "#{?#{m/r:(^|#,)IS(PANE|MODE)($|#,),#{prompt_flags}},,fill=themeblack}"},
		},
		{"fg=#{?a,red,blue},bg=#[fg=red,bg=blue],bold", []string{"fg=#{?a,red,blue}", "bg=#[fg=red,bg=blue]", "bold"}},
		{"", []string{""}},
	}
	for _, tc := range cases {
		got := splitStyleAttrs(tc.value)
		if !slices.Equal(got, tc.want) {
			t.Errorf("splitStyleAttrs(%q) = %q; want %q", tc.value, got, tc.want)
		}
	}
}

func TestDecorateStyleValueFormatAttributesRenderPlainly(t *testing.T) {
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	value := "fg=red,#{?x,fg=blue,bg=green}"
	out := decorateStyleValue(value, &body, nil)
	if out == "" {
		t.Fatal("expected fg=red to be decorated")
	}
	if !strings.Contains(out, "\x1b[31m") {
		t.Errorf("expected red swatch, got %q", out)
	}
	if !strings.Contains(out, body.Render("#{?x,fg=blue,bg=green}")) {
		t.Errorf("format attribute must render through bodyStyle as one unit, got %q", out)
	}
	for _, forbidden := range []string{"\x1b[34m", "\x1b[32m"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("colour inside format string must not be decorated (%q), got %q", forbidden, out)
		}
	}
}

func TestDecorateShowOptionsLineArrayKey(t *testing.T) {
	decorated, ok := decorateShowOptionsLine("pane-colours[3] #ff00aa", nil, nil)
	if !ok {
		t.Fatal("expected decoration for array option entry")
	}
	if !strings.Contains(decorated, "\x1b[38;2;255;0;170m") {
		t.Errorf("expected hex swatch on array entry value, got %q", decorated)
	}
	if stripped := ansi.Strip(decorated); stripped != "pane-colours[3] #ff00aa" {
		t.Errorf("stripped output changed: %q", stripped)
	}
}

func TestModelColourResolverUsesSocketAndClient(t *testing.T) {
	prev := resolveThemeColourFn
	t.Cleanup(func() { resolveThemeColourFn = prev })
	var gotSocket, gotClient, gotName string
	resolveThemeColourFn = func(socket, client, name string) (string, bool) {
		gotSocket, gotClient, gotName = socket, client, name
		return "#010203", true
	}
	m := NewModel(ModelConfig{Width: 80, Height: 24, SocketPath: "test.sock", ClientID: "/dev/ttys004"})
	spec, ok := m.colourResolver()("themegreen")
	if !ok || spec != "#010203" {
		t.Fatalf("resolver returned %q, %v", spec, ok)
	}
	if gotSocket != "test.sock" || gotClient != "/dev/ttys004" || gotName != "themegreen" {
		t.Fatalf("resolver called with (%q, %q, %q)", gotSocket, gotClient, gotName)
	}
}

func TestModelColourResolverWithoutClientIsNil(t *testing.T) {
	prev := resolveThemeColourFn
	t.Cleanup(func() { resolveThemeColourFn = prev })
	resolveThemeColourFn = func(socket, client, name string) (string, bool) {
		t.Fatalf("resolver must not be consulted without a tty client (called with %q, %q, %q)", socket, client, name)
		return "", false
	}
	m := NewModel(ModelConfig{Width: 80, Height: 24, SocketPath: "test.sock"})
	if m.colourResolver() != nil {
		t.Fatal("expected nil resolver when the model has no client id")
	}
	decorated, ok := decorateShowOptionsLine("display-panes-colour themeblue", nil, m.colourResolver())
	if ok && strings.Contains(decorated, "\x1b[38;2;") {
		t.Fatalf("expected no colour swatch for a theme name without a resolver, got %q", decorated)
	}
}
