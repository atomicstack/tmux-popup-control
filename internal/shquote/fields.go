package shquote

// Fields splits s into tokens using tmux's quoting rules:
//
//   - Single quotes: content is literal, no escape processing.
//   - Double quotes: content is grouped, backslash escapes the next byte.
//   - Backslash outside quotes: escapes the next byte.
//   - Unquoted whitespace separates tokens.
//
// Quotes are stripped from the returned values. This mirrors the
// tokenisation behaviour of the tmux lexer (cmd-parse.y) for
// interactive command input.
func Fields(s string) []string {
	var tokens []string
	i := 0
	for i < len(s) {
		// skip whitespace
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}
		tok, end := scanToken(s, i)
		tokens = append(tokens, tok)
		i = end
	}
	return tokens
}

// TrailingSpace reports whether the input has unquoted whitespace after
// the last token, i.e. the user has finished typing a token and moved
// on. An empty or whitespace-only string returns true.
func TrailingSpace(s string) bool {
	if s == "" {
		return true
	}
	i := 0
	lastTokenEnd := 0
	for i < len(s) {
		if s[i] == ' ' || s[i] == '\t' {
			i++
			continue
		}
		_, end := scanToken(s, i)
		lastTokenEnd = end
		i = end
	}
	return lastTokenEnd < len(s)
}

// scanToken reads one token starting at position start and returns the
// unquoted token value and the byte offset immediately after the token.
func scanToken(s string, start int) (string, int) {
	var buf []byte
	i := start

	const (
		qNone   = 0
		qSingle = 1
		qDouble = 2
	)
	state := qNone

	for i < len(s) {
		ch := s[i]

		// unquoted whitespace ends the token
		if state == qNone && (ch == ' ' || ch == '\t') {
			break
		}

		// backslash escaping (not inside single quotes)
		if ch == '\\' && state != qSingle {
			i++
			if i < len(s) {
				buf = append(buf, s[i])
				i++
			}
			continue
		}

		// quote transitions
		if ch == '\'' {
			if state == qNone {
				state = qSingle
				i++
				continue
			}
			if state == qSingle {
				state = qNone
				i++
				continue
			}
		}
		if ch == '"' {
			if state == qNone {
				state = qDouble
				i++
				continue
			}
			if state == qDouble {
				state = qNone
				i++
				continue
			}
		}

		buf = append(buf, ch)
		i++
	}
	return string(buf), i
}
