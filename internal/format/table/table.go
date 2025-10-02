package table

import "strings"

type Alignment int

const (
	AlignLeft Alignment = iota
	AlignRight
)

// Format returns the rows padded according to the widest entry in each column.
func Format(rows [][]string, alignments []Alignment) []string {
	if len(rows) == 0 {
		return nil
	}
	colCount := len(rows[0])
	widths := make([]int, colCount)
	for _, row := range rows {
		for c, cell := range row {
			width := cellWidth(cell)
			if width > widths[c] {
				widths[c] = width
			}
		}
	}
	out := make([]string, len(rows))
	for i, row := range rows {
		var b strings.Builder
		for c, cell := range row {
			if c > 0 {
				b.WriteString("  ")
			}
			width := widths[c] - cellWidth(cell)
			if width < 0 {
				width = 0
			}
			if c < len(alignments) && alignments[c] == AlignRight {
				writeSpaces(&b, width)
				b.WriteString(cell)
			} else {
				b.WriteString(cell)
				writeSpaces(&b, width)
			}
		}
		out[i] = b.String()
	}
	return out
}

func cellWidth(text string) int {
	return len([]rune(text))
}

func writeSpaces(b *strings.Builder, count int) {
	if count <= 0 {
		return
	}
	for i := 0; i < count; i++ {
		b.WriteByte(' ')
	}
}
