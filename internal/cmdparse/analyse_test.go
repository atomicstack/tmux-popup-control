package cmdparse

import (
	"slices"
	"testing"
)

func attachSchema() *CommandSchema {
	return &CommandSchema{
		Name:      "attach-session",
		Alias:     "attach",
		BoolFlags: []rune{'d', 'E', 'r', 'x'},
		ArgFlags: []ArgFlagDef{
			{Short: 'c', ArgType: "working-directory"},
			{Short: 'f', ArgType: "flags"},
			{Short: 't', ArgType: "target-session"},
		},
	}
}

func bindKeySchema() *CommandSchema {
	return &CommandSchema{
		Name:      "bind-key",
		Alias:     "bind",
		BoolFlags: []rune{'n', 'r'},
		ArgFlags: []ArgFlagDef{
			{Short: 'T', ArgType: "key-table"},
			{Short: 'N', ArgType: "note"},
		},
		Positionals: []PositionalDef{
			{Name: "key", Required: true},
			{Name: "command", Required: false},
			{Name: "argument", Required: false, Variadic: true},
		},
	}
}

func buildTestRegistry(schemas ...*CommandSchema) map[string]*CommandSchema {
	reg := make(map[string]*CommandSchema)
	for _, s := range schemas {
		reg[s.Name] = s
		if s.Alias != "" {
			reg[s.Alias] = s
		}
	}
	return reg
}

func containsRune(rs []rune, r rune) bool {
	return slices.Contains(rs, r)
}

func TestAnalyseEmptyInput(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "")
	if ctx.Kind != ContextCommandName {
		t.Errorf("expected ContextCommandName, got %d", ctx.Kind)
	}
}

func TestAnalysePartialCommandName(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "att")
	if ctx.Kind != ContextCommandName {
		t.Errorf("expected ContextCommandName, got %d", ctx.Kind)
	}
	if ctx.Prefix != "att" {
		t.Errorf("expected prefix %q, got %q", "att", ctx.Prefix)
	}
}

func TestAnalyseAfterCommandNameSpace(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "attach-session ")
	if ctx.Kind != ContextFlagName {
		t.Errorf("expected ContextFlagName, got %d", ctx.Kind)
	}
}

func TestAnalysePartialFlag(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "attach-session -")
	if ctx.Kind != ContextFlagName {
		t.Errorf("expected ContextFlagName, got %d", ctx.Kind)
	}
	if ctx.Prefix != "-" {
		t.Errorf("expected prefix %q, got %q", "-", ctx.Prefix)
	}
}

func TestAnalyseAfterBoolFlag(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "attach-session -d ")
	if ctx.Kind != ContextFlagName {
		t.Errorf("expected ContextFlagName, got %d", ctx.Kind)
	}
	if !containsRune(ctx.FlagsUsed, 'd') {
		t.Errorf("expected 'd' in FlagsUsed, got %v", ctx.FlagsUsed)
	}
}

func TestAnalyseAfterArgFlagSpace(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "attach-session -t ")
	if ctx.Kind != ContextFlagValue {
		t.Errorf("expected ContextFlagValue, got %d", ctx.Kind)
	}
	if ctx.ArgType != "target-session" {
		t.Errorf("expected ArgType %q, got %q", "target-session", ctx.ArgType)
	}
}

func TestAnalyseMidFlagValue(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "attach-session -t mys")
	if ctx.Kind != ContextFlagValue {
		t.Errorf("expected ContextFlagValue, got %d", ctx.Kind)
	}
	if ctx.Prefix != "mys" {
		t.Errorf("expected prefix %q, got %q", "mys", ctx.Prefix)
	}
}

func TestAnalyseAfterFlagValueCompleted(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "attach-session -t mysess ")
	if ctx.Kind != ContextFlagName {
		t.Errorf("expected ContextFlagName, got %d", ctx.Kind)
	}
	if !containsRune(ctx.FlagsUsed, 't') {
		t.Errorf("expected 't' in FlagsUsed, got %v", ctx.FlagsUsed)
	}
}

