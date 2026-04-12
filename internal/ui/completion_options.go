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

	ft := findFilterTokens(schema, current.Filter, isOpt, isHook)
	if ft.Option == "" {
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
			sc = primaryScope(catalog, ft.Option)
		}
	}
	if sc == "" {
		return 0, 0, "", false
	}
	runeStart := len([]rune(current.Filter[:ft.OptionByte]))
	runeEnd := runeStart + len([]rune(ft.Option))
	return runeStart, runeEnd, sc, true
}

// filterColourSpans returns the set of coloured spans to apply in the filter
// prompt: the option name in its scope colour, and (when applicable) the
// value token rendered in its own colour for colour-typed options.
func (m *Model) filterColourSpans() []filterSpan {
	current := m.currentLevel()
	if current == nil || current.Node == nil || !current.Node.FilterCommand {
		return nil
	}
	if m.commandSchemas == nil {
		return nil
	}
	schema := m.lookupCommandSchema(current.Filter)
	if schema == nil {
		return nil
	}
	isOpt := commandsCompletingOptions[schema.Name]
	isHook := commandsCompletingHooks[schema.Name]
	if !isOpt && !isHook {
		return nil
	}

	ft := findFilterTokens(schema, current.Filter, isOpt, isHook)
	if ft.Option == "" {
		return nil
	}

	var spans []filterSpan

	// option name span — scope colour
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
			sc = primaryScope(catalog, ft.Option)
		}
	}
	if ss := scopeStyleFor(sc); ss != nil {
		runeStart := len([]rune(current.Filter[:ft.OptionByte]))
		spans = append(spans, filterSpan{
			Start: runeStart,
			End:   runeStart + len([]rune(ft.Option)),
			Style: *ss,
		})
	}

	// value span — render in the colour the value represents
	if ft.Value != "" && commandsCompletingOptionValues[schema.Name] {
		if spec, ok := colourSpecForName(ft.Value); ok {
			runeStart := len([]rune(current.Filter[:ft.ValueByte]))
			spans = append(spans, filterSpan{
				Start: runeStart,
				End:   runeStart + len([]rune(ft.Value)),
				Style: lipgloss.NewStyle().Foreground(lipgloss.Color(spec)),
			})
		}
	}

	return spans
}

// filterTokens holds the option and value tokens found in the filter text.
type filterTokens struct {
	Option     string
	OptionByte int
	Value      string
	ValueByte  int
}

// findFilterTokens walks the filter tokens to locate the positionals that
// correspond to "option"/"hook" and "value" arguments in the command schema.
func findFilterTokens(schema *cmdparse.CommandSchema, filter string, isOpt, isHook bool) filterTokens {
	tokens := strings.Fields(filter)
	if len(tokens) < 2 {
		return filterTokens{}
	}

	offsets := tokenByteOffsets(filter, tokens)
	var result filterTokens
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
			result.Option = tok
			result.OptionByte = offsets[i]
		} else if pos != nil && pos.Name == "value" && result.Option != "" {
			result.Value = tok
			result.ValueByte = offsets[i]
			return result
		}

		if pos != nil && !pos.Variadic {
			posIndex++
		}
		i++
	}
	return result
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
// option); additionally, when the option is TypeColour and the value
// resolves to a renderable lipgloss colour, the value text itself is
// rendered in that colour. The returned text contains ANSI escapes. ok is
// false when the line is malformed (no space) or has no applicable
// decoration. bodyStyle — when non-nil — wraps the undecorated text so
// decorated and undecorated spans share consistent foreground/background.
func decorateShowOptionsLine(line string, bodyStyle *lipgloss.Style) (string, bool) {
	sp := strings.IndexByte(line, ' ')
	if sp <= 0 {
		return "", false
	}
	name := line[:sp]
	rest := line[sp:] // leading space preserved
	value := strings.TrimSpace(rest)

	// Strip the trailing '*' that tmux appends for inherited values in
	// show-options -A output before looking up the catalog entry.
	lookupName := strings.TrimRight(name, "*")

	catalog, err := tmuxopts.Default()
	if err != nil || catalog == nil {
		return "", false
	}

	scope := primaryScope(catalog, lookupName)
	scopeStyle := scopeStyleFor(scope)

	var valueRendered string
	if value != "" {
		opt, _ := catalog.Lookup(lookupName)
		if opt != nil && opt.Type == tmuxopts.TypeColour {
			// Bare colour value — render the whole value in its colour.
			if spec, ok := colourSpecForName(value); ok {
				valueRendered = lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render(value)
			}
		}
		if valueRendered == "" {
			// Try to colour inline colour references in style attributes
			// (e.g. "fg=colour33", "bg=red,bold").
			valueRendered = decorateStyleValue(value, bodyStyle)
		}
	}

	if scopeStyle == nil && valueRendered == "" {
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

	if valueRendered != "" {
		return nameRendered + renderBody(" ") + valueRendered, true
	}
	return nameRendered + renderBody(rest), true
}

// decorateStyleValue colours inline colour references in tmux style values
// like "fg=colour33", "bg=red,bold", "fg=#ff00ff,bg=blue". Each comma-
// separated attribute is checked; those of the form key=colourValue have
// the colour portion rendered in its own colour. Returns "" when no colour
// references were found.
func decorateStyleValue(value string, bodyStyle *lipgloss.Style) string {
	renderBody := func(text string) string {
		if bodyStyle == nil {
			return text
		}
		return bodyStyle.Render(text)
	}

	attrs := strings.Split(value, ",")
	any := false
	var parts []string
	for _, attr := range attrs {
		if eq := strings.IndexByte(attr, '='); eq >= 0 {
			key := attr[:eq]
			colourName := attr[eq+1:]
			if (key == "fg" || key == "bg") && colourName != "" {
				if spec, ok := colourSpecForName(colourName); ok {
					coloured := lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render(colourName)
					parts = append(parts, renderBody(key+"=")+coloured)
					any = true
					continue
				}
			}
		}
		parts = append(parts, renderBody(attr))
	}
	if !any {
		return ""
	}
	return strings.Join(parts, renderBody(","))
}

// decorateColourLabel renders a colour value's display label in the colour
// it represents when the colour can be resolved by lipgloss. When the colour
// name is an X11 extended name or otherwise unresolvable, the label is
// returned unchanged.
func decorateColourLabel(label, value string) string {
	spec, ok := colourSpecForName(value)
	if !ok {
		return label
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render(label)
}

// colourSpecForName returns a lipgloss.Color-compatible spec for the given
// tmux colour name when it is one of the forms lipgloss can render without
// an external name table:
//
//   - The 18 basic tmux names (black/red/…/white, bright variants, default,
//     terminal) map to ANSI indices. "default" and "terminal" have no
//     intrinsic colour and return ok=false.
//   - "colourN" / "colorN" forms map to ANSI index N (0..255).
//   - "#RRGGBB" / "#RGB" pass through unchanged.
//
// Extended X11 colour names (AliceBlue, cornflower blue, …) are NOT
// supported here because lipgloss does not ship an X11 name table; callers
// should present those without colour decoration rather than mis-rendering them.
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
// "no intrinsic colour".
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
