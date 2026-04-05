package cmdparse

import "strings"

// Analyse inspects the user's partial command input and returns a
// CompletionContext describing what kind of value is expected at the cursor
// position (assumed to be at the end of input).
func Analyse(registry map[string]*CommandSchema, input string) CompletionContext {
	if input == "" {
		return CompletionContext{Kind: ContextCommandName}
	}

	trailingSpace := strings.HasSuffix(input, " ")
	tokens := strings.Fields(input)

	if len(tokens) == 0 {
		return CompletionContext{Kind: ContextCommandName}
	}

	// single token, no trailing space — user is still typing the command name
	if len(tokens) == 1 && !trailingSpace {
		return CompletionContext{Kind: ContextCommandName, Prefix: tokens[0]}
	}

	// look up the command in the registry
	cmdName := tokens[0]
	schema, ok := registry[cmdName]
	if !ok {
		return CompletionContext{Kind: ContextNone}
	}

	// walk tokens after the command name
	var flagsUsed []rune
	posIndex := 0          // how many positional args consumed
	inPositionals := false // true once we've seen a non-flag token
	i := 1
	for i < len(tokens) {
		tok := tokens[i]

		// is this the last token and there's no trailing space?
		// the user is mid-typing this token
		isLast := i == len(tokens)-1 && !trailingSpace

		if strings.HasPrefix(tok, "-") {
			// bare "-" or "-X" — treat as flag territory
			if len(tok) < 2 || isLast {
				if isLast {
					// user is mid-typing a flag
					return CompletionContext{
						Kind:      ContextFlagName,
						Prefix:    tok,
						FlagsUsed: flagsUsed,
					}
				}
			}

			flag := rune(tok[1])

			if schemaHasArgFlag(schema, flag) {
				// arg flag — consumes next token as its value
				flagsUsed = append(flagsUsed, flag)
				i++ // skip the flag token
				if i < len(tokens) {
					isValueLast := i == len(tokens)-1 && !trailingSpace
					if isValueLast {
						// user is mid-typing the flag value
						return CompletionContext{
							Kind:      ContextFlagValue,
							ArgType:   schemaArgFlagType(schema, flag),
							Prefix:    tokens[i],
							FlagsUsed: flagsUsed,
						}
					}
					// value fully typed, move on
					i++
				} else {
					// flag at end with trailing space — need value
					return CompletionContext{
						Kind:      ContextFlagValue,
						ArgType:   schemaArgFlagType(schema, flag),
						FlagsUsed: flagsUsed,
					}
				}
				continue
			}

			// bool flag
			flagsUsed = append(flagsUsed, flag)
			i++
			continue
		}

		// not a flag — positional argument
		inPositionals = true
		if isLast {
			// mid-typing a positional
			pos := positionalAt(schema, posIndex)
			if pos != nil {
				return CompletionContext{
					Kind:      ContextPositionalValue,
					ArgType:   pos.Name,
					Prefix:    tok,
					FlagsUsed: flagsUsed,
				}
			}
			return CompletionContext{Kind: ContextNone, FlagsUsed: flagsUsed}
		}

		// fully typed positional
		pos := positionalAt(schema, posIndex)
		if pos != nil && !pos.Variadic {
			posIndex++
		}
		i++
	}

	// all tokens consumed and there's a trailing space — what comes next?

	// check if the previous token was an arg flag awaiting its value
	if len(tokens) >= 2 {
		prevTok := tokens[len(tokens)-1]
		if strings.HasPrefix(prevTok, "-") && len(prevTok) >= 2 {
			flag := rune(prevTok[1])
			if schemaHasArgFlag(schema, flag) {
				return CompletionContext{
					Kind:      ContextFlagValue,
					ArgType:   schemaArgFlagType(schema, flag),
					FlagsUsed: flagsUsed,
				}
			}
		}
	}

	// if we haven't entered positional territory and unused flags remain,
	// suggest a flag; once positionals have started, flags are no longer valid
	if !inPositionals && hasUnusedFlags(schema, flagsUsed) {
		return CompletionContext{
			Kind:      ContextFlagName,
			FlagsUsed: flagsUsed,
		}
	}

	// suggest next positional
	pos := positionalAt(schema, posIndex)
	if pos != nil {
		return CompletionContext{
			Kind:      ContextPositionalValue,
			ArgType:   pos.Name,
			FlagsUsed: flagsUsed,
		}
	}

	return CompletionContext{Kind: ContextNone, FlagsUsed: flagsUsed}
}

// schemaHasArgFlag reports whether the schema defines flag as an arg-taking flag.
func schemaHasArgFlag(schema *CommandSchema, flag rune) bool {
	for _, af := range schema.ArgFlags {
		if af.Short == flag {
			return true
		}
	}
	return false
}

// schemaArgFlagType returns the argument type string for the given arg flag.
func schemaArgFlagType(schema *CommandSchema, flag rune) string {
	for _, af := range schema.ArgFlags {
		if af.Short == flag {
			return af.ArgType
		}
	}
	return ""
}

// hasUnusedFlags reports whether the schema has any flags (bool or arg) not yet
// present in the used set.
func hasUnusedFlags(schema *CommandSchema, used []rune) bool {
	usedSet := make(map[rune]bool, len(used))
	for _, r := range used {
		usedSet[r] = true
	}

	for _, flag := range schema.OrderedFlags() {
		if flag.Repeatable || !usedSet[flag.Short] {
			return true
		}
	}
	return false
}

// positionalAt returns the positional definition at the given index, or nil if
// out of range. For variadic positionals, the last one is returned for any
// index beyond the list.
func positionalAt(schema *CommandSchema, index int) *PositionalDef {
	if len(schema.Positionals) == 0 {
		return nil
	}
	if index < len(schema.Positionals) {
		return &schema.Positionals[index]
	}
	last := &schema.Positionals[len(schema.Positionals)-1]
	if last.Variadic {
		return last
	}
	return nil
}
