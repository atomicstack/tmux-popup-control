package cmdparse

import (
	"testing"

	"github.com/atomicstack/tmux-popup-control/internal/tmuxopts"
)

func defaultCatalog(t *testing.T) *tmuxopts.Catalog {
	t.Helper()
	catalog, err := tmuxopts.Default()
	if err != nil {
		t.Fatalf("tmuxopts.Default failed: %v", err)
	}
	return catalog
}

func flagByShort(schema *CommandSchema, short rune) *FlagDef {
	for _, flag := range schema.OrderedFlags() {
		if flag.Short == short {
			return &flag
		}
	}
	return nil
}

func flagRunes(schema *CommandSchema) []rune {
	flags := schema.OrderedFlags()
	out := make([]rune, 0, len(flags))
	for _, flag := range flags {
		out = append(out, flag.Short)
	}
	return out
}

func positionalNames(schema *CommandSchema) []string {
	out := make([]string, 0, len(schema.Positionals))
	for _, pos := range schema.Positionals {
		out = append(out, pos.Name)
	}
	return out
}

// The synopsis scraper used to drop every command whose bool cluster
// contained a digit ([-1aNr], [-2]) into the positional parser, leaving
// them with zero flags. The catalog carries the flags structurally.
func TestCatalogRegistryBoolFlagsWithDigits(t *testing.T) {
	reg := BuildRegistryFromCatalog(defaultCatalog(t), nil)

	cases := []struct {
		name      string
		wantBool  []rune
		wantArgs  []rune
		wantPos   []string
		wantAlias string
	}{
		{
			name:      "list-keys",
			wantBool:  []rune{'1', 'a', 'N', 'r'},
			wantArgs:  []rune{'F', 'O', 'P', 'T'},
			wantPos:   []string{"key"},
			wantAlias: "lsk",
		},
		{
			name:     "command-prompt",
			wantBool: []rune{'1', 'C', 'b', 'e', 'F', 'i', 'k', 'l', 'N', 'P'},
			wantArgs: []rune{'I', 'p', 't', 'T'},
			wantPos:  []string{"template"},
		},
		{
			name:     "send-prefix",
			wantBool: []rune{'2'},
			wantArgs: []rune{'t'},
		},
	}
	for _, tc := range cases {
		schema, ok := reg[tc.name]
		if !ok {
			t.Errorf("registry missing %q", tc.name)
			continue
		}
		if schema.Alias != tc.wantAlias {
			t.Errorf("%s: alias = %q, want %q", tc.name, schema.Alias, tc.wantAlias)
		}
		if tc.wantAlias != "" && reg[tc.wantAlias] != schema {
			t.Errorf("%s: alias %q does not resolve to the same schema", tc.name, tc.wantAlias)
		}
		if !runesEqual(schema.BoolFlags, tc.wantBool) {
			t.Errorf("%s: bool flags = %q, want %q", tc.name, string(schema.BoolFlags), string(tc.wantBool))
		}
		gotArgs := make([]rune, 0, len(schema.ArgFlags))
		for _, af := range schema.ArgFlags {
			gotArgs = append(gotArgs, af.Short)
		}
		if !runesEqual(gotArgs, tc.wantArgs) {
			t.Errorf("%s: arg flags = %q, want %q", tc.name, string(gotArgs), string(tc.wantArgs))
		}
		gotPos := positionalNames(schema)
		if len(gotPos) != len(tc.wantPos) {
			t.Errorf("%s: positionals = %v, want %v", tc.name, gotPos, tc.wantPos)
			continue
		}
		for i := range tc.wantPos {
			if gotPos[i] != tc.wantPos[i] {
				t.Errorf("%s: positional[%d] = %q, want %q", tc.name, i, gotPos[i], tc.wantPos[i])
			}
		}
	}
}

func TestCatalogRegistryFlagOrderFollowsUsage(t *testing.T) {
	reg := BuildRegistryFromCatalog(defaultCatalog(t), nil)
	schema := reg["attach-session"]
	if schema == nil {
		t.Fatal("registry missing attach-session")
	}
	want := []rune{'d', 'E', 'r', 'x', 'c', 'f', 't'}
	if got := flagRunes(schema); !runesEqual(got, want) {
		t.Fatalf("flag order = %q, want %q", string(got), string(want))
	}
	if flag := flagByShort(schema, 't'); flag == nil || flag.ArgType != "target-session" {
		t.Fatalf("-t = %+v, want arg type target-session", flag)
	}
}

func TestCatalogRegistryPositionals(t *testing.T) {
	reg := BuildRegistryFromCatalog(defaultCatalog(t), nil)
	schema := reg["bind-key"]
	if schema == nil {
		t.Fatal("registry missing bind-key")
	}
	want := []PositionalDef{
		{Name: "key", Required: true},
		{Name: "command", Required: false},
		{Name: "argument", Required: false, Variadic: true},
	}
	if len(schema.Positionals) != len(want) {
		t.Fatalf("positionals = %+v, want %+v", schema.Positionals, want)
	}
	for i := range want {
		if schema.Positionals[i] != want[i] {
			t.Errorf("positional[%d] = %+v, want %+v", i, schema.Positionals[i], want[i])
		}
	}
}

