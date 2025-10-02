package menu

import (
	"bufio"
	"strings"
)

func splitLines(input string) []string {
	scanner := bufio.NewScanner(strings.NewReader(input))
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines
}
