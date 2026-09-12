package cmdparse

import (
	"strings"

	"github.com/atomicstack/tmux-popup-control/internal/tmuxopts"
)

// BuildCatalogRegistry builds the completion registry for the given
// `tmux list-commands` synopsis lines, taking each command's schema from the
// embedded option catalog when the catalog knows the command and falling
// back to parsing the synopsis otherwise. If the embedded catalog cannot be
// loaded the result is the same as BuildRegistry(lines).
func BuildCatalogRegistry(lines []string) map[string]*CommandSchema {
	catalog, err := tmuxopts.Default()
	if err != nil {
		catalog = nil
	}
	return BuildRegistryFromCatalog(catalog, lines)
}

// BuildRegistryFromCatalog merges catalog-derived schemas with synopsis
// parsing. When lines is non-empty it names the commands the running tmux
// accepts: each one gets the catalog schema if present, else the parsed
// synopsis; catalog commands absent from lines are left out. When lines is
// empty every catalog command is included. A nil catalog degrades to
// synopsis parsing only.
func BuildRegistryFromCatalog(catalog *tmuxopts.Catalog, lines []string) map[string]*CommandSchema {
	fromCatalog := make(map[string]*CommandSchema)
	if catalog != nil {
		for _, entry := range catalog.TmuxCommands() {
			if schema := schemaFromCatalogEntry(entry); schema != nil {
				fromCatalog[schema.Name] = schema
			}
		}
	}

	reg := make(map[string]*CommandSchema)
	register := func(schema *CommandSchema) {
		reg[schema.Name] = schema
		if schema.Alias != "" {
			reg[schema.Alias] = schema
		}
	}

	if len(lines) == 0 {
		for _, schema := range fromCatalog {
			register(schema)
		}
		return reg
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parsed, err := ParseSynopsis(line)
		if err != nil {
			continue
		}
		if schema, ok := fromCatalog[parsed.Name]; ok {
			register(schema)
			continue
		}
		register(parsed)
	}
	return reg
}

// schemaFromCatalogEntry converts one catalog command into a CommandSchema.
// Flag membership, value requirement and value names come from the
// structured flags[] data. The catalog does not name positional arguments
// (positional_arguments only carries min/max counts) and records flags in
// getopt-template order rather than display order, so both positionals and
// flag ordering are taken from the entry's usage line, which has the same
// shape as a list-commands synopsis.
func schemaFromCatalogEntry(entry tmuxopts.TmuxCommandEntry) *CommandSchema {
	if entry.Name == "" {
		return nil
	}
	schema := &CommandSchema{Name: entry.Name, Alias: entry.Alias}

	byShort := make(map[rune]tmuxopts.TmuxCommandFlag, len(entry.Flags))
	order := make([]rune, 0, len(entry.Flags))
	for _, flag := range entry.Flags {
		short, ok := flagShort(flag.Name)
		if !ok {
			continue
		}
		if _, dup := byShort[short]; dup {
			continue
		}
		byShort[short] = flag
		order = append(order, short)
	}

	if usage, err := ParseSynopsis(entry.Usage); err == nil && usage.Name == entry.Name {
		schema.Positionals = usage.Positionals
		if entry.PositionalArguments != nil && entry.PositionalArguments.Maximum == 0 {
			schema.Positionals = nil
		}
		usageOrder := make([]rune, 0, len(order))
		seen := make(map[rune]bool, len(order))
		for _, flag := range usage.OrderedFlags() {
			if _, ok := byShort[flag.Short]; ok && !seen[flag.Short] {
				usageOrder = append(usageOrder, flag.Short)
				seen[flag.Short] = true
			}
		}
		for _, short := range order {
			if !seen[short] {
				usageOrder = append(usageOrder, short)
			}
		}
		order = usageOrder
	}

	for _, short := range order {
		flag := byShort[short]
		def := FlagDef{
			Short:      short,
			Repeatable: isRepeatableFlag(entry.Name, short),
		}
		switch flag.ValueMode {
		case tmuxopts.ValueModeRequired, tmuxopts.ValueModeOptional:
			def.ArgType = flag.ValueName
			if def.ArgType == "" {
				def.ArgType = "value"
			}
			def.OptionalValue = flag.ValueMode == tmuxopts.ValueModeOptional
			schema.ArgFlags = append(schema.ArgFlags, ArgFlagDef{Short: short, ArgType: def.ArgType})
		default:
			schema.BoolFlags = append(schema.BoolFlags, short)
		}
		schema.Flags = append(schema.Flags, def)
	}
	return schema
}

// flagShort extracts the single flag character from a catalog flag name
// such as "-F".
func flagShort(name string) (rune, bool) {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) != 2 || runes[0] != '-' || !isFlagChar(runes[1]) {
		return 0, false
	}
	return runes[1], true
}
