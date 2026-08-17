// Package cmdhelp exposes per-command help for tmux commands: a one-line
// summary plus a description for each flag.
//
// The data is derived from the option catalog embedded in
// internal/tmuxopts, which since catalog schema v2 carries command usage,
// descriptions, and per-flag metadata. Refreshing the catalog JSON is
// therefore the only step needed to update command help — there is no
// generated Go file to regenerate.
package cmdhelp

import (
	"sync"

	"github.com/atomicstack/tmux-popup-control/internal/tmuxopts"
)

// CommandHelp is the help text for one tmux command.
type CommandHelp struct {
	Summary string
	Args    []ArgHelp
}

// ArgHelp describes a single command flag. Name includes the leading dash
// (e.g. "-t") so callers can look it up by the token the user typed.
type ArgHelp struct {
	Name        string
	Description string
}

var commands = sync.OnceValue(buildCommands)

// Commands returns help for every tmux command in the catalog, keyed by
// canonical command name. The map is built once; callers must not mutate
// it. It is empty if the embedded catalog fails to load.
func Commands() map[string]CommandHelp {
	return commands()
}

func buildCommands() map[string]CommandHelp {
	catalog, err := tmuxopts.Default()
	if err != nil {
		return map[string]CommandHelp{}
	}
	entries := catalog.TmuxCommands()
	out := make(map[string]CommandHelp, len(entries))
	for _, entry := range entries {
		help := CommandHelp{Summary: entry.Description}
		if len(entry.Flags) > 0 {
			help.Args = make([]ArgHelp, 0, len(entry.Flags))
			for _, flag := range entry.Flags {
				help.Args = append(help.Args, ArgHelp{
					Name:        flag.Name,
					Description: flag.Description,
				})
			}
		}
		out[entry.Name] = help
	}
	return out
}
