package tmuxopts

import (
	"slices"
	"strings"
)

// ValueCandidate is one suggested value for an option along with
// display metadata.
type ValueCandidate struct {
	// Value is the literal text to insert.
	Value string
	// Label is a human-readable display form. Defaults to Value.
	Label string
	// Description is short help text for the candidate, if available.
	Description string
}

// ValueCandidates returns completion candidates for the given option's
// value position. The result is determined by the option's type, choices,
// and value.menu metadata.
//
//   - Unknown or pseudo-options yield an empty result.
//   - Flag options return canonical "on"/"off".
//   - Choice options return their choices slice.
//   - Colour options return the union of basic and extended named colours.
//   - Enum-menu options return the menu values list.
//   - Other types return nil (open-ended); callers should still show the
//     option summary as a hint.
//
// The returned slice is sorted for stable presentation. The second return
// value is the static_checkability policy, which callers can use to decide
// whether to enforce strict client-side validation.
func (c *Catalog) ValueCandidates(name string) ([]ValueCandidate, Checkability) {
	if c == nil {
		return nil, CheckDynamic
	}
	opt, _ := c.Lookup(name)
	if opt == nil {
		return nil, CheckDynamic
	}

	switch opt.Type {
	case TypeFlag:
		if c.domains.Flag != nil && len(c.domains.Flag.CanonicalValues) > 0 {
			return toCandidates(c.domains.Flag.CanonicalValues, nil), opt.Value.StaticCheckability
		}
		return toCandidates([]string{"on", "off"}, nil), opt.Value.StaticCheckability
	case TypeChoice:
		if len(opt.Choices) > 0 {
			return toCandidates(opt.Choices, nil), opt.Value.StaticCheckability
		}
	case TypeColour:
		return colourCandidates(c.domains.Colour), opt.Value.StaticCheckability
	case TypeKey:
		if c.domains.Key != nil {
			return toCandidates(c.domains.Key.BaseKeyNames, nil), opt.Value.StaticCheckability
		}
	}

	// Fall back to the menu-level hints for non-type-specific enums.
	if opt.Value.Menu.Kind == MenuEnum && len(opt.Value.Menu.Values) > 0 {
		return toCandidates(opt.Value.Menu.Values, nil), opt.Value.StaticCheckability
	}

	return nil, opt.Value.StaticCheckability
}

// ValueHint returns a short hint string describing what kind of value
// the option expects — useful for UI ghost text when there are no static
// completion candidates. Returns "" when no useful hint is available.
func (c *Catalog) ValueHint(name string) string {
	opt, pseudo := c.Lookup(name)
	if opt == nil && pseudo == nil {
		return ""
	}
	if pseudo != nil {
		return "string"
	}
	if opt.Value.Menu.Pattern != "" {
		return opt.Value.Menu.Pattern
	}
	if opt.NumericConstraints != nil {
		unit := opt.NumericConstraints.Unit
		rng := opt.NumericConstraints.Minimum + ".." + opt.NumericConstraints.Maximum
		if unit != "" {
			return rng + " " + unit
		}
		return rng
	}
	if opt.Value.Menu.SharedDomainRef != "" {
		return opt.Value.Menu.SharedDomainRef
	}
	if opt.Value.Domain != "" {
		return opt.Value.Domain
	}
	return string(opt.Type)
}

// OptionSummary returns a one-line help summary for the option, or "" if
// the option is unknown. Pseudo-options return their generic summary.
func (c *Catalog) OptionSummary(name string) string {
	opt, pseudo := c.Lookup(name)
	if opt != nil {
		if opt.Summary != "" {
			return opt.Summary
		}
		return opt.Description
	}
	if pseudo != nil {
		return pseudo.Summary
	}
	return ""
}

func colourCandidates(d *ColourDomain) []ValueCandidate {
	if d == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(d.BasicNames)+len(d.ExtendedNamedColours))
	names := make([]string, 0, len(d.BasicNames)+len(d.ExtendedNamedColours))
	for _, n := range d.BasicNames {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	for _, n := range d.ExtendedNamedColours {
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		names = append(names, n)
	}
	// Basic names are lowercase, extended are mixed-case. Sort
	// case-insensitively for a consistent display order.
	slices.SortFunc(names, func(a, b string) int {
		return strings.Compare(strings.ToLower(a), strings.ToLower(b))
	})
	return toCandidates(names, nil)
}

func toCandidates(values []string, descriptions map[string]string) []ValueCandidate {
	out := make([]ValueCandidate, 0, len(values))
	for _, v := range values {
		c := ValueCandidate{Value: v, Label: v}
		if d, ok := descriptions[v]; ok {
			c.Description = d
		}
		out = append(out, c)
	}
	return out
}
