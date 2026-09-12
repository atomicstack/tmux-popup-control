package tmux

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
)

// themeColourNames mirrors colour.c:colour_theme_table in tmux next-3.8.
// Theme colours are resolved per client at draw time (dark vs light theme,
// 16/256/truecolour), so they have no static value the UI could look up —
// tmux has to be asked on behalf of a specific client.
var themeColourNames = map[string]struct{}{
	"themeblack":     {},
	"themewhite":     {},
	"themelightgrey": {},
	"themedarkgrey":  {},
	"themegreen":     {},
	"themeyellow":    {},
	"themered":       {},
	"themeblue":      {},
	"themecyan":      {},
	"thememagenta":   {},
}

// IsThemeColourName reports whether name is one of tmux's theme colour
// names (themeblack, themeblue, thememagenta, …). Matching is
// case-insensitive, as tmux's own colour_fromstring is.
func IsThemeColourName(name string) bool {
	_, ok := themeColourNames[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// themeColourCache memoizes theme colour lookups per (socket, client, name)
// for the life of the process. The popup is short-lived and the theme a
// client sees does not change underneath it, so a permanent cache is safe.
// Negative results are cached too: the caller sits on the render path and
// must never pay a control-mode round-trip per frame for a name tmux
// cannot resolve.
var (
	themeColourCacheMu sync.Mutex
	themeColourCache   = map[string]themeColourEntry{}
)

type themeColourEntry struct {
	spec string
	ok   bool
}

func themeColourCacheKey(socketPath, clientID, name string) string {
	return socketPath + "\x00" + clientID + "\x00" + name
}

func resetThemeColourCache() {
	themeColourCacheMu.Lock()
	themeColourCache = map[string]themeColourEntry{}
	themeColourCacheMu.Unlock()
}

// ResolveThemeColour asks tmux to resolve a theme colour name for the given
// TTY client and returns the result as a lipgloss-compatible colour spec:
// "#rrggbb" for truecolour clients or a palette index ("N") for 256/16
// colour clients. It relies on the c/f format modifier, which emits the SGR
// sequence tmux would use to draw that colour on that client. Non-theme
// names return ok=false without touching tmux. When clientID is empty the
// query runs without -c, which tmux answers with the 16-colour fallback.
func ResolveThemeColour(socketPath, clientID, name string) (string, bool) {
	lowered := strings.ToLower(strings.TrimSpace(name))
	if _, ok := themeColourNames[lowered]; !ok {
		return "", false
	}
	key := themeColourCacheKey(socketPath, clientID, lowered)
	themeColourCacheMu.Lock()
	entry, cached := themeColourCache[key]
	themeColourCacheMu.Unlock()
	if cached {
		return entry.spec, entry.ok
	}

	spec, ok := queryThemeColour(socketPath, clientID, lowered)

	themeColourCacheMu.Lock()
	themeColourCache[key] = themeColourEntry{spec: spec, ok: ok}
	themeColourCacheMu.Unlock()
	return spec, ok
}

func queryThemeColour(socketPath, clientID, name string) (string, bool) {
	client, err := newTmux(socketPath)
	if err != nil {
		return "", false
	}
	args := []string{"display-message"}
	if clientID = strings.TrimSpace(clientID); clientID != "" {
		args = append(args, "-c", clientID)
	}
	args = append(args, "-p", fmt.Sprintf("#{c/f:%s}", name))
	out, err := client.Command(args...)
	if err != nil {
		return "", false
	}
	return parseSGRColourSpec(out)
}

// parseSGRColourSpec extracts the colour from a single SGR sequence such as
// "\x1b[38;2;154;205;50m" (→ "#9acd32"), "\x1b[38;5;33m" (→ "33") or
// "\x1b[32m" (→ "2"). Foreground and background forms are both accepted;
// SGR 39/49 (default colour) and anything that is not an SGR sequence
// return ok=false.
func parseSGRColourSpec(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "\x1b[") {
		return "", false
	}
	parser := ansi.NewParser()
	seq, _, _, _ := ansi.DecodeSequence(s, ansi.NormalState, parser)
	if seq == "" || ansi.Cmd(parser.Command()).Final() != 'm' {
		return "", false
	}
	params := parser.Params()
	for i := 0; i < len(params); {
		p := params[i].Param(0)
		switch {
		case p >= 30 && p <= 37:
			return strconv.Itoa(p - 30), true
		case p >= 40 && p <= 47:
			return strconv.Itoa(p - 40), true
		case p >= 90 && p <= 97:
			return strconv.Itoa(p - 90 + 8), true
		case p >= 100 && p <= 107:
			return strconv.Itoa(p - 100 + 8), true
		case p == 38 || p == 48 || p == 58:
			var c color.Color
			n := ansi.ReadStyleColor(params[i:], &c)
			if n == 0 {
				return "", false
			}
			return colourToSpec(c)
		}
		i++
	}
	return "", false
}

// colourToSpec renders a parsed SGR colour as a lipgloss colour spec.
func colourToSpec(c color.Color) (string, bool) {
	switch v := c.(type) {
	case ansi.IndexedColor:
		return strconv.Itoa(int(v)), true
	case ansi.BasicColor:
		return strconv.Itoa(int(v)), true
	case color.RGBA:
		return fmt.Sprintf("#%02x%02x%02x", v.R, v.G, v.B), true
	case nil:
		return "", false
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8)), true
}
