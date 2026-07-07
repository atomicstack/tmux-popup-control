package extract

import "strings"

// Token is one extracted candidate.
type Token struct {
	Text     string
	Category Category
}

// Extract returns deduped, reverse-ordered (most-recent-on-screen first)
// tokens of the requested category from text.
func Extract(text string, cat Category) []Token {
	switch cat {
	case Line:
		return finalize(extractLines(text))
	case All:
		return extractAll(text)
	default:
		def, ok := filters()[cat]
		if !ok {
			return nil
		}
		return finalize(runFilter(text, def, cat))
	}
}

// runFilter applies one filterDef, returning post-processed token texts in
// source order (pre-dedup, pre-reverse).
func runFilter(text string, def filterDef, cat Category) []Token {
	var out []Token
	matches := def.re.FindAllStringSubmatch("\n"+text, -1)
	for _, m := range matches {
		item := strings.Join(nonEmpty(m[1:]), "")
		if def.lstrip != "" {
			item = strings.TrimLeft(item, def.lstrip)
		}
		if def.rstrip != "" {
			item = strings.TrimRight(item, def.rstrip)
		}
		if len([]rune(item)) < def.minLen {
			continue
		}
		if def.exclude != nil && def.exclude.MatchString(item) {
			continue
		}
		out = append(out, Token{Text: item, Category: cat})
	}
	return out
}

func extractLines(text string) []Token {
	var out []Token
	for _, ln := range strings.Split(text, "\n") {
		ln = strings.TrimSpace(ln)
		if len([]rune(ln)) < defaultMinLength {
			continue
		}
		out = append(out, Token{Text: ln, Category: Line})
	}
	return out
}

func extractAll(text string) []Token {
	var out []Token
	for _, cat := range []Category{Path, URL, Quote, SQuote} {
		def := filters()[cat]
		if !def.inAll {
			continue
		}
		out = append(out, runFilter(text, def, cat)...)
	}
	return finalize(out)
}

// finalize dedups (order-preserving) then reverses so the most-recent token
// on screen sorts first, matching extrakto's res.reverse().
func finalize(in []Token) []Token {
	seen := make(map[string]struct{}, len(in))
	deduped := make([]Token, 0, len(in))
	for _, t := range in {
		if _, ok := seen[t.Text]; ok {
			continue
		}
		seen[t.Text] = struct{}{}
		deduped = append(deduped, t)
	}
	for i, j := 0, len(deduped)-1; i < j; i, j = i+1, j-1 {
		deduped[i], deduped[j] = deduped[j], deduped[i]
	}
	return deduped
}

func nonEmpty(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
