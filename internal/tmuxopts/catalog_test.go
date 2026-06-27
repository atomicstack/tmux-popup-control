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
		t.Fatalf("status type drifted to %q", opt.Type)
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

func TestValueCandidatesColourOptionField(t *testing.T) {
	c := MustDefault()
	// cursor-colour is Type=="string" with ColourOption==true in the
	// refreshed catalog (it used to be Type=="colour"). It must still yield
	// the full colour candidate list via the colour_option signal.
	opt, _ := c.Lookup("cursor-colour")
	if opt == nil {
		t.Fatal("expected cursor-colour in catalog")
	}
	if opt.Type == TypeColour {
		t.Fatalf("expected cursor-colour to be re-typed away from colour, got %q", opt.Type)
	}
	if !opt.IsColour() {
		t.Fatal("expected cursor-colour.IsColour() to be true via colour_option")
	}
	candidates, _ := c.ValueCandidates("cursor-colour")
	if len(candidates) < 10 {
		t.Fatalf("expected many colour candidates for cursor-colour, got %d", len(candidates))
	}
	got := make([]string, len(candidates))
	for i, cand := range candidates {
		got[i] = cand.Value
	}
	for _, want := range []string{"red", "blue", "black", "white"} {
		if !slices.Contains(got, want) {
			t.Errorf("expected basic colour %q in cursor-colour candidates", want)
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

func TestTmuxCommands(t *testing.T) {
	c := MustDefault()
	cmds := c.TmuxCommands()
	if len(cmds) == 0 {
		t.Fatal("expected non-empty tmux command list")
	}
	names := make([]string, len(cmds))
	var hasNewSession, hasAlias bool
	for i, cmd := range cmds {
		if cmd.Name == "" {
			t.Errorf("found tmux command with empty name: %+v", cmd)
		}
		names[i] = cmd.Name
		if cmd.Name == "new-session" {
			hasNewSession = true
		}
		if cmd.Alias != "" {
			hasAlias = true
		}
	}
	if !slices.IsSorted(names) {
		t.Error("expected TmuxCommands to be sorted by name")
	}
	if !hasNewSession {
		t.Error("expected new-session in tmux command list")
	}
	if !hasAlias {
		t.Error("expected at least one tmux command to have an alias")
	}
}

func TestFormatVariables(t *testing.T) {
	c := MustDefault()
	vars := c.FormatVariables()
	if len(vars) == 0 {
		t.Fatal("expected non-empty format variable list")
	}
	names := make([]string, len(vars))
	want := map[string]bool{"pane_id": false, "session_name": false, "window_index": false}
	for i, v := range vars {
		if v.Name == "" {
			t.Errorf("found format variable with empty name: %+v", v)
		}
		if v.ValueType == "" {
			t.Errorf("format variable %q has empty value_type", v.Name)
		}
		names[i] = v.Name
		if _, ok := want[v.Name]; ok {
			want[v.Name] = true
		}
	}
	if !slices.IsSorted(names) {
		t.Error("expected FormatVariables to be sorted by name")
	}
	for name, found := range want {
		if !found {
			t.Errorf("expected format variable %q to be present", name)
		}
	}
}

func TestSharedDomainsTypedFields(t *testing.T) {
	c := MustDefault()
	d := c.SharedDomains()
	if d.TmuxCommand == nil {
		t.Fatal("expected typed TmuxCommand domain")
	}
	if d.TmuxCommand.Description == "" {
		t.Error("expected TmuxCommand description")
	}
	if d.FormatString == nil {
		t.Fatal("expected typed FormatString domain")
	}
	if d.FormatString.Description == "" {
		t.Error("expected FormatString description")
	}
	if len(d.FormatString.VariableNotes) == 0 {
		t.Error("expected FormatString.VariableNotes to be populated")
	}
}

func TestLookupNewOptions(t *testing.T) {
	c := MustDefault()
	// copy-mode-line-numbers is a new choice option in the updated catalog.
	opt, _ := c.Lookup("copy-mode-line-numbers")
	if opt == nil {
		t.Fatal("expected copy-mode-line-numbers to be present")
	}
	if opt.Type != TypeChoice {
		t.Errorf("expected copy-mode-line-numbers to be a choice, got %q", opt.Type)
	}
	if !slices.Contains(opt.Choices, "hybrid") {
		t.Errorf("expected 'hybrid' in copy-mode-line-numbers choices, got %v", opt.Choices)
	}
}
