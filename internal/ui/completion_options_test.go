package ui

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
)

func completionValues(h *Harness) []string {
	if h.model.completion == nil {
		return nil
	}
	out := make([]string, 0, len(h.model.completion.filtered))
	for _, item := range h.model.completion.filtered {
		out = append(out, item.Value)
	}
	return out
}

func TestCompletionOffersOptionNamesForSetOption(t *testing.T) {
	h := setupCommandHarness(t)

	// Type a leading character to commit to positional territory; without
	// it, cmdparse.Analyse offers flag completion because set-option still
	// has unused flags.
	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "m")

	if !h.model.completionVisible() {
		t.Fatal("expected option-name completion visible after 'set-option m'")
	}
	if got := h.model.completion.argType; got != "option" {
		t.Fatalf("expected argType 'option', got %q", got)
	}
	values := completionValues(h)
	if !slices.Contains(values, "mouse") {
		t.Errorf("expected 'mouse' in option candidates, got %d items: %v", len(values), values)
	}
}

func TestCompletionOffersOptionNamesForSetWindowOption(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "set-window-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "m")

	if !h.model.completionVisible() {
		t.Fatal("expected option-name completion visible after 'set-window-option m'")
	}
	if got := h.model.completion.argType; got != "option" {
		t.Fatalf("expected argType 'option', got %q", got)
	}
	values := completionValues(h)
	if len(values) == 0 {
		t.Fatal("expected non-empty option candidates")
	}
}

func TestCompletionFiltersOptionNamesByPrefix(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "mou")

	if !h.model.completionVisible() {
		t.Fatal("expected completion still visible while filtering option name")
	}
	values := completionValues(h)
	if !slices.Contains(values, "mouse") {
		t.Errorf("expected 'mouse' after filtering with 'mou', got %v", values)
	}
}

func TestCompletionOffersFlagValuesForSetOption(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "mouse")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	if !h.model.completionVisible() {
		t.Fatal("expected value completion visible after 'set-option mouse '")
	}
	if got := h.model.completion.argType; got != "value" {
		t.Fatalf("expected argType 'value', got %q", got)
	}
	values := completionValues(h)
	if !slices.Contains(values, "on") || !slices.Contains(values, "off") {
		t.Errorf("expected on/off in value candidates, got %v", values)
	}
}

func TestCompletionOffersChoiceValuesForSetOption(t *testing.T) {
	h := setupCommandHarness(t)

	// activity-action is a choice option with none/any/current/other.
	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "activity-action")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	if !h.model.completionVisible() {
		t.Fatal("expected choice value completion visible after 'set-option activity-action '")
	}
	values := completionValues(h)
	for _, want := range []string{"none", "any", "current", "other"} {
		if !slices.Contains(values, want) {
			t.Errorf("expected %q in choice candidates, got %v", want, values)
		}
	}
}

func TestCompletionFreeformValueShowsPlaceholder(t *testing.T) {
	h := setupCommandHarness(t)

	// status-format is a freeform (format-string) option — no static candidates.
	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "status-format")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	if h.model.completionVisible() {
		t.Fatal("expected dropdown NOT visible for freeform value")
	}
	if h.model.completion == nil {
		t.Fatal("expected placeholder completion state (non-nil) for freeform value")
	}
	if h.model.completion.argType != "value" {
		t.Fatalf("expected argType 'value' in placeholder state, got %q", h.model.completion.argType)
	}
	if h.model.completion.typeLabel == "" {
		t.Error("expected non-empty typeLabel hint for freeform value")
	}
}

func TestCompletionUserOptionHasNoStaticValues(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "set-option")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "@myvar")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})

	if h.model.completionVisible() {
		t.Fatal("expected no static candidates for @user option value")
	}
	if h.model.completion == nil {
		t.Fatal("expected placeholder state for user option value")
	}
	if h.model.completion.argType != "value" {
		t.Fatalf("expected argType 'value' for user option value, got %q", h.model.completion.argType)
	}
}

func TestCompletionOffersHookNamesForSetHook(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "set-hook")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "a")

	if !h.model.completionVisible() {
		t.Fatal("expected hook-name completion visible after 'set-hook a'")
	}
	if got := h.model.completion.argType; got != "hook" {
		t.Fatalf("expected argType 'hook', got %q", got)
	}
	values := completionValues(h)
	if len(values) == 0 {
		t.Fatal("expected non-empty hook candidates")
	}
}

func TestCompletionOffersOptionNamesForShowOptions(t *testing.T) {
	h := setupCommandHarness(t)

	sendKeys(h, "show-options")
	h.Send(tea.KeyPressMsg{Code: tea.KeySpace})
	sendKeys(h, "m")

	if !h.model.completionVisible() {
		t.Fatal("expected option-name completion visible after 'show-options m'")
	}
	if got := h.model.completion.argType; got != "option" {
		t.Fatalf("expected argType 'option', got %q", got)
	}
}

func TestPrecedingPositionalWalksFlags(t *testing.T) {
	schemas := cmdparse.BuildRegistry([]string{
		"set-option (set) [-aFgopqsuUw] [-t target-pane] option [value]",
	})
	schema := schemas["set-option"]
	if schema == nil {
		t.Fatal("expected set-option schema")
	}

	cases := []struct {
		filter string
		idx    int
		want   string
	}{
		{"set-option mouse on", 0, "mouse"},
		{"set-option -g mouse on", 0, "mouse"},
		{"set-option -t main mouse on", 0, "mouse"},
		{"set-option -g -t main mouse on", 0, "mouse"},
		{"set-option", 0, ""},
		{"set-option -g", 0, ""},
	}
	for _, tc := range cases {
		got := precedingPositional(schema, tc.filter, tc.idx)
		if got != tc.want {
			t.Errorf("precedingPositional(%q, %d) = %q, want %q", tc.filter, tc.idx, got, tc.want)
		}
	}
}
