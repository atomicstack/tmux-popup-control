package cmdparse

import (
	"fmt"
	"strings"
)

// ParseSynopsis parses a single line of tmux list-commands output into a
// CommandSchema. The expected format is:
//
//	command-name [(alias)] [-boolflags] [-f arg-type] ... [positional ...]
func ParseSynopsis(line string) (*CommandSchema, error) {
	tokens := tokenize(line)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty synopsis")
	}

	s := &CommandSchema{}
	i := 0

	// first token is the command name
	s.Name = tokens[i]
	i++

	// optional alias in parens: (alias)
	if i < len(tokens) && strings.HasPrefix(tokens[i], "(") && strings.HasSuffix(tokens[i], ")") {
		s.Alias = tokens[i][1 : len(tokens[i])-1]
		i++
	}

	// parse flags and positionals from remaining tokens
	for i < len(tokens) {
		tok := tokens[i]

		// clustered bool flags: [-abc] or [-aBcD]
		if isBoolCluster(tok) {
			inner := stripBrackets(tok)
			// remove leading dash
			for _, r := range inner[1:] {
				s.BoolFlags = append(s.BoolFlags, r)
				s.Flags = append(s.Flags, FlagDef{
					Short:      r,
					Repeatable: isRepeatableFlag(s.Name, r),
				})
			}
			i++
			continue
		}

		// arg flag: [-f arg-type] — appears as two consecutive tokens
		// first is [-f and second is arg-type]
		// or first is -f inside a bracket group parsed as [-f, arg-type]
		if isArgFlag(tok, tokens, i) {
			short, argType, consumed := parseArgFlag(tok, tokens, i)
			s.ArgFlags = append(s.ArgFlags, ArgFlagDef{Short: short, ArgType: argType})
			s.Flags = append(s.Flags, FlagDef{
				Short:      short,
				ArgType:    argType,
				Repeatable: isRepeatableFlag(s.Name, short),
			})
			i += consumed
			continue
		}

		// everything else is a positional argument
		positionals := parsePositionals(tokens[i:])
		s.Positionals = append(s.Positionals, positionals...)
		break
	}

	return s, nil
}

// BuildRegistry parses all synopsis lines and returns a map from command name
// (and alias) to the corresponding schema.
func BuildRegistry(lines []string) map[string]*CommandSchema {
	reg := make(map[string]*CommandSchema)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		s, err := ParseSynopsis(line)
		if err != nil {
			continue
		}
		reg[s.Name] = s
		if s.Alias != "" {
			reg[s.Alias] = s
		}
	}
	return reg
}

// tokenize splits a synopsis line into tokens, treating bracket groups as
// individual tokens when they contain flag definitions (e.g. [-abc], [-f val]).
// Nested brackets like [command [argument ...]] are flattened.
func tokenize(line string) []string {
	var tokens []string
	runes := []rune(line)
	i := 0

	for i < len(runes) {
		// skip whitespace
		if runes[i] == ' ' || runes[i] == '\t' {
			i++
			continue
		}

		if runes[i] == '[' {
			// find matching close bracket, handling nesting
			depth := 0
			start := i
			for i < len(runes) {
				if runes[i] == '[' {
					depth++
				} else if runes[i] == ']' {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				i++
			}
			group := string(runes[start:i])
			// if it's a flag group like [-abc] or [-f val], keep as one token
			inner := stripBrackets(group)
			if strings.HasPrefix(inner, "-") {
				tokens = append(tokens, group)
			} else {
				// positional bracket group — flatten nested brackets
				// e.g. [command [argument ...]] -> [command] [argument ...]
				flattenBracketGroup(inner, &tokens)
			}
			continue
		}

		// regular word token
		start := i
		for i < len(runes) && runes[i] != ' ' && runes[i] != '\t' {
			i++
		}
		tokens = append(tokens, string(runes[start:i]))
	}

	return tokens
}

// flattenBracketGroup takes the inner content of a bracket group (without
// outer brackets) and produces individual optional positional tokens.
func flattenBracketGroup(inner string, tokens *[]string) {
	parts := strings.Fields(inner)
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		// strip any remaining brackets
		p = strings.TrimLeft(p, "[")
		p = strings.TrimRight(p, "]")
		if p == "" {
			continue
		}
		// check for variadic marker
		if p == "..." {
			// attach to previous token
			if len(*tokens) > 0 {
				prev := (*tokens)[len(*tokens)-1]
				(*tokens)[len(*tokens)-1] = prev + " ..."
			}
			continue
		}
		// look ahead for ...
		if i+1 < len(parts) {
			next := strings.TrimRight(parts[i+1], "]")
			if next == "..." {
				*tokens = append(*tokens, "["+p+" ...]")
				i++ // skip ...
				continue
			}
		}
		*tokens = append(*tokens, "["+p+"]")
	}
}

// stripBrackets removes one layer of surrounding brackets.
func stripBrackets(s string) string {
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		return s[1 : len(s)-1]
	}
	return s
}

// isBoolCluster returns true if the token looks like [-abc] — a bracketed
// dash followed by letters with no spaces.
func isBoolCluster(tok string) bool {
	inner := stripBrackets(tok)
	if !strings.HasPrefix(inner, "-") {
		return false
	}
	// bool cluster: -X where X is all letters (no spaces in inner)
	if strings.Contains(inner, " ") {
		return false
	}
	for _, r := range inner[1:] {
		if !isLetter(r) {
			return false
		}
	}
	return len(inner) > 1
}

// isArgFlag checks whether the token at position i is an arg-flag (a flag
// that takes an argument value).
func isArgFlag(tok string, _ []string, _ int) bool {
	inner := stripBrackets(tok)
	parts := strings.SplitN(inner, " ", 2)
	if len(parts) == 2 && strings.HasPrefix(parts[0], "-") && len(parts[0]) == 2 {
		return true
	}
	return false
}

// parseArgFlag extracts the flag rune and argument type from an arg-flag token.
func parseArgFlag(tok string, _ []string, _ int) (short rune, argType string, consumed int) {
	inner := stripBrackets(tok)
	parts := strings.SplitN(inner, " ", 2)
	flag := []rune(parts[0])
	return flag[1], parts[1], 1
}

// parsePositionals converts remaining tokens into positional definitions.
func parsePositionals(tokens []string) []PositionalDef {
	var result []PositionalDef
	for _, tok := range tokens {
		optional := false
		variadic := false

		name := tok
		// check for brackets indicating optional
		if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
			optional = true
			name = name[1 : len(name)-1]
		}

		// check for variadic marker
		if strings.HasSuffix(name, " ...") {
			variadic = true
			name = strings.TrimSuffix(name, " ...")
		} else if strings.HasSuffix(name, "...") {
			variadic = true
			name = strings.TrimSuffix(name, "...")
		}

		// clean up any remaining brackets
		name = strings.TrimLeft(name, "[")
		name = strings.TrimRight(name, "]")

		if name == "" || name == "..." {
			continue
		}

		result = append(result, PositionalDef{
			Name:     name,
			Required: !optional,
			Variadic: variadic,
		})
	}
	return result
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
