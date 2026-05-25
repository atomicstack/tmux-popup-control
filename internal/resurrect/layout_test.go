package resurrect

import "testing"

func TestSelectableLayoutLeavesNamedLayoutUnchanged(t *testing.T) {
	if got := selectableLayout(" tiled "); got != "tiled" {
		t.Fatalf("selectableLayout() = %q, want %q", got, "tiled")
	}
}

func TestSelectableLayoutRemovesSavedPaneIDs(t *testing.T) {
	input := "0917,179x58,0,0,11"
	want := "49df,179x58,0,0"
	if got := selectableLayout(input); got != want {
		t.Fatalf("selectableLayout() = %q, want %q", got, want)
	}
}

func TestSelectableLayoutRemovesVisibleSuffixAndPaneIDs(t *testing.T) {
	input := "0ad5,179x58,0,0{89x58,0,0,37,89x58,90,0,39}<89x58,0,0,37,89x58,90,0,39>"
	want := "5dcb,179x58,0,0{89x58,0,0,89x58,90,0}"
	if got := selectableLayout(input); got != want {
		t.Fatalf("selectableLayout() = %q, want %q", got, want)
	}
}
