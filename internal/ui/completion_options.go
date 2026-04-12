package ui

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/tmuxopts"
)

// commandsCompletingOptions lists commands whose "option" positional should
// be completed from the tmuxopts catalog.
var commandsCompletingOptions = map[string]bool{
	"set-option":        true,
	"set-window-option": true,
	"show-options":      true,
	"show-window-options": true,
}

// commandsCompletingHooks lists commands whose "hook" positional should be
// completed from the tmuxopts catalog hook names.
var commandsCompletingHooks = map[string]bool{
	"set-hook":   true,
	"show-hooks": true,
}

// commandsCompletingOptionValues lists commands whose "value" positional is
// a tmux option value (not some unrelated argument like environment values).
var commandsCompletingOptionValues = map[string]bool{
	"set-option":        true,
	"set-window-option": true,
}

// tmuxOptCompletion returns completion options from the tmux option catalog
// when the current context is an option name, hook name, or option value
// position. The handled return value indicates whether the tmuxopts path
// applies; when false, callers should fall through to the default resolver.
func (m *Model) tmuxOptCompletion(schema *cmdparse.CommandSchema, ctx cmdparse.CompletionContext, filter string) (opts CompletionOptions, handled bool) {
	if schema == nil || ctx.Kind != cmdparse.ContextPositionalValue {
		return CompletionOptions{}, false
	}
	catalog, err := tmuxopts.Default()
	if err != nil || catalog == nil {
		return CompletionOptions{}, false
	}

	switch ctx.ArgType {
	case "option":
		if !commandsCompletingOptions[schema.Name] {
			return CompletionOptions{}, false
		}
		names := catalog.OptionNames()
		names = mergeUserOptions(names, m.userOptionNames)
		descriptions := make(map[string]string, len(names))
		scopes := make(map[string]OptionScope, len(names))
		for _, name := range names {
			descriptions[name] = catalog.OptionSummary(name)
			scopes[name] = primaryScope(catalog, name)
		}
		return CompletionOptions{
			Items:        names,
			Descriptions: descriptions,
			Scopes:       scopes,
			ArgType:      "option",
			TypeLabel:    "tmux-option",
			Prefix:       ctx.Prefix,
		}, true

	case "hook":
		if !commandsCompletingHooks[schema.Name] {
			return CompletionOptions{}, false
		}
		names := catalog.HookNames()
		descriptions := make(map[string]string, len(names))
		scopes := make(map[string]OptionScope, len(names))
		for _, name := range names {
			descriptions[name] = catalog.OptionSummary(name)
			scopes[name] = primaryScope(catalog, name)
		}
		return CompletionOptions{
			Items:        names,
			Descriptions: descriptions,
			Scopes:       scopes,
			ArgType:      "hook",
			TypeLabel:    "tmux-hook",
			Prefix:       ctx.Prefix,
		}, true

	case "value":
		if !commandsCompletingOptionValues[schema.Name] {
			return CompletionOptions{}, false
		}
		optionName := precedingPositional(schema, filter, 0)
		if optionName == "" {
			return CompletionOptions{}, false
		}
		candidates, _ := catalog.ValueCandidates(optionName)
		if len(candidates) == 0 {
			// Signal "handled but empty" so the caller shows a
			// non-intrusive hint placeholder rather than falling back
			// to an unrelated resolver.
			canonical := catalog.Canonicalize(optionName)
			typeLabel := "value"
			if hint := catalog.ValueHint(canonical); hint != "" {
				typeLabel = hint
			}
			return CompletionOptions{
				ArgType:   "value",
				TypeLabel: typeLabel,
				Prefix:    ctx.Prefix,
			}, true
		}
		isColour := false
		if opt, _ := catalog.Lookup(optionName); opt != nil && opt.Type == tmuxopts.TypeColour {
			isColour = true
		}
		values := make([]string, 0, len(candidates))
		labels := make(map[string]string, len(candidates))
		descriptions := make(map[string]string, len(candidates))
		for _, cand := range candidates {
			values = append(values, cand.Value)
			label := cand.Label
			if label == "" {
				label = cand.Value
			}
			if isColour {
				label = decorateColourLabel(label, cand.Value)
			}
			if label != cand.Value {
				labels[cand.Value] = label
			}
			if cand.Description != "" {
				descriptions[cand.Value] = cand.Description
			}
		}
		return CompletionOptions{
			Items:        values,
			Labels:       labels,
			Descriptions: descriptions,
			ArgType:      "value",
			TypeLabel:    optionLabelFor(catalog, optionName),
			Prefix:       ctx.Prefix,
		}, true
	}

	return CompletionOptions{}, false
}

