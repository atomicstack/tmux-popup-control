package state

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

func TestSessionStoreClonesEntries(t *testing.T) {
	store := NewSessionStore()
	input := []menu.SessionEntry{{Name: "main", Label: "main", Clients: []string{"c1"}}}
	store.SetEntries(input)
	input[0].Name = "mutated"
	input[0].Clients[0] = "changed"

	got := store.Entries()
	if got[0].Name != "main" {
		t.Fatalf("store should keep original name, got %q", got[0].Name)
	}
	if got[0].Clients[0] != "c1" {
		t.Fatalf("store should deep-clone nested client slices, got %q", got[0].Clients[0])
	}
	got[0].Label = "edited"
	gotAgain := store.Entries()
	if gotAgain[0].Label != "main" {
		t.Fatalf("Entries should return a clone, got %q", gotAgain[0].Label)
	}
}

func TestWindowStoreTracksCurrentAndClonesEntries(t *testing.T) {
	store := NewWindowStore()
	input := []menu.WindowEntry{{ID: "main:1", Label: "editor"}}
	store.SetEntries(input)
	input[0].Label = "mutated"
	store.SetCurrent("main:1", "editor", "main")
	store.SetIncludeCurrent(false)

	got := store.Entries()
	if got[0].Label != "editor" {
		t.Fatalf("store should keep original label, got %q", got[0].Label)
	}
	if store.CurrentID() != "main:1" || store.CurrentLabel() != "editor" || store.CurrentSession() != "main" {
		t.Fatalf("unexpected current window state: id=%q label=%q session=%q", store.CurrentID(), store.CurrentLabel(), store.CurrentSession())
	}
	if store.IncludeCurrent() {
		t.Fatal("expected include current to be false")
	}
}

func TestPaneStoreTracksCurrentAndClonesEntries(t *testing.T) {
	store := NewPaneStore()
	input := []menu.PaneEntry{{ID: "%1", Label: "shell"}}
	store.SetEntries(input)
	input[0].Label = "mutated"
	store.SetCurrent("%1", "shell")
	store.SetIncludeCurrent(false)

	got := store.Entries()
	if got[0].Label != "shell" {
		t.Fatalf("store should keep original label, got %q", got[0].Label)
	}
	if store.CurrentID() != "%1" || store.CurrentLabel() != "shell" {
		t.Fatalf("unexpected current pane state: id=%q label=%q", store.CurrentID(), store.CurrentLabel())
	}
	if store.IncludeCurrent() {
		t.Fatal("expected include current to be false")
	}
}
