// Package extract implements extrakto-style token extraction from captured
// pane text. It is consumer-agnostic: no bubbletea/tmux/menu imports.
package extract

// Category selects which kind of token Extract produces.
type Category int

const (
	Word Category = iota
	Path
	URL
	Quote
	SQuote
	Line
	Host
	Quoted
	All
)

// DefaultCategory is the category shown when the extract view first opens.
const DefaultCategory = Word

// order is the ctrl-f cycle order. Quote-family modes (quote, s-quote, quoted)
// are grouped together, url/host near the end before all.
var order = []Category{Word, Path, Line, Quote, SQuote, Quoted, URL, Host, All}

// Categories returns the category cycle order
// (word→path→line→quote→s-quote→quoted→url→host→all).
// Callers get a copy, so mutating the returned slice cannot corrupt the
// package-level cycle order used by Next().
func Categories() []Category { return append([]Category(nil), order...) }

func (c Category) String() string {
	switch c {
	case Word:
		return "word"
	case Path:
		return "path"
	case URL:
		return "url"
	case Quote:
		return "quote"
	case SQuote:
		return "s-quote"
	case Line:
		return "line"
	case Host:
		return "host"
	case Quoted:
		return "quoted"
	case All:
		return "all"
	default:
		return "word"
	}
}

// Next returns the next category in cycle order, wrapping after All.
func (c Category) Next() Category {
	for i, o := range order {
		if o == c {
			return order[(i+1)%len(order)]
		}
	}
	return DefaultCategory
}