// currentOptionFilterSpan reports the rune range in the current filter text
// that is a tmux option (or hook) name, together with its display scope.
// ok is false when no such span applies. The span covers the full option
// token regardless of cursor position — once an option name has been typed,
// it stays coloured even after the user moves on to type a value.
func (m *Model) currentOptionFilterSpan() (start, end int, scope OptionScope, ok bool) {
	current := m.currentLevel()
	if current == nil || current.Node == nil || !current.Node.FilterCommand {
		return 0, 0, "", false
	}
	if m.commandSchemas == nil {
		return 0, 0, "", false
	}
	schema := m.lookupCommandSchema(current.Filter)
	if schema == nil {
		return 0, 0, "", false
	}
	isOpt := commandsCompletingOptions[schema.Name]
	isHook := commandsCompletingHooks[schema.Name]
	if !isOpt && !isHook {
		return 0, 0, "", false
	}

	optToken, tokenByteStart := findOptionToken(schema, current.Filter, isOpt, isHook)
	if optToken == "" {
		return 0, 0, "", false
	}

	// Prefer the live completion's highlighted candidate so the colour
	// changes in step with the user's arrow-key selection. Fall back to a
	// catalog lookup for the typed prefix — this covers the exact-match
	// dismissal path where m.completion is nil but the prefix is a
	// complete, known option name (e.g. "mouse").
	var sc OptionScope
	if m.completion != nil && m.completion.visible && len(m.completion.filtered) > 0 {
		idx := m.completion.cursor
		if idx >= 0 && idx < len(m.completion.filtered) {
			sc = m.completion.filtered[idx].Scope
		}
	}
	if sc == "" {
		catalog, err := tmuxopts.Default()
		if err == nil && catalog != nil {
			sc = primaryScope(catalog, optToken)
		}
	}
	if sc == "" {
		return 0, 0, "", false
	}
	runeStart := len([]rune(current.Filter[:tokenByteStart]))
	runeEnd := runeStart + len([]rune(optToken))
	return runeStart, runeEnd, sc, true
}

// findOptionToken walks the filter tokens to locate the positional that
// corresponds to an "option" or "hook" argument in the command schema.
// It returns the token text and its byte offset in the filter string.
func findOptionToken(schema *cmdparse.CommandSchema, filter string, isOpt, isHook bool) (token string, byteOffset int) {
	tokens := strings.Fields(filter)
	if len(tokens) < 2 {
		return "", 0
	}

	offsets := tokenByteOffsets(filter, tokens)

	posIndex := 0
	i := 1
	for i < len(tokens) {
		tok := tokens[i]
		if strings.HasPrefix(tok, "-") && len(tok) >= 2 {
			flag := rune(tok[1])
			if cmdparse.SchemaHasArgFlag(schema, flag) {
				i += 2
				continue
			}
			i++
			continue
		}

		pos := cmdparse.PositionalAt(schema, posIndex)
		if pos != nil && ((isOpt && pos.Name == "option") || (isHook && pos.Name == "hook")) {
			return tok, offsets[i]
		}

		if pos != nil && !pos.Variadic {
			posIndex++
		}
		i++
	}
	return "", 0
}

func tokenByteOffsets(s string, tokens []string) []int {
	offsets := make([]int, len(tokens))
	pos := 0
	for i, tok := range tokens {
		idx := strings.Index(s[pos:], tok)
		if idx < 0 {
			break
		}
		offsets[i] = pos + idx
		pos += idx + len(tok)
	}
	return offsets
}

