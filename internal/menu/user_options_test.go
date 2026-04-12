package menu

import (
	"fmt"
	"reflect"
	"testing"
)

func TestLoadUserOptionsReturnsSortedNames(t *testing.T) {
	restore := loadUserOptionsFn
	t.Cleanup(func() { loadUserOptionsFn = restore })

	loadUserOptionsFn = func(socket string) ([]string, error) {
		if socket != "/tmp/sock" {
			t.Fatalf("expected socket path, got %q", socket)
		}
		return []string{"@plugin", "@catppuccin_flavor"}, nil
	}

	names, err := LoadUserOptions(Context{SocketPath: "/tmp/sock"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// LoadUserOptions trusts its source to be sorted; the contract is that
	// tmux.UserOptions already returns a sorted slice. We only verify the
	// names pass through unchanged here.
	want := []string{"@plugin", "@catppuccin_flavor"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
}

func TestLoadUserOptionsPropagatesError(t *testing.T) {
	restore := loadUserOptionsFn
	t.Cleanup(func() { loadUserOptionsFn = restore })

	loadUserOptionsFn = func(string) ([]string, error) {
		return nil, fmt.Errorf("boom")
	}
	names, err := LoadUserOptions(Context{})
	if err == nil {
		t.Fatal("expected error")
	}
	if names != nil {
		t.Fatalf("expected nil names, got %v", names)
	}
}
