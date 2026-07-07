package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/extract"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
)

// ctrlF constructs the ctrl+f key press message. Verified against the
// bubbletea v2 vendor (charmbracelet/ultraviolet key.go Keystroke()): with an
// empty Text field, String() falls back to Keystroke(), which prefixes
// "ctrl+" for ModCtrl and appends the rune for Code 'f', yielding "ctrl+f".
func ctrlF() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl}
}

func TestExtractCycleAdvancesCategoryAndReloads(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "open https://example.com file internal/x.go", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	if got := h.Model().extractCategory; got != extract.Word {
		t.Fatalf("initial category = %v, want word", got)
	}
	current := h.Model().currentLevel()
	if current == nil || current.ID != extractLevelID {
		t.Fatalf("expected extract level to be current, got %+v", current)
	}

	h.Send(ctrlF())

	if got := h.Model().extractCategory; got != extract.Path {
		t.Fatalf("after ctrl+f category = %v, want path", got)
	}

	current = h.Model().currentLevel()
	found := false
	for _, item := range current.Items {
		if item.ID == "internal/x.go" {
			found = true
			break
		}
	}
	if !found {
		ids := make([]string, 0, len(current.Items))
		for _, item := range current.Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("path items missing internal/x.go: %v", ids)
	}
}

func TestExtractCyclePreservesFilterQuery(t *testing.T) {
	restore := menu.SetExtractCaptureForTest(func(sock, target string) (string, error) {
		return "alpha https://alpha.dev/x beta internal/beta.go", nil
	})
	defer restore()

	m := NewModel(ModelConfig{Width: 80, Height: 24, RootMenu: "extract", SocketPath: "test.sock"})
	h := NewHarness(m)

	current := h.Model().currentLevel()
	current.SetFilter("be", 2)

	h.Send(ctrlF())

	if got := h.Model().currentLevel().Filter; got != "be" {
		t.Fatalf("filter after cycle = %q, want %q", got, "be")
	}
}
