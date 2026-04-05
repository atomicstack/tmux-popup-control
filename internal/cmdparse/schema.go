package cmdparse

// CommandSchema is the parsed representation of a tmux command synopsis.
type CommandSchema struct {
	Name        string
	Alias       string
	BoolFlags   []rune
	ArgFlags    []ArgFlagDef
	Flags       []FlagDef
	Positionals []PositionalDef
}

// FlagDef describes a command flag in synopsis order.
type FlagDef struct {
	Short      rune
	ArgType    string
	Repeatable bool
}

// ArgFlagDef is a flag that expects a typed argument value.
type ArgFlagDef struct {
	Short   rune
	ArgType string
}

// PositionalDef is a positional argument.
type PositionalDef struct {
	Name     string
	Required bool
	Variadic bool
}

// CompletionContext describes what kind of completion is available at the
// current cursor position in the command input.
type CompletionContext struct {
	Kind      ContextKind
	ArgType   string // for FlagValue/PositionalValue: the argument type name
	TypeLabel string // display label for ghost hint (e.g. "src-window")
	Prefix    string // text already typed for the current token
	FlagsUsed []rune // flags already present in the input
}

// ContextKind identifies the type of completion point.
type ContextKind int

const (
	ContextNone            ContextKind = iota
	ContextCommandName                 // first token: completing a command name
	ContextFlagName                    // expecting a flag (e.g. after space following completed arg)
	ContextFlagValue                   // expecting a value for a flag (e.g. after "-t ")
	ContextPositionalValue             // expecting a positional argument value
)

func (s *CommandSchema) OrderedFlags() []FlagDef {
	if s == nil {
		return nil
	}
	if len(s.Flags) > 0 {
		return s.Flags
	}

	flags := make([]FlagDef, 0, len(s.BoolFlags)+len(s.ArgFlags))
	for _, short := range s.BoolFlags {
		flags = append(flags, FlagDef{
			Short:      short,
			Repeatable: isRepeatableFlag(s.Name, short),
		})
	}
	for _, argFlag := range s.ArgFlags {
		flags = append(flags, FlagDef{
			Short:      argFlag.Short,
			ArgType:    argFlag.ArgType,
			Repeatable: isRepeatableFlag(s.Name, argFlag.Short),
		})
	}
	return flags
}

func isRepeatableFlag(command string, flag rune) bool {
	flags, ok := repeatableFlagsByCommand[command]
	if !ok {
		return false
	}
	return flags[flag]
}

var repeatableFlagsByCommand = map[string]map[rune]bool{
	"display-popup":  {'e': true},
	"new-session":    {'e': true},
	"new-window":     {'e': true},
	"refresh-client": {'A': true},
	"respawn-pane":   {'e': true},
	"respawn-window": {'e': true},
	"split-window":   {'e': true},
}
