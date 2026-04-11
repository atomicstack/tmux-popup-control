package ui

import (
	"strings"

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
		descriptions := make(map[string]string, len(names))
		for _, name := range names {
			descriptions[name] = catalog.OptionSummary(name)
		}
		return CompletionOptions{
			Items:        names,
			Descriptions: descriptions,
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
		for _, name := range names {
			descriptions[name] = catalog.OptionSummary(name)
		}
		return CompletionOptions{
			Items:        names,
			Descriptions: descriptions,
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
		values := make([]string, 0, len(candidates))
		labels := make(map[string]string, len(candidates))
		descriptions := make(map[string]string, len(candidates))
		for _, cand := range candidates {
			values = append(values, cand.Value)
			if cand.Label != "" && cand.Label != cand.Value {
				labels[cand.Value] = cand.Label
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

// optionLabelFor returns a short display label for a value-completion
// dropdown header, e.g. "status-keys" or "@user".
func optionLabelFor(catalog *tmuxopts.Catalog, raw string) string {
	canonical := catalog.Canonicalize(raw)
	if canonical == "" {
		return raw
	}
	return canonical
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
