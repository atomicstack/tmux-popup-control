// Package tmuxopts exposes the tmux option catalog as Go data.
//
// The catalog is generated upstream (see ~/git_tree/tmux/tmux-option-catalog.md
// for the schema) and embedded here so consumers can offer completion,
// validation, and inline help for tmux option names and values without any
// tmux runtime dependency.
//
// The package is deliberately consumer-agnostic: it knows nothing about
// bubbletea, lipgloss, or the command menu. Callers decide how to present
// the information.
package tmuxopts

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
)

//go:embed tmux-option-catalog.json
var embeddedCatalog []byte

// Kind is "option" or "hook".
type Kind string

const (
	KindOption Kind = "option"
	KindHook   Kind = "hook"
)

// Type is the base tmux type of an option's value.
type Type string

const (
	TypeString  Type = "string"
	TypeNumber  Type = "number"
	TypeKey     Type = "key"
	TypeColour  Type = "colour"
	TypeFlag    Type = "flag"
	TypeChoice  Type = "choice"
	TypeCommand Type = "command"
)

// MenuKind guides completion UI behaviour.
type MenuKind string

const (
	MenuEnum     MenuKind = "enum"
	MenuInteger  MenuKind = "integer"
	MenuPattern  MenuKind = "pattern"
	MenuPath     MenuKind = "path"
	MenuFreeform MenuKind = "freeform"
)

// Checkability describes how aggressively client-side validation can reject.
type Checkability string

const (
	CheckStaticExact   Checkability = "static-exact"
	CheckStaticRange   Checkability = "static-range"
	CheckStaticPattern Checkability = "static-pattern"
	CheckStaticPartial Checkability = "static-partial"
	CheckDynamic       Checkability = "dynamic"
)

// Status marks deprecated or obsolete entries.
type Status string

const (
	StatusDeprecated Status = "deprecated"
	StatusObsolete   Status = "obsolete"
)

// Scope is a tmux option scope.
type Scope string

const (
	ScopeServer  Scope = "server"
	ScopeSession Scope = "session"
	ScopeWindow  Scope = "window"
	ScopePane    Scope = "pane"
)

// ScopeInfo carries scope details for an option.
type ScopeInfo struct {
	Scopes         []Scope `json:"scopes"`
	DefaultInferred Scope  `json:"default_inferred_scope"`
}

// ValueMenu describes how a UI should complete a value for this option.
type ValueMenu struct {
	Kind            MenuKind `json:"kind"`
	Values          []string `json:"values,omitempty"`
	Minimum         string   `json:"minimum,omitempty"`
	Maximum         string   `json:"maximum,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	Pattern         string   `json:"pattern,omitempty"`
	SharedDomainRef string   `json:"shared_domain_ref,omitempty"`
}

// ValueInfo captures the value-side metadata of an option.
type ValueInfo struct {
	Domain             string       `json:"domain"`
	StaticCheckability Checkability `json:"static_checkability"`
	SetTimeValidation  string       `json:"set_time_validation"`
	ToggleWithoutValue bool         `json:"toggle_without_value"`
	Menu               ValueMenu    `json:"menu"`
	Notes              []string     `json:"notes,omitempty"`
}

// NumericConstraints holds numeric range info.
type NumericConstraints struct {
	Minimum string `json:"minimum"`
	Maximum string `json:"maximum"`
	Unit    string `json:"unit,omitempty"`
}

// Option is one tmux option or hook entry.
type Option struct {
	Name               string              `json:"name"`
	SyntaxName         string              `json:"syntax_name"`
	Kind               Kind                `json:"kind"`
	Scope              ScopeInfo           `json:"scope"`
	Type               Type                `json:"type"`
	Array              bool                `json:"array"`
	StyleOption        bool                `json:"style_option"`
	AlternativeNames   []string            `json:"alternative_names,omitempty"`
	Summary            string              `json:"summary"`
	Description        string              `json:"description"`
	Value              ValueInfo           `json:"value"`
	Choices            []string            `json:"choices,omitempty"`
	Pattern            string              `json:"pattern,omitempty"`
	Separator          string              `json:"separator,omitempty"`
	NumericConstraints *NumericConstraints `json:"numeric_constraints,omitempty"`
	Status             Status              `json:"status,omitempty"`
}

// PseudoOption represents a schema entry for names not enumerated
// concretely (currently only `@<name>` user options).
type PseudoOption struct {
	NamePattern string    `json:"name_pattern"`
	Kind        string    `json:"kind"`
	Scope       ScopeInfo `json:"scope"`
	Type        Type      `json:"type"`
	Array       bool      `json:"array"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Value       ValueInfo `json:"value"`
	Notes       []string  `json:"notes,omitempty"`
}

