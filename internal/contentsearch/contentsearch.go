// Package contentsearch finds a plain-text query inside captured pane text.
// Matching is a case-insensitive substring test, one line at a time. The
// package is consumer-agnostic: no bubbletea, lipgloss, tmux or menu imports.
package contentsearch

import (
	"strings"
	"unicode/utf8"
)

// Match is the first line of a text that contains the query.
type Match struct {
	Line int    // zero-based line index within the text
	Text string // the matching line, unmodified
}

// Blank reports whether query has nothing to search for.
func Blank(query string) bool { return strings.TrimSpace(query) == "" }

// First returns the first line of text containing query, ignoring case.
// A blank query matches nothing. The query is otherwise used as typed, so
// inner and trailing spaces are significant.
func First(text, query string) (Match, bool) {
	if Blank(query) || strings.Contains(query, "\n") {
		return Match{}, false
	}
	needle := strings.ToLower(query)
	line := 0
	for len(text) > 0 {
		cur, rest, more := strings.Cut(text, "\n")
		if strings.Contains(strings.ToLower(cur), needle) {
			return Match{Line: line, Text: cur}, true
		}
		if !more {
			break
		}
		text = rest
		line++
	}
	return Match{}, false
}

// Spans returns the byte ranges [start, end) of every non-overlapping,
// case-insensitive occurrence of query in line, in order. Offsets index the
// original line, so they stay correct when case folding changes byte lengths.
func Spans(line, query string) [][2]int {
	if Blank(query) {
		return nil
	}
	want := utf8.RuneCountInString(query)
	var spans [][2]int
	for i := 0; i < len(line); {
		end, ok := foldPrefixEnd(line[i:], query, want)
		if ok {
			spans = append(spans, [2]int{i, i + end})
			i += end
			continue
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		i += size
	}
	return spans
}

// foldPrefixEnd reports whether s starts with query under case folding, and
// the byte length of that prefix in s. runes is the query's rune count.
func foldPrefixEnd(s, query string, runes int) (int, bool) {
	end := 0
	for range runes {
		if end >= len(s) {
			return 0, false
		}
		_, size := utf8.DecodeRuneInString(s[end:])
		end += size
	}
	return end, strings.EqualFold(s[:end], query)
}

// Snippet prepares a matching line for a single-row label: whitespace runs
// collapse to one space and the ends are trimmed. When the match starts more
// than lead runes in, the text before it is cut to lead runes behind an
// ellipsis so the match stays visible in a narrow column.
func Snippet(line, query string, lead int) string {
	clean := strings.Join(strings.Fields(line), " ")
	spans := Spans(clean, strings.Join(strings.Fields(query), " "))
	if len(spans) == 0 {
		return clean
	}
	start := spans[0][0]
	before := []rune(clean[:start])
	if len(before) <= lead {
		return clean
	}
	return "…" + string(before[len(before)-lead:]) + clean[start:]
}
