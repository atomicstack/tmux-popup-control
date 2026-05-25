package resurrect

import (
	"fmt"
	"strings"
)

// selectableLayout converts saved tmux layouts into the portable form accepted
// by select-layout. Exact layouts include source pane IDs, but tmux can parse
// the same cells without them and assign the restored panes in order.
func selectableLayout(layout string) string {
	layout = strings.TrimSpace(layout)
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
	for i := 0; i < len(layout); i++ {
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
