package cmdparse

import "fmt"

// DataSource provides raw data for resolving completion candidates.
type DataSource interface {
	Sessions() []string
	Windows() []string
	Panes() []string
	Clients() []string
	Commands() []string
	Buffers() []string
}

// StoreResolver resolves argument types to completion candidates using live
// data from the UI layer plus a few hardcoded enumerations.
type StoreResolver struct {
	src DataSource
}

// NewStoreResolver creates a resolver backed by the given data source.
func NewStoreResolver(src DataSource) *StoreResolver {
	return &StoreResolver{src: src}
}

// Resolve returns completion candidates for the given argument type.
func (r *StoreResolver) Resolve(argType string) []string {
	if r == nil || r.src == nil {
		return nil
	}

	switch argType {
	case "target-session", "session-name":
		return r.src.Sessions()
	case "target-window", "window-name", "src-window", "dst-window":
		return r.src.Windows()
	case "target-pane", "src-pane", "dst-pane", "pane":
		return r.src.Panes()
	case "target-client":
		return r.src.Clients()
	case "command":
		return r.src.Commands()
	case "buffer-name", "new-buffer-name":
		return r.src.Buffers()
	case "key-table":
		return []string{"root", "prefix", "copy-mode", "copy-mode-vi"}
	case "layout-name":
		return []string{"even-horizontal", "even-vertical", "main-horizontal", "main-vertical", "tiled"}
	case "prompt-type":
		return []string{"command", "search", "target", "window-target"}
	default:
		return nil
	}
}

// FlagCandidate represents a flag available for completion.
type FlagCandidate struct {
	Flag    rune
	Label   string
	ArgType string
}

// FlagCandidates returns all flags from schema that are not already used,
// preferring argument-taking flags before boolean flags.
func FlagCandidates(schema *CommandSchema, used []rune) []FlagCandidate {
	if schema == nil {
		return nil
	}

	usedSet := make(map[rune]bool, len(used))
	for _, flag := range used {
		usedSet[flag] = true
	}

	candidates := make([]FlagCandidate, 0, len(schema.ArgFlags)+len(schema.BoolFlags))
	for _, argFlag := range schema.ArgFlags {
		if usedSet[argFlag.Short] {
			continue
		}
		candidates = append(candidates, FlagCandidate{
			Flag:    argFlag.Short,
			Label:   fmt.Sprintf("-%c %s", argFlag.Short, argFlag.ArgType),
			ArgType: argFlag.ArgType,
		})
	}
	for _, boolFlag := range schema.BoolFlags {
		if usedSet[boolFlag] {
			continue
		}
		candidates = append(candidates, FlagCandidate{
			Flag:  boolFlag,
			Label: fmt.Sprintf("-%c", boolFlag),
		})
	}

	return candidates
}