func TestAnalysePositionalArg(t *testing.T) {
	reg := buildTestRegistry(bindKeySchema())
	ctx := Analyse(reg, "bind-key -n ")
	if ctx.Kind != ContextFlagName {
		t.Errorf("expected ContextFlagName, got %d", ctx.Kind)
	}
}

func TestAnalysePositionalAfterAllFlags(t *testing.T) {
	reg := buildTestRegistry(bindKeySchema())
	ctx := Analyse(reg, "bind-key -n -T root C-a ")
	if ctx.Kind != ContextPositionalValue {
		t.Errorf("expected ContextPositionalValue, got %d", ctx.Kind)
	}
	if ctx.ArgType != "command" {
		t.Errorf("expected ArgType %q, got %q", "command", ctx.ArgType)
	}
}

func TestAnalyseUnknownCommand(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "nonexistent-cmd ")
	if ctx.Kind != ContextNone {
		t.Errorf("expected ContextNone, got %d", ctx.Kind)
	}
}

func TestAnalyseAlias(t *testing.T) {
	reg := buildTestRegistry(attachSchema())
	ctx := Analyse(reg, "attach -t ")
	if ctx.Kind != ContextFlagValue {
		t.Errorf("expected ContextFlagValue, got %d", ctx.Kind)
	}
	if ctx.ArgType != "target-session" {
		t.Errorf("expected ArgType %q, got %q", "target-session", ctx.ArgType)
	}
}

func TestAnalyseAfterUsingAllFlagsStillSuggestsRepeatableFlag(t *testing.T) {
	schema, err := ParseSynopsis("new-window (neww) [-abdkPS] [-c start-directory] [-e environment] [-F format] [-n window-name] [-t target-window] [shell-command [argument ...]]")
	if err != nil {
		t.Fatalf("ParseSynopsis failed: %v", err)
	}

	reg := buildTestRegistry(schema)
	ctx := Analyse(reg, "new-window -a -b -d -k -P -S -c dir -e FOO=bar -F fmt -n name -t work:1 ")
	if ctx.Kind != ContextFlagName {
		t.Fatalf("expected ContextFlagName, got %d", ctx.Kind)
	}
}

func resizePaneSchema() *CommandSchema {
	return &CommandSchema{
		Name:      "resize-pane",
		Alias:     "resizep",
		BoolFlags: []rune{'M', 'T', 'Z'},
		ArgFlags: []ArgFlagDef{
			{Short: 'D', ArgType: "lines"},
			{Short: 't', ArgType: "target-pane"},
		},
		Flags: []FlagDef{
			{Short: 'M'},
			{Short: 'T'},
			{Short: 'Z'},
			{Short: 'D', ArgType: "lines", OptionalValue: true},
			{Short: 't', ArgType: "target-pane"},
		},
	}
}

// An optional-value flag (getopt "D::") is complete on its own: with nothing
// after it, the next completion point is another flag, not a mandatory value.
func TestAnalyseOptionalValueFlagDoesNotDemandValue(t *testing.T) {
	reg := buildTestRegistry(resizePaneSchema())
	ctx := Analyse(reg, "resize-pane -D ")
	if ctx.Kind != ContextFlagName {
		t.Fatalf("expected ContextFlagName, got %d", ctx.Kind)
	}
	if !containsRune(ctx.FlagsUsed, 'D') {
		t.Errorf("expected 'D' in FlagsUsed, got %v", ctx.FlagsUsed)
	}
}

// A following token that does not look like a flag is the optional value.
func TestAnalyseOptionalValueFlagConsumesValueToken(t *testing.T) {
	reg := buildTestRegistry(resizePaneSchema())
	ctx := Analyse(reg, "resize-pane -D 5")
	if ctx.Kind != ContextFlagValue {
		t.Fatalf("expected ContextFlagValue, got %d", ctx.Kind)
	}
	if ctx.ArgType != "lines" {
		t.Errorf("expected ArgType %q, got %q", "lines", ctx.ArgType)
	}
	if ctx.Prefix != "5" {
		t.Errorf("expected prefix %q, got %q", "5", ctx.Prefix)
	}
}