// NameAliases maps alternative option spellings to canonical names.
type NameAliases struct {
	AliasToCanonical   map[string]string   `json:"alias_to_canonical"`
	CanonicalToAliases map[string][]string `json:"canonical_to_aliases"`
}

// SharedDomains holds reusable value spaces referenced by options.
type SharedDomains struct {
	Colour                       *ColourDomain           `json:"colour,omitempty"`
	Flag                         *FlagDomain             `json:"flag,omitempty"`
	Key                          *KeyDomain              `json:"key,omitempty"`
	Style                        *StyleDomain            `json:"style,omitempty"`
	TmuxCommand                  map[string]any          `json:"tmux_command,omitempty"`
	FormatString                 map[string]any          `json:"format_string,omitempty"`
	TerminalFeatures             []string                `json:"terminal_features,omitempty"`
	TerminalOverrideCapabilities []TerminalOverrideEntry `json:"terminal_override_capabilities,omitempty"`
}

// TerminalOverrideEntry describes one entry in terminal_override_capabilities.
type TerminalOverrideEntry struct {
	Name      string `json:"name"`
	ValueType string `json:"value_type"`
}

// ColourDomain enumerates tmux colour literals.
type ColourDomain struct {
	BasicNames            []string `json:"basic_names"`
	ExtendedNamedColours  []string `json:"extended_named_colours"`
	OtherAcceptedForms    []string `json:"other_accepted_forms,omitempty"`
}

// FlagDomain enumerates on/off literals.
type FlagDomain struct {
	CanonicalValues  []string            `json:"canonical_values"`
	AcceptedLiterals map[string][]string `json:"accepted_literals"`
}

// KeyDomain enumerates tmux key names.
type KeyDomain struct {
	BaseKeyNames []string `json:"base_key_names"`
}

// StyleDomain enumerates style keywords.
type StyleDomain struct {
	Attributes            []string       `json:"attributes"`
	AttributeNegationForm string         `json:"attribute_negation_form"`
	OtherKeywords         []string       `json:"other_keywords"`
	ParameterizedTokens   map[string]any `json:"parameterized_tokens"`
	ContainsRuntimeFormats bool          `json:"contains_runtime_formats"`
}

// rawCatalog is the on-disk JSON shape.
type rawCatalog struct {
	SchemaVersion int            `json:"schema_version"`
	Options       []Option       `json:"options"`
	PseudoOptions []PseudoOption `json:"pseudo_options"`
	NameAliases   NameAliases    `json:"name_aliases"`
	SharedDomains SharedDomains  `json:"shared_domains"`
}

// Catalog is the parsed tmux option catalog with fast lookup helpers.
type Catalog struct {
	schemaVersion int
	options       []Option
	byName        map[string]*Option
	pseudo        []PseudoOption
	aliases       NameAliases
	domains       SharedDomains
	names         []string // sorted canonical names (options)
	hookNames     []string // sorted canonical names (hooks)
}

var (
	defaultOnce    sync.Once
	defaultCatalog *Catalog
	defaultErr     error
)

// Default returns the catalog loaded from the embedded JSON. It is safe
// for concurrent use and parses the JSON only once.
func Default() (*Catalog, error) {
	defaultOnce.Do(func() {
		defaultCatalog, defaultErr = Load(embeddedCatalog)
	})
	return defaultCatalog, defaultErr
}