// decorateShowOptionsLine decorates a single `show-options` output line of
// the form "optionName value" so it renders in keeping with the completion
// dropdown's colour cues. The option name is rendered in its scope colour
// when the name resolves to a known catalog entry or starts with "@" (user
// option); additionally, a coloured swatch is prepended to the value when
// the option is TypeColour and the value resolves to a renderable lipgloss
// colour. The returned text contains ANSI escapes. ok is false when the
// line is malformed (no space) or has no applicable decoration. bodyStyle —
// when non-nil — wraps the undecorated text so decorated and undecorated
// spans share consistent foreground/background.
func decorateShowOptionsLine(line string, bodyStyle *lipgloss.Style) (string, bool) {
	sp := strings.IndexByte(line, ' ')
	if sp <= 0 {
		return "", false
	}
	name := line[:sp]
	rest := line[sp:] // leading space preserved
	value := strings.TrimSpace(rest)

	catalog, err := tmuxopts.Default()
	if err != nil || catalog == nil {
		return "", false
	}

	scope := primaryScope(catalog, name)
	scopeStyle := scopeStyleFor(scope)

	var swatch string
	if value != "" {
		if opt, _ := catalog.Lookup(name); opt != nil && opt.Type == tmuxopts.TypeColour {
			if spec, ok := colourSpecForName(value); ok {
				swatch = lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render("█")
			}
		}
	}

	if scopeStyle == nil && swatch == "" {
		return "", false
	}

	renderBody := func(text string) string {
		if bodyStyle == nil {
			return text
		}
		return bodyStyle.Render(text)
	}

	nameRendered := renderBody(name)
	if scopeStyle != nil {
		nameRendered = scopeStyle.Render(name)
	}

	if swatch != "" {
		return nameRendered + renderBody(" ") + swatch + renderBody(" "+value), true
	}
	return nameRendered + renderBody(rest), true
}

// decorateColourLabel prepends a coloured swatch block to a colour value's
// display label when the colour can be resolved by lipgloss. When the colour
// name is an X11 extended name or otherwise unresolvable, a blank padding
// space is prepended so rows still align visually. The swatch is two cells
// wide ("█ ") to leave comfortable breathing room before the name.
func decorateColourLabel(label, value string) string {
	spec, ok := colourSpecForName(value)
	if !ok {
		return "  " + label
	}
	swatch := lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render("█")
	return swatch + " " + label
}

// colourSpecForName returns a lipgloss.Color-compatible spec for the given
// tmux colour name when it is one of the forms lipgloss can render without
// an external name table:
//
//   - The 18 basic tmux names (black/red/…/white, bright variants, default,
//     terminal) map to ANSI indices. "default" and "terminal" are treated
//     as "no swatch" since they have no intrinsic colour.
//   - "colourN" / "colorN" forms map to ANSI index N (0..255).
//   - "#RRGGBB" / "#RGB" pass through unchanged.
//
// Extended X11 colour names (AliceBlue, cornflower blue, …) are NOT
// supported here because lipgloss does not ship an X11 name table; callers
// should present those without a swatch rather than mis-rendering them.
func colourSpecForName(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	if strings.HasPrefix(name, "#") {
		return name, true
	}
	lowered := strings.ToLower(name)
	if idx, ok := basicColourIndices[lowered]; ok {
		if idx < 0 {
			return "", false
		}
		return strconv.Itoa(idx), true
	}
	for _, prefix := range []string{"colour", "color"} {
		if strings.HasPrefix(lowered, prefix) {
			rest := lowered[len(prefix):]
			if n, err := strconv.Atoi(rest); err == nil && n >= 0 && n < 256 {
				return strconv.Itoa(n), true
			}
		}
	}
	return "", false
}