// A following flag token ends the optional flag without a value, matching
// tmux's args parser, so "-t" is then analysed as its own flag.
func TestAnalyseOptionalValueFlagFollowedByFlag(t *testing.T) {
	reg := buildTestRegistry(resizePaneSchema())
	ctx := Analyse(reg, "resize-pane -D -t ")
	if ctx.Kind != ContextFlagValue {
		t.Fatalf("expected ContextFlagValue, got %d", ctx.Kind)
	}
	if ctx.ArgType != "target-pane" {
		t.Errorf("expected ArgType %q, got %q", "target-pane", ctx.ArgType)
	}
	if !containsRune(ctx.FlagsUsed, 'D') || !containsRune(ctx.FlagsUsed, 't') {
		t.Errorf("expected D and t in FlagsUsed, got %v", ctx.FlagsUsed)
	}
}

func TestAnalyseRequiredValueFlagStillDemandsValue(t *testing.T) {
	reg := buildTestRegistry(resizePaneSchema())
	ctx := Analyse(reg, "resize-pane -t ")
	if ctx.Kind != ContextFlagValue {
		t.Fatalf("expected ContextFlagValue, got %d", ctx.Kind)
	}
}

func TestAnalyseIncompleteFlagTokens(t *testing.T) {
	schema := &CommandSchema{Name: "new-session", BoolFlags: []rune{'d'}, ArgFlags: []ArgFlagDef{{Short: 's', ArgType: "session-name"}}, Positionals: []PositionalDef{{Name: "shell-command"}, {Name: "argument", Variadic: true}}}
	reg := buildTestRegistry(schema)
	for _, tt := range []struct {
		input       string
		kind        ContextKind
		arg, prefix string
	}{
		{"new-session - ", ContextPositionalValue, "argument", ""},
		{"new-session -- ", ContextPositionalValue, "shell-command", ""},
		{"new-session -- -s", ContextPositionalValue, "shell-command", "-s"},
		{"new-session -- -s ", ContextPositionalValue, "argument", ""},
		{"new-session -s - ", ContextFlagName, "", ""},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got := Analyse(reg, tt.input)
			if got.Kind != tt.kind || got.ArgType != tt.arg || got.Prefix != tt.prefix {
				t.Fatalf("got %+v, want kind %v, arg %q, prefix %q", got, tt.kind, tt.arg, tt.prefix)
			}
		})
	}
}

func FuzzAnalyseIncompleteInput(f *testing.F) {
	reg := buildTestRegistry(attachSchema(), bindKeySchema(), resizePaneSchema(), &CommandSchema{Name: "new-session", BoolFlags: []rune{'d'}})
	for _, seed := range []string{"new-session - ", "new-session -- ", "attach-session -t ", "attach-session -t - ", "bind-key -- -T ", "resize-pane -D -", "bind-key '", "bind-key \\", "attach-session \"-"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) { Analyse(reg, input) })
}

func TestAnalyseFlagTerminatorRequiresSeparator(t *testing.T) {
	schema := &CommandSchema{Name: "set-option", BoolFlags: []rune{'g'}, Positionals: []PositionalDef{{Name: "option-name"}, {Name: "value"}}}
	reg := buildTestRegistry(schema)
	for _, tt := range []struct {
		input string
		kind  ContextKind
		arg   string
	}{
		{"set-option --", ContextNone, ""},
		{"set-option -g --", ContextNone, ""},
		{"set-option -- ", ContextPositionalValue, "option-name"},
		{"set-option -g -- ", ContextPositionalValue, "option-name"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got := Analyse(reg, tt.input)
			if got.Kind != tt.kind || got.ArgType != tt.arg {
				t.Fatalf("got %+v, want kind %v and arg %q", got, tt.kind, tt.arg)
			}
		})
	}
}
