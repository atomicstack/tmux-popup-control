package tmux

import (
	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
	"testing"
)

func TestLinkedWindowSnapshotPreservesLinkContext(t *testing.T) {
	t.Setenv("TMUX_POPUP_CONTROL_SESSION_ID", "$1")
	t.Setenv("TMUX_POPUP_CONTROL_SWITCH_CURRENT", "1")
	fake := &fakeClient{
		sessions: []*gotmux.Session{{Id: "$1", Name: "alpha"}, {Id: "$2", Name: "beta"}},
		windows: []*gotmux.Window{
			{Id: "@1", Session: "alpha", Index: 1, Name: "shared", Active: true, ActiveSessionsList: []string{"alpha", "beta"}, LinkedSessionsList: []string{"alpha", "beta"}},
			{Id: "@1", Session: "beta", Index: 7, Name: "shared", Active: true, ActiveSessionsList: []string{"alpha", "beta"}, LinkedSessionsList: []string{"alpha", "beta"}},
		},
		listWindowsFormatLines: []string{"@1\t$1\talpha\t1\talpha:1\talpha:1: shared", "@1\t$2\tbeta\t7\tbeta:7\tbeta:7: shared"},
	}
	withStubTmux(t, func(string) (tmuxClient, error) { return fake, nil })
	withStubCommander(t, func(string, ...string) commander { return stubCommander{} })
	snap, err := FetchWindows("review-socket")
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Windows) != 2 {
		t.Fatalf("expected two links, got %d", len(snap.Windows))
	}
	for i, want := range []struct {
		id, session string
		index       int
		current     bool
	}{
		{"alpha:1", "alpha", 1, true},
		{"beta:7", "beta", 7, false},
	} {
		got := snap.Windows[i]
		if got.ID != want.id || got.Session != want.session || got.Index != want.index || got.Current != want.current {
			t.Errorf("link %s: got session=%q index=%d current=%v; want session=%q index=%d current=%v", want.id, got.Session, got.Index, got.Current, want.session, want.index, want.current)
		}
	}
	if snap.CurrentID != "alpha:1" {
		t.Errorf("current window: got %q; want alpha:1", snap.CurrentID)
	}
}