func TestCatalogRegistryOptionalValueFlags(t *testing.T) {
	reg := BuildRegistryFromCatalog(defaultCatalog(t), nil)
	schema := reg["resize-pane"]
	if schema == nil {
		t.Fatal("registry missing resize-pane")
	}
	for _, short := range []rune{'D', 'L', 'R', 'U'} {
		flag := flagByShort(schema, short)
		if flag == nil {
			t.Errorf("-%c missing", short)
			continue
		}
		if !flag.OptionalValue {
			t.Errorf("-%c: OptionalValue = false, want true", short)
		}
		if flag.ArgType == "" {
			t.Errorf("-%c: ArgType empty, want value name from catalog", short)
		}
		if !SchemaHasArgFlag(schema, short) {
			t.Errorf("-%c: expected to be listed as an arg flag", short)
		}
		if !SchemaFlagValueOptional(schema, short) {
			t.Errorf("-%c: SchemaFlagValueOptional = false, want true", short)
		}
	}
	if flag := flagByShort(schema, 't'); flag == nil || flag.OptionalValue {
		t.Errorf("-t = %+v, want required-value flag", flag)
	}
	if SchemaFlagValueOptional(schema, 't') {
		t.Error("-t: SchemaFlagValueOptional = true, want false")
	}
}

func TestCatalogRegistryOptionalValueFlagCandidateLabel(t *testing.T) {
	reg := BuildRegistryFromCatalog(defaultCatalog(t), nil)
	schema := reg["resize-pane"]
	if schema == nil {
		t.Fatal("registry missing resize-pane")
	}
	for _, candidate := range FlagCandidates(schema, nil) {
		switch candidate.Flag {
		case 'D':
			if !candidate.OptionalValue {
				t.Error("-D candidate: OptionalValue = false, want true")
			}
			if candidate.Label != "-D [lines]" {
				t.Errorf("-D label = %q, want %q", candidate.Label, "-D [lines]")
			}
		case 't':
			if candidate.OptionalValue {
				t.Error("-t candidate: OptionalValue = true, want false")
			}
			if candidate.Label != "-t target-pane" {
				t.Errorf("-t label = %q, want %q", candidate.Label, "-t target-pane")
			}
		}
	}
}

func TestCatalogRegistryRepeatableFlags(t *testing.T) {
	reg := BuildRegistryFromCatalog(defaultCatalog(t), nil)
	cases := []struct {
		command string
		flag    rune
	}{
		{"new-pane", 'e'},
		{"refresh-client", 'B'},
		{"new-window", 'e'},
		{"refresh-client", 'A'},
	}
	for _, tc := range cases {
		schema := reg[tc.command]
		if schema == nil {
			t.Errorf("registry missing %q", tc.command)
			continue
		}
		flag := flagByShort(schema, tc.flag)
		if flag == nil {
			t.Errorf("%s: -%c missing", tc.command, tc.flag)
			continue
		}
		if !flag.Repeatable {
			t.Errorf("%s: -%c Repeatable = false, want true", tc.command, tc.flag)
		}
	}
}

func TestCatalogRegistryFallsBackToSynopsisForUnknownCommand(t *testing.T) {
	lines := []string{
		// present in the catalog: the catalog schema must win even though
		// the live synopsis line is missing flags the catalog knows about.
		"kill-session [-a] [-t target-session]",
		// absent from the catalog: only the synopsis can describe it.
		"frobnicate-thing (frob) [-x] [-t target-pane] name",
	}
	reg := BuildRegistryFromCatalog(defaultCatalog(t), lines)

	kill := reg["kill-session"]
	if kill == nil {
		t.Fatal("registry missing kill-session")
	}
	if flagByShort(kill, 'C') == nil {
		t.Errorf("kill-session: expected catalog flag -C, got flags %q", string(flagRunes(kill)))
	}

	frob := reg["frobnicate-thing"]
	if frob == nil {
		t.Fatal("registry missing synopsis-only command frobnicate-thing")
	}
	if reg["frob"] != frob {
		t.Error("alias frob does not resolve to the synopsis-only schema")
	}
	if got := flagRunes(frob); !runesEqual(got, []rune{'x', 't'}) {
		t.Errorf("frobnicate-thing flags = %q, want %q", string(got), "xt")
	}
	if got := positionalNames(frob); len(got) != 1 || got[0] != "name" {
		t.Errorf("frobnicate-thing positionals = %v, want [name]", got)
	}

	// live lines restrict the registry to what the running tmux accepts.
	if _, ok := reg["new-pane"]; ok {
		t.Error("registry should not include catalog commands absent from the live command list")
	}
}

func TestCatalogRegistryNilCatalogUsesSynopsisOnly(t *testing.T) {
	lines := []string{"kill-session [-a] [-t target-session]"}
	reg := BuildRegistryFromCatalog(nil, lines)
	schema := reg["kill-session"]
	if schema == nil {
		t.Fatal("registry missing kill-session")
	}
	if got := flagRunes(schema); !runesEqual(got, []rune{'a', 't'}) {
		t.Errorf("flags = %q, want %q", string(got), "at")
	}
}

func TestBuildCatalogRegistryUsesEmbeddedCatalog(t *testing.T) {
	reg := BuildCatalogRegistry([]string{"send-prefix [-2] [-t target-pane]"})
	schema := reg["send-prefix"]
	if schema == nil {
		t.Fatal("registry missing send-prefix")
	}
	if got := flagRunes(schema); !runesEqual(got, []rune{'2', 't'}) {
		t.Errorf("flags = %q, want %q", string(got), "2t")
	}
}
