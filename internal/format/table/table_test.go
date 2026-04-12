package table

import "testing"

func TestFormatReturnsNilForEmptyRows(t *testing.T) {
	if got := Format(nil, nil); got != nil {
		t.Fatalf("Format(nil, nil) = %#v, want nil", got)
	}
}

func TestFormatAlignsColumns(t *testing.T) {
	rows := [][]string{
		{"name", "count", "π"},
		{"alpha", "2", "12"},
		{"β", "10", "3"},
	}

	got := Format(rows, []Alignment{AlignLeft, AlignRight, AlignRight})
	want := []string{
		"name   count   π",
		"alpha      2  12",
		"β         10   3",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestFormatHandlesRaggedRows(t *testing.T) {
	rows := [][]string{
		{"name", "count"},
		{"alpha"},
		{"beta", "3", "extra"},
	}

	got := Format(rows, []Alignment{AlignLeft, AlignRight, AlignLeft})
	want := []string{
		"name   count",
		"alpha",
		"beta       3  extra",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d: got %q want %q", i, got[i], want[i])
		}
	}
}
