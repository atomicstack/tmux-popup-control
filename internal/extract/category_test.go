package extract

import "testing"

func TestCategoryString(t *testing.T) {
	cases := map[Category]string{
		Word: "word", Path: "path", URL: "url", Quote: "quote",
		SQuote: "s-quote", Line: "line", All: "all",
	}
	for c, want := range cases {
		if got := c.String(); got != want {
			t.Errorf("Category(%d).String() = %q, want %q", c, got, want)
		}
	}
}

func TestCategoryNextWraps(t *testing.T) {
	order := []Category{Word, Path, URL, Quote, SQuote, Line, All, Word}
	got := Word
	for i := 1; i < len(order); i++ {
		got = got.Next()
		if got != order[i] {
			t.Fatalf("Next() step %d = %v, want %v", i, got, order[i])
		}
	}
}

func TestDefaultCategory(t *testing.T) {
	if DefaultCategory != Word {
		t.Fatalf("DefaultCategory = %v, want Word", DefaultCategory)
	}
}
