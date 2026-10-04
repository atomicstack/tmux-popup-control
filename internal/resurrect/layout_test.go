package resurrect

import (
	"encoding/json"
	"strings"
	"testing"
)

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

// tmux next-3.9 layout strings are a JSON subset: {"V":2,"L":{...}} where each
// pane cell carries its source pane id under "I". Restoring onto freshly split
// panes must not pin cells to ids from the saved server, so the ids are
// stripped; tmux then assigns panes to cells in order.
const jsonLayoutWithIDs = `{"V":2,"L":{"t":"h","w":120,"h":40,"x":0,"y":0,"c":[{"t":"p","w":60,"h":40,"x":0,"y":0,"l":1,"i":0,"I":"%0"},{"t":"p","w":59,"h":40,"x":61,"y":0,"l":0,"i":1,"I":"%1"},{"t":"p","w":28,"h":6,"x":6,"y":6,"a":true,"i":2,"z":0,"I":"%2"}]}}`

func TestSelectableLayoutStripsPaneIDsFromJSONLayout(t *testing.T) {
	got := selectableLayout(jsonLayoutWithIDs)
	if strings.Contains(got, `"I"`) {
		t.Fatalf("selectableLayout() kept pane ids: %s", got)
	}
	var want, have any
	if err := json.Unmarshal([]byte(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(jsonLayoutWithIDs, `,"I":"%0"`, ""), `,"I":"%1"`, ""), `,"I":"%2"`, "")), &want); err != nil {
		t.Fatalf("want: %v", err)
	}
	if err := json.Unmarshal([]byte(got), &have); err != nil {
		t.Fatalf("selectableLayout() produced invalid json %q: %v", got, err)
	}
	if fmtJSON(t, have) != fmtJSON(t, want) {
		t.Fatalf("selectableLayout() = %s, want %s", fmtJSON(t, have), fmtJSON(t, want))
	}
	if strings.ContainsAny(got, " \n\t") {
		t.Fatalf("selectableLayout() must stay whitespace-free for the control-mode line protocol: %q", got)
	}
}

func TestSelectableLayoutLeavesMalformedJSONUnchanged(t *testing.T) {
	input := `{"V":2,"L":{"t":"h"`
	if got := selectableLayout(input); got != input {
		t.Fatalf("selectableLayout() = %q, want %q", got, input)
	}
}

func TestIsJSONLayout(t *testing.T) {
	if !isJSONLayout(" " + jsonLayoutWithIDs) {
		t.Fatal("expected json layout to be detected")
	}
	if isJSONLayout("5dcb,179x58,0,0{89x58,0,0,89x58,90,0}") || isJSONLayout("tiled") {
		t.Fatal("v1 and named layouts are not json layouts")
	}
}

func fmtJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDropLayoutPanesCollapsesSingleChildNode(t *testing.T) {
	layout := `{"L":{"c":[{"h":66,"i":0,"t":"p","w":195,"x":0,"y":0},{"a":true,"h":50,"i":1,"t":"p","w":173,"x":11,"y":7,"z":0}],"h":66,"t":"v","w":195,"x":0,"y":0},"V":2}`
	got := dropLayoutPanes(layout, map[int]bool{1: true})
	if want := `{"L":{"h":66,"i":0,"t":"p","w":195,"x":0,"y":0},"V":2}`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestDropLayoutPanesKeepsMultiChildNode(t *testing.T) {
	layout := `{"L":{"c":[{"h":66,"i":0,"t":"p","w":97,"x":0,"y":0},{"h":66,"i":1,"t":"p","w":97,"x":98,"y":0},{"h":20,"i":2,"t":"p","w":40,"x":5,"y":5,"z":0}],"h":66,"t":"h","w":195,"x":0,"y":0},"V":2}`
	got := dropLayoutPanes(layout, map[int]bool{2: true})
	if want := `{"L":{"c":[{"h":66,"i":0,"t":"p","w":97,"x":0,"y":0},{"h":66,"i":1,"t":"p","w":97,"x":98,"y":0}],"h":66,"t":"h","w":195,"x":0,"y":0},"V":2}`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestDropLayoutPanesLeavesV1LayoutUnchanged(t *testing.T) {
	// v1 layouts never carry floating cells, so there is nothing to drop.
	layout := "b25f,80x24,0,0,1"
	if got := dropLayoutPanes(layout, map[int]bool{1: true}); got != layout {
		t.Fatalf("got %s, want unchanged", got)
	}
}
