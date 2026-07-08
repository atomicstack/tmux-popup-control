package extract

import (
	"sort"
	"strings"
)

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
	case Host:
		return finalize(extractHosts(text))
	case Quoted:
		return finalize(extractQuoted(text))
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

// extractHosts derives bare hostnames from both scheme-form (scheme://host)
// and scp-form (user@host:path) URLs, combined in true source order so
// finalize's reverse yields most-recent-on-screen first regardless of which
// form each host came from.
func extractHosts(text string) []Token {
	src := "\n" + text
	type hit struct {
		pos  int
		host string
	}
	var hits []hit
	for _, m := range reHostScheme.FindAllStringSubmatchIndex(src, -1) {
		hits = append(hits, hit{pos: m[0], host: src[m[2]:m[3]]}) // group 1
	}
	for _, m := range reHostSCP.FindAllStringSubmatchIndex(src, -1) {
		hits = append(hits, hit{pos: m[0], host: src[m[4]:m[5]]}) // group 2
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].pos < hits[j].pos })
	var out []Token
	for _, h := range hits {
		if len([]rune(h.host)) < defaultMinLength {
			continue
		}
		out = append(out, Token{Text: h.host, Category: Host})
	}
	return out
}

// extractQuoted returns the inner text (quotes stripped) of both double- and
// single-quoted spans, combined into a single category.
func extractQuoted(text string) []Token {
	var out []Token
	out = append(out, runFilter(text, filterDef{re: reQuoteInner, minLen: defaultMinLength}, Quoted)...)
	out = append(out, runFilter(text, filterDef{re: reSQuoteInner, minLen: defaultMinLength}, Quoted)...)
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
