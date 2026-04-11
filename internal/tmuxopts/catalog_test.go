package tmuxopts

import (
	"slices"
	"testing"
)

func TestDefaultLoads(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatalf("Default() error: %v", err)
	}
	if c == nil {
		t.Fatal("Default() returned nil catalog")
	}
	if c.SchemaVersion() == 0 {
		t.Errorf("expected non-zero schema version")
	}
	if len(c.OptionNames()) == 0 {
		t.Error("expected non-empty option names")
	}
	if len(c.HookNames()) == 0 {
		t.Error("expected non-empty hook names")
	}
	all := c.AllNames()
	if len(all) != len(c.OptionNames())+len(c.HookNames()) {
		t.Errorf("AllNames should be the union: got %d want %d", len(all), len(c.OptionNames())+len(c.HookNames()))
	}
}

func TestOptionNamesSorted(t *testing.T) {
	c := MustDefault()
	names := c.OptionNames()
	if !slices.IsSorted(names) {
		t.Error("OptionNames should be sorted")
	}
}

func TestLookupCanonical(t *testing.T) {
	c := MustDefault()
	opt, pseudo := c.Lookup("status")
	if pseudo != nil {
		t.Fatal("status should not resolve to pseudo-option")
	}
	if opt == nil {
		t.Fatal("status lookup returned nil")
	}
	if opt.Name != "status" {
		t.Errorf("expected name 'status', got %q", opt.Name)
	}
	if opt.Type != TypeChoice && opt.Type != TypeFlag {
		// status is historically a choice; guard against schema drift
		t.Logf("note: 'status' option type is %q", opt.Type)
	}
}

func TestLookupAliasNormalization(t *testing.T) {
	c := MustDefault()
	// display-panes-color is a documented alias for display-panes-colour.
	opt, pseudo := c.Lookup("display-panes-color")
	if pseudo != nil {
		t.Fatal("aliased name should resolve to a concrete option, not a pseudo")
	}
	if opt == nil {
		t.Fatal("expected alias lookup to succeed")
	}
	if opt.Name != "display-panes-colour" {
		t.Errorf("expected canonical 'display-panes-colour', got %q", opt.Name)
	}
}

func TestLookupArraySubscript(t *testing.T) {
	c := MustDefault()
	opt, _ := c.Lookup("command-alias[0]")
	if opt == nil {
		t.Fatal("expected command-alias[0] to resolve")
	}
	if opt.Name != "command-alias" {
		t.Errorf("expected canonical 'command-alias', got %q", opt.Name)
	}
	if !opt.Array {
		t.Error("expected command-alias to be array")
	}
}

func TestLookupUserOptionPseudo(t *testing.T) {
	c := MustDefault()
	opt, pseudo := c.Lookup("@my-custom")
	if opt != nil {
		t.Fatal("user option should not match a concrete entry")
	}
	if pseudo == nil {
		t.Fatal("expected pseudo-option for @-prefixed name")
	}
	if pseudo.Type == "" {
		t.Error("pseudo-option should have a type")
	}
}

func TestLookupUnknown(t *testing.T) {
	c := MustDefault()
	opt, pseudo := c.Lookup("nonsense-option-xyz")
	if opt != nil || pseudo != nil {
		t.Errorf("expected unknown lookup to return (nil, nil), got (%v, %v)", opt, pseudo)
	}
}

func TestValueCandidatesFlag(t *testing.T) {
	c := MustDefault()
	// mouse is a flag option.
	candidates, check := c.ValueCandidates("mouse")
	if len(candidates) == 0 {
		t.Fatal("expected flag candidates for 'mouse'")
	}
	gotValues := make([]string, len(candidates))
	for i, cand := range candidates {
		gotValues[i] = cand.Value
	}
	if !slices.Contains(gotValues, "on") || !slices.Contains(gotValues, "off") {
		t.Errorf("expected on/off in flag candidates, got %v", gotValues)
	}
	if check != CheckStaticExact {
		t.Errorf("expected static-exact for flag, got %q", check)
	}
}

func TestValueCandidatesChoice(t *testing.T) {
	c := MustDefault()
	// activity-action is a choice option with values none/any/current/other.
	candidates, check := c.ValueCandidates("activity-action")
	if len(candidates) < 4 {
		t.Fatalf("expected >=4 candidates, got %d", len(candidates))
	}
	got := make([]string, len(candidates))
	for i, cand := range candidates {
		got[i] = cand.Value
	}
	for _, want := range []string{"none", "any", "current", "other"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected choice %q in candidates, got %v", want, got)
		}
	}
	if check != CheckStaticExact {
		t.Errorf("expected static-exact for choice, got %q", check)
	}
}

func TestValueCandidatesColour(t *testing.T) {
	c := MustDefault()
	// display-panes-colour is a colour option.
	candidates, _ := c.ValueCandidates("display-panes-colour")
	if len(candidates) < 10 {
		t.Fatalf("expected many colour candidates, got %d", len(candidates))
	}
	got := make([]string, len(candidates))
	for i, cand := range candidates {
		got[i] = cand.Value
	}
	for _, want := range []string{"red", "blue", "black", "white"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected basic colour %q in candidates", want)
		}
	}
}

func TestValueCandidatesDynamic(t *testing.T) {
	c := MustDefault()
	// status-format is an array of format strings — freeform.
	candidates, check := c.ValueCandidates("status-format")
	if len(candidates) != 0 {
		t.Errorf("expected no static candidates for freeform option, got %v", candidates)
	}
	if check == CheckStaticExact {
		t.Errorf("expected non-exact checkability for freeform option, got %q", check)
	}
}

func TestValueCandidatesUserOption(t *testing.T) {
	c := MustDefault()
	candidates, check := c.ValueCandidates("@anything")
	if len(candidates) != 0 {
		t.Errorf("expected no candidates for user option, got %v", candidates)
	}
	if check != CheckDynamic {
		t.Errorf("expected dynamic checkability for user option, got %q", check)
	}
}

func TestValueHintNumeric(t *testing.T) {
	c := MustDefault()
	hint := c.ValueHint("base-index")
	if hint == "" {
		t.Error("expected non-empty hint for numeric option")
	}
}

func TestOptionSummary(t *testing.T) {
	c := MustDefault()
	s := c.OptionSummary("mouse")
	if s == "" {
		t.Error("expected non-empty summary for 'mouse'")
	}
}

func TestCanonicalizeIdempotent(t *testing.T) {
	c := MustDefault()
	if got := c.Canonicalize("status"); got != "status" {
		t.Errorf("Canonicalize('status') = %q, want 'status'", got)
	}
	if got := c.Canonicalize("status[0]"); got != "status" {
		t.Errorf("Canonicalize('status[0]') = %q, want 'status'", got)
	}
}

func TestIsKnown(t *testing.T) {
	c := MustDefault()
	if !c.IsKnown("mouse") {
		t.Error("expected mouse to be known")
	}
	if !c.IsKnown("@user") {
		t.Error("expected @user pseudo to be known")
	}
	if c.IsKnown("nonsense-xyz") {
		t.Error("expected nonsense-xyz to be unknown")
	}
}
