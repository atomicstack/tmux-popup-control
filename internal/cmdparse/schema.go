package cmdparse

// CommandSchema is the parsed representation of a tmux command synopsis.
type CommandSchema struct {
	Name        string
	Alias       string
	BoolFlags   []rune
	ArgFlags    []ArgFlagDef
	Positionals []PositionalDef
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
