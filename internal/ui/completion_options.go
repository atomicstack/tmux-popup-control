package ui

import (
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/atomicstack/tmux-popup-control/internal/cmdparse"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
	"github.com/atomicstack/tmux-popup-control/internal/tmuxopts"
)

// colourResolver resolves colour names that colourSpecForName cannot map on
// its own — tmux theme names such as themeblue — to a lipgloss colour spec.
// A nil resolver means no external resolution is available.
type colourResolver func(name string) (string, bool)

// resolveThemeColourFn resolves theme colour names through tmux for a given
// socket and TTY client. Injectable so UI tests can stub the lookup.
var resolveThemeColourFn = tmux.ResolveThemeColour

// colourResolver returns a resolver bound to the popup's socket and the
// user's TTY client, so theme colours come back as the user's terminal
// would actually draw them (dark/light theme, 16/256/truecolour). Without
// a known TTY client there is no terminal to resolve against (tmux would
// answer for the control-mode connection), so nil is returned and theme
// names render undecorated.
func (m *Model) colourResolver() colourResolver {
	socket, client := m.socketPath, strings.TrimSpace(m.clientID)
	if client == "" {
		return nil
	}
	return func(name string) (string, bool) {
		return resolveThemeColourFn(socket, client, name)
	}
}

// commandsCompletingOptions lists commands whose "option" positional should
// be completed from the tmuxopts catalog.
var commandsCompletingOptions = map[string]bool{
	"set-option":          true,
	"set-window-option":   true,
	"show-options":        true,
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
		if opt, _ := catalog.Lookup(optionName); opt != nil && opt.IsColour() {
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
				label = decorateColourLabel(label, cand.Value, m.colourResolver())
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
		if spec, ok := colourSpecForName(ft.Value, m.colourResolver()); ok {
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
			i += 1 + flagValueTokensAt(schema, tokens, i)
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
// option); additionally, when the option is a colour option (IsColour) and
// the value resolves to a renderable lipgloss colour, the value text itself is
// rendered in that colour. The returned text contains ANSI escapes. ok is
// false when the line is malformed (no space) or has no applicable
// decoration. bodyStyle — when non-nil — wraps the undecorated text so
// decorated and undecorated spans share consistent foreground/background.
// resolve — when non-nil — maps theme colour names (themeblue, …) to a
// concrete colour; values it cannot resolve are rendered plainly.
func decorateShowOptionsLine(line string, bodyStyle *lipgloss.Style, resolve colourResolver) (string, bool) {
	var name, rest, value string
	sp := strings.IndexByte(line, ' ')
	if sp > 0 {
		name = line[:sp]
		rest = line[sp:]
		value = strings.TrimSpace(rest)
	} else {
		name = line
	}

	lookupName := optionLookupName(name)

	catalog, err := tmuxopts.Default()
	if err != nil || catalog == nil {
		return "", false
	}

	scope := primaryScope(catalog, lookupName)
	scopeStyle := scopeStyleFor(scope)

	renderBody := func(text string) string {
		if text == "" || bodyStyle == nil {
			return text
		}
		return bodyStyle.Render(text)
	}

	var valueRendered string
	if value != "" {
		// tmux quotes values containing format syntax or spaces; decorate
		// the inner text and keep the quotes as plain body text.
		open, inner, closing := unquoteOptionValue(value)
		var innerRendered string
		opt, _ := catalog.Lookup(lookupName)
		if opt != nil && opt.IsColour() {
			// Bare colour value — render the whole value in its colour.
			if spec, ok := colourSpecForName(inner, resolve); ok {
				innerRendered = lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render(inner)
			}
		}
		if innerRendered == "" {
			// Try to colour inline colour references in style attributes
			// (e.g. "fg=colour33", "bg=red,bold").
			innerRendered = decorateStyleValue(inner, bodyStyle, resolve)
		}
		if innerRendered != "" {
			valueRendered = renderBody(open) + innerRendered + renderBody(closing)
		}
	}

	if scopeStyle == nil && valueRendered == "" {
		return "", false
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

// optionLookupName reduces a show-options key to the catalog option name:
// the trailing '*' that show-options -A appends to inherited values and the
// "[N]" index suffix on array option entries (pane-colours[3]) are both
// stripped.
func optionLookupName(name string) string {
	name = strings.TrimRight(name, "*")
	if i := strings.IndexByte(name, '['); i > 0 && strings.HasSuffix(name, "]") {
		name = name[:i]
	}
	return name
}

// unquoteOptionValue splits a show-options value into its surrounding quote
// (if tmux quoted it) and the inner text. open/closing are "" when the
// value is unquoted.
func unquoteOptionValue(value string) (open, inner, closing string) {
	if len(value) >= 2 {
		if q := value[0]; (q == '"' || q == '\'') && value[len(value)-1] == q {
			return value[:1], value[1 : len(value)-1], value[len(value)-1:]
		}
	}
	return "", value, ""
}

// styleColourKeys lists the tmux style attributes whose value is a colour.
var styleColourKeys = map[string]bool{
	"fg":   true,
	"bg":   true,
	"us":   true,
	"fill": true,
}

// decorateStyleValue colours inline colour references in tmux style values
// like "fg=colour33", "bg=red,bold", "fg=#ff00ff,bg=blue". Each attribute
// (see splitStyleAttrs) is checked; those of the form key=colourValue have
// the colour portion rendered in its own colour. Attributes that are format
// strings (#{…} / #[…]) are rendered plainly. Returns "" when no colour
// references were found.
func decorateStyleValue(value string, bodyStyle *lipgloss.Style, resolve colourResolver) string {
	renderBody := func(text string) string {
		if text == "" || bodyStyle == nil {
			return text
		}
		return bodyStyle.Render(text)
	}

	attrs := splitStyleAttrs(value)
	any := false
	parts := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		if key, colourName, ok := strings.Cut(attr, "="); ok && styleColourKeys[key] && colourName != "" {
			if spec, ok := colourSpecForName(colourName, resolve); ok {
				coloured := lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render(colourName)
				parts = append(parts, renderBody(key+"=")+coloured)
				any = true
				continue
			}
		}
		parts = append(parts, renderBody(attr))
	}
	if !any {
		return ""
	}
	return strings.Join(parts, renderBody(","))
}

// splitStyleAttrs splits a tmux style string into its comma-separated
// attributes the way tmux's style parser does: "#," is an escaped comma
// (kept in the attribute), "##" is an escaped hash, and commas inside
// #{…} conditionals or #[…] style blocks never split.
func splitStyleAttrs(value string) []string {
	var parts []string
	start := 0
	braces, brackets := 0, 0
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '#':
			if i+1 < len(value) {
				switch value[i+1] {
				case ',', '#':
					i++
				case '{':
					braces++
					i++
				case '[':
					brackets++
					i++
				}
			}
		case '}':
			if braces > 0 {
				braces--
			}
		case ']':
			if brackets > 0 {
				brackets--
			}
		case ',':
			if braces == 0 && brackets == 0 {
				parts = append(parts, value[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, value[start:])
}

// decorateColourLabel renders a colour value's display label in the colour
// it represents when the colour can be resolved. When the colour name is an
// X11 extended name or otherwise unresolvable, the label is returned
// unchanged.
func decorateColourLabel(label, value string, resolve colourResolver) string {
	spec, ok := colourSpecForName(value, resolve)
	if !ok {
		return label
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(spec)).Render(label)
}

// colourSpecForName returns a lipgloss.Color-compatible spec for the given
// tmux colour name when it is one of the forms that can be rendered without
// an external name table:
//
//   - The 18 basic tmux names (black/red/…/white, bright variants, default,
//     terminal) map to ANSI indices. "default" and "terminal" have no
//     intrinsic colour and return ok=false.
//   - "colourN" / "colorN" forms map to ANSI index N (0..255).
//   - "#RRGGBB" (exactly six hex digits, the only hex form tmux accepts)
//     passes through unchanged.
//   - Anything else is handed to resolve (theme colour names), when given.
//
// Format strings (#{…} / #[…]) are never colours even though they start
// with '#'. Extended X11 colour names (AliceBlue, cornflower blue, …) are
// NOT supported here because lipgloss does not ship an X11 name table;
// callers should present those without colour decoration rather than
// mis-rendering them.
func colourSpecForName(name string, resolve colourResolver) (string, bool) {
	if name == "" || strings.Contains(name, "#{") || strings.Contains(name, "#[") {
		return "", false
	}
	if strings.HasPrefix(name, "#") {
		return name, isHexColour(name)
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
	if resolve != nil {
		return resolve(name)
	}
	return "", false
}

// isHexColour reports whether s is a "#rrggbb" colour, the only hex form
// tmux's colour_fromstring accepts.
func isHexColour(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(s[1:], 16, 32)
	return err == nil
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
	if opt.Kind == tmuxopts.KindHook {
		return ScopeHook
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
	slices.Sort(merged)
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
	seen := 0
	i := 1 // skip command name
	for i < len(tokens) {
		tok := tokens[i]
		if strings.HasPrefix(tok, "-") && len(tok) >= 2 && tok != "--" {
			// skip the flag and, when it takes one, its value
			i += 1 + flagValueTokensAt(schema, tokens, i)
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

// flagValueTokensAt reports how many tokens the flag at tokens[i] consumes
// as its value (0 or 1), honouring optional-value flags that are followed by
// another flag or by nothing.
func flagValueTokensAt(schema *cmdparse.CommandSchema, tokens []string, i int) int {
	flag := rune(tokens[i][1])
	next := ""
	hasNext := i+1 < len(tokens)
	if hasNext {
		next = tokens[i+1]
	}
	return cmdparse.FlagValueTokens(schema, flag, next, hasNext)
}
