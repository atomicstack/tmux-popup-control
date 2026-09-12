package resurrect

import (
	"encoding/json"
	"fmt"
	"strings"
)

// selectableLayout converts saved tmux layouts into the portable form accepted
// by select-layout. Exact layouts include source pane IDs, but tmux can parse
// the same cells without them and assign the restored panes in order. Both
// layout formats are handled: the JSON (v2) form tmux next-3.9 sends to control
// clients that set the new-layouts flag, and the older checksummed v1 form.
func selectableLayout(layout string) string {
	layout = strings.TrimSpace(layout)
	if isJSONLayout(layout) {
		return stripJSONLayoutPaneIDs(layout)
	}
	if !isExactLayout(layout) {
		return layout
	}

	body := layout[5:]
	if idx := strings.IndexRune(body, '<'); idx > 0 && strings.HasSuffix(body, ">") {
		body = strings.TrimSpace(body[:idx])
	}
	rewritten, ok := stripLayoutPaneIDs(body)
	if !ok {
		return layout
	}
	return fmt.Sprintf("%04x,%s", layoutChecksum(rewritten), rewritten)
}

// isJSONLayout reports whether the layout uses the tmux next-3.9 JSON subset
// format ({"V":2,"L":{...}}). tmux sniffs the same way: a leading brace.
func isJSONLayout(layout string) bool {
	return strings.HasPrefix(strings.TrimSpace(layout), "{")
}

// stripJSONLayoutPaneIDs removes the "I" (source pane id) keys from every cell
// of a JSON layout. tmux assigns panes to cells in order when the ids do not
// match, so a layout saved on one server can be applied to freshly split panes
// on another. Malformed input is returned unchanged so tmux reports the error.
func stripJSONLayoutPaneIDs(layout string) string {
	var root any
	if err := json.Unmarshal([]byte(layout), &root); err != nil {
		return layout
	}
	dropJSONKey(root, "I")
	out, err := json.Marshal(root)
	if err != nil {
		return layout
	}
	return string(out)
}

func dropJSONKey(node any, key string) {
	switch v := node.(type) {
	case map[string]any:
		delete(v, key)
		for _, child := range v {
			dropJSONKey(child, key)
		}
	case []any:
		for _, child := range v {
			dropJSONKey(child, key)
		}
	}
}

func isExactLayout(layout string) bool {
	if len(layout) < 6 || layout[4] != ',' {
		return false
	}
	for i := range 4 {
		if !isHex(layout[i]) {
			return false
		}
	}
	return true
}

func stripLayoutPaneIDs(layout string) (string, bool) {
	var out strings.Builder
	for i := 0; i < len(layout); {
		if !isDigit(layout[i]) {
			out.WriteByte(layout[i])
			i++
			continue
		}

		start := i
		if !scanLayoutNumber(layout, &i) || i >= len(layout) || layout[i] != 'x' {
			return "", false
		}
		i++
		if !scanLayoutNumber(layout, &i) || i >= len(layout) || layout[i] != ',' {
			return "", false
		}
		i++
		if !scanLayoutNumber(layout, &i) || i >= len(layout) || layout[i] != ',' {
			return "", false
		}
		i++
		if !scanLayoutNumber(layout, &i) {
			return "", false
		}

		out.WriteString(layout[start:i])

		if i >= len(layout) || layout[i] != ',' {
			continue
		}

		paneIDStart := i
		i++
		if !scanLayoutNumber(layout, &i) {
			out.WriteString(layout[paneIDStart:i])
			continue
		}
		if i < len(layout) && layout[i] == 'x' {
			out.WriteString(layout[paneIDStart:i])
		}
	}
	return out.String(), true
}

func scanLayoutNumber(s string, idx *int) bool {
	start := *idx
	for *idx < len(s) && isDigit(s[*idx]) {
		*idx++
	}
	return *idx > start
}

func layoutChecksum(layout string) uint16 {
	var csum uint16
	for i := range len(layout) {
		csum = (csum >> 1) + ((csum & 1) << 15)
		csum += uint16(layout[i])
	}
	return csum
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func isHex(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}
