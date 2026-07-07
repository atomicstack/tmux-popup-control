package menu

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/extract"
)

func TestLoadExtractMenuWord(t *testing.T) {
	orig := extractCaptureFn
	extractCaptureFn = func(socket, target string) (string, error) {
		return "please make build", nil
	}
	defer func() { extractCaptureFn = orig }()

	items, err := loadExtractMenu(Context{ExtractCategory: extract.Word})
	if err != nil {
		t.Fatalf("loadExtractMenu: %v", err)
	}
	got := make([]string, len(items))
	for i, it := range items {
		got[i] = it.Label
	}
	// reverse order, min length 5.
	want := []string{"build", "please"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("labels = %v, want %v", got, want)
	}
	// item ID equals the token text (used verbatim by insert/copy).
	if items[0].ID != "build" {
		t.Fatalf("item[0].ID = %q, want %q", items[0].ID, "build")
	}
}

func TestExtractRegisteredAsCategory(t *testing.T) {
	if _, ok := CategoryLoaders()["extract"]; !ok {
		t.Fatal("extract not in CategoryLoaders")
	}
	found := false
	for _, it := range RootItems() {
		if it.ID == "extract" {
			found = true
		}
	}
	if !found {
		t.Fatal("extract not in RootItems")
	}
}
