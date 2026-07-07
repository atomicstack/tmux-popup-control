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

// TestCategoriesMatchesCycle ensures Categories() is the single source of
// truth for the ctrl-f cycle order (used by both Category.Next() and the UI
// header rendering), and that callers cannot mutate package state through
// the returned slice.
func TestCategoriesMatchesCycle(t *testing.T) {
	want := []Category{Word, Path, URL, Quote, SQuote, Line, All}
	got := Categories()
	if len(got) != len(want) {
		t.Fatalf("Categories() = %v, want %v", got, want)
	}
	for i, c := range want {
		if got[i] != c {
			t.Fatalf("Categories()[%d] = %v, want %v", i, got[i], c)
		}
	}

	// Mutating the returned slice must not affect a second call.
	got[0] = All
	second := Categories()
	if second[0] != Word {
		t.Fatalf("Categories() copy semantics broken: mutating first result changed second call: %v", second)
	}
}