// basicColourIndices maps the 18 tmux basic colour names to their ANSI
// palette indices. "default" and "terminal" resolve to -1 to indicate
// "no intrinsic swatch colour".
var basicColourIndices = map[string]int{
	"default":       -1,
	"terminal":      -1,
	"black":         0,
	"red":           1,
	"green":         2,
	"yellow":        3,
	"blue":          4,
	"magenta":       5,
	"cyan":          6,
	"white":         7,
	"brightblack":   8,
	"brightred":     9,
	"brightgreen":   10,
	"brightyellow":  11,
	"brightblue":    12,
	"brightmagenta": 13,
	"brightcyan":    14,
	"brightwhite":   15,
}

// optionLabelFor returns a short display label for a value-completion
// dropdown header, e.g. "status-keys" or "@user".
func optionLabelFor(catalog *tmuxopts.Catalog, raw string) string {
	canonical := catalog.Canonicalize(raw)
	if canonical == "" {
		return raw
	}
	return canonical
}

// primaryScope derives a single OptionScope for display from a catalog
// entry. The narrowest declared scope wins (pane > window > session >
// server) so pane-capable options are tagged distinctly from options that
// only live at the window level. Unknown names starting with `@` are
// treated as user options; everything else returns the empty scope.
func primaryScope(catalog *tmuxopts.Catalog, name string) OptionScope {
	if catalog == nil {
		if strings.HasPrefix(name, "@") {
			return ScopeUser
		}
		return ""
	}
	opt, pseudo := catalog.Lookup(name)
	if opt == nil && pseudo != nil {
		return ScopeUser
	}
	if opt == nil {
		if strings.HasPrefix(name, "@") {
			return ScopeUser
		}
		return ""
	}
	scopes := opt.Scope.Scopes
	// Rank: pane is the most specific, then window, then session, then server.
	has := func(s tmuxopts.Scope) bool { return slices.Contains(scopes, s) }
	switch {
	case has(tmuxopts.ScopePane):
		return ScopePane
	case has(tmuxopts.ScopeWindow):
		return ScopeWindow
	case has(tmuxopts.ScopeSession):
		return ScopeSession
	case has(tmuxopts.ScopeServer):
		return ScopeServer
	}
	switch opt.Scope.DefaultInferred {
	case tmuxopts.ScopePane:
		return ScopePane
	case tmuxopts.ScopeWindow:
		return ScopeWindow
	case tmuxopts.ScopeSession:
		return ScopeSession
	case tmuxopts.ScopeServer:
		return ScopeServer
	}
	return ""
}

// mergeUserOptions returns a sorted union of catalog option names and
// live user-defined @-option names. Duplicates are dropped. The catalog
// input is expected to already be sorted; the merged slice is sorted as
// a whole to keep ordering stable regardless of input order.
func mergeUserOptions(catalogNames, userNames []string) []string {
	if len(userNames) == 0 {
		return catalogNames
	}
	seen := make(map[string]struct{}, len(catalogNames)+len(userNames))
	merged := make([]string, 0, len(catalogNames)+len(userNames))
	for _, name := range catalogNames {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		merged = append(merged, name)
	}
	for _, name := range userNames {
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		merged = append(merged, name)
	}
	sort.Strings(merged)
	return merged
}

// precedingPositional returns the Nth positional argument in filter, walking
// past the command name and any flag/arg-flag tokens. Returns "" when no such
// positional has been typed yet. The mid-typed token at the cursor position
// is included in the scan, which is the desired behaviour when the caller
// asks for an earlier positional (e.g. positionalIdx=0 while typing a value).
func precedingPositional(schema *cmdparse.CommandSchema, filter string, positionalIdx int) string {
	tokens := strings.Fields(filter)
	if len(tokens) < 2 {
		return ""
	}
	argFlags := make(map[rune]bool, len(schema.ArgFlags))
	for _, af := range schema.ArgFlags {
		argFlags[af.Short] = true
	}
	seen := 0
	i := 1 // skip command name
	for i < len(tokens) {
		tok := tokens[i]
		if strings.HasPrefix(tok, "-") && len(tok) >= 2 && tok != "--" {
			flag := rune(tok[1])
			if argFlags[flag] {
				// arg-flag: skip flag and its value
				i += 2
				continue
			}
			i++
			continue
		}
		if seen == positionalIdx {
			return tok
		}
		seen++
		i++
	}
	return ""
}
