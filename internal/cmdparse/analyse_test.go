package cmdparse

import (
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
	for _, v := range rs {
		if v == r {
			return true
		}
	}
	return false
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