// MustDefault is like Default but panics on error. Use during program
// startup where a malformed embedded catalog is a bug.
func MustDefault() *Catalog {
	c, err := Default()
	if err != nil {
		panic(fmt.Errorf("tmuxopts: failed to load embedded catalog: %w", err))
	}
	return c
}

// Load parses the given JSON bytes into a Catalog.
func Load(data []byte) (*Catalog, error) {
	var raw rawCatalog
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("tmuxopts: decode catalog: %w", err)
	}
	c := &Catalog{
		schemaVersion: raw.SchemaVersion,
		options:       raw.Options,
		byName:        make(map[string]*Option, len(raw.Options)),
		pseudo:        raw.PseudoOptions,
		aliases:       raw.NameAliases,
		domains:       raw.SharedDomains,
	}
	for i := range c.options {
		opt := &c.options[i]
		c.byName[opt.Name] = opt
		if opt.Kind == KindHook {
			c.hookNames = append(c.hookNames, opt.Name)
		} else {
			c.names = append(c.names, opt.Name)
		}
	}
	slices.Sort(c.names)
	slices.Sort(c.hookNames)
	return c, nil
}

// SchemaVersion returns the schema version of the embedded catalog.
func (c *Catalog) SchemaVersion() int {
	if c == nil {
		return 0
	}
	return c.schemaVersion
}

// OptionNames returns sorted canonical names of regular options (kind=option).
// The returned slice is a copy; callers may mutate it freely.
func (c *Catalog) OptionNames() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.names)
}

// HookNames returns sorted canonical names of hook options (kind=hook).
func (c *Catalog) HookNames() []string {
	if c == nil {
		return nil
	}
	return slices.Clone(c.hookNames)
}

// AllNames returns the union of OptionNames and HookNames, sorted.
func (c *Catalog) AllNames() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.names)+len(c.hookNames))
	names = append(names, c.names...)
	names = append(names, c.hookNames...)
	slices.Sort(names)
	return names
}

// Pseudo returns the pseudo-option entries (e.g. @<name>).
func (c *Catalog) Pseudo() []PseudoOption {
	if c == nil {
		return nil
	}
	return slices.Clone(c.pseudo)
}

// SharedDomains exposes the catalog's shared value spaces.
func (c *Catalog) SharedDomains() SharedDomains {
	if c == nil {
		return SharedDomains{}
	}
	return c.domains
}

// Lookup resolves name to an Option entry, normalizing aliases and ignoring
// trailing `[index]` array subscripts. If name matches a pseudo-option
// pattern, Lookup returns nil and pseudo is true.
//
// Returns:
//   - opt != nil, pseudo == nil  → concrete catalog entry
//   - opt == nil, pseudo != nil  → pseudo-option (e.g. @foo)
//   - opt == nil, pseudo == nil  → unknown option
func (c *Catalog) Lookup(name string) (opt *Option, pseudo *PseudoOption) {
	if c == nil || name == "" {
		return nil, nil
	}
	canonical := c.Canonicalize(name)
	if o, ok := c.byName[canonical]; ok {
		return o, nil
	}
	if strings.HasPrefix(canonical, "@") && len(c.pseudo) > 0 {
		return nil, &c.pseudo[0]
	}
	return nil, nil
}

// Canonicalize normalizes a user-typed option name by:
//   - stripping a trailing `[N]` array subscript
//   - applying the alias-to-canonical map
//
// It does not validate that the result exists in the catalog.
func (c *Catalog) Canonicalize(name string) string {
	if c == nil || name == "" {
		return name
	}
	stripped := stripArrayIndex(name)
	if canonical, ok := c.aliases.AliasToCanonical[stripped]; ok {
		return canonical
	}
	return stripped
}

// IsKnown reports whether name resolves to any catalog entry (including
// pseudo-options).
func (c *Catalog) IsKnown(name string) bool {
	opt, pseudo := c.Lookup(name)
	return opt != nil || pseudo != nil
}

// stripArrayIndex removes a trailing `[...]` subscript from name.
func stripArrayIndex(name string) string {
	if i := strings.IndexByte(name, '['); i > 0 && strings.HasSuffix(name, "]") {
		return name[:i]
	}
	return name
}
