package extract

import "regexp"

const defaultMinLength = 5

// filterDef is a regex-based extraction rule. Ported from extrakto.conf.
type filterDef struct {
	re      *regexp.Regexp
	exclude *regexp.Regexp // optional; drop matches that match this
	lstrip  string         // cutset trimmed from the left
	rstrip  string         // cutset trimmed from the right
	minLen  int
	inAll   bool // included in the All union
}

var (
	// word: anything except brackets, =, $, box-drawing/symbol ranges, editor
	// glyphs and whitespace. Excluded from All (extrakto in_all:false).
	reWord = regexp.MustCompile(`([^][(){}=$─-➿-⋅↴│ \t\n\r]+)`)
	// path: a token containing '/', optionally rooted at ~ or /.
	rePath   = regexp.MustCompile(`(?:[ \t\n"([<':]|^)(~|/)?([-~a-zA-Z0-9_+-,.]+/[^ \t\n\r|:"'$%&)>\]]*)`)
	rePathEx = regexp.MustCompile(`[kmgKMG]/s$|^\d+/\d+$`)
	// url: scheme + body.
	reURL = regexp.MustCompile(`(https?://|git@|git://|ssh://|s*ftp://|file:///)([a-zA-Z0-9?=%/_.:,;~@!#$&()*+-]*)`)
	// quote / s-quote: the quoted span including the surrounding quotes.
	reQuote  = regexp.MustCompile(`("[^"\n\r]+")`)
	reSQuote = regexp.MustCompile(`('[^'\n\r]+')`)
)

func filters() map[Category]filterDef {
	return map[Category]filterDef{
		Word:   {re: reWord, lstrip: `,:;()[]{}<>'"|`, rstrip: `,:;()[]{}<>'"|.`, minLen: defaultMinLength, inAll: false},
		Path:   {re: rePath, exclude: rePathEx, rstrip: `,):`, minLen: defaultMinLength, inAll: true},
		URL:    {re: reURL, rstrip: `,):`, minLen: defaultMinLength, inAll: true},
		Quote:  {re: reQuote, minLen: defaultMinLength, inAll: true},
		SQuote: {re: reSQuote, minLen: defaultMinLength, inAll: true},
	}
}
