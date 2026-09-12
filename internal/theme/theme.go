package theme

import "charm.land/lipgloss/v2"

// Styles describes reusable Lip Gloss styles shared across the UI.
type Styles struct {
	Loading               *lipgloss.Style
	Item                  *lipgloss.Style
	ItemIndicator         *lipgloss.Style
	SelectedItemIndicator *lipgloss.Style
	SelectedItem          *lipgloss.Style
	Error                 *lipgloss.Style
	Warning               *lipgloss.Style
	Info                  *lipgloss.Style
	Header                *lipgloss.Style
	Footer                *lipgloss.Style
	Filter                *lipgloss.Style
	FilterPrompt          *lipgloss.Style
	FilterPlaceholder     *lipgloss.Style
	SelectorValue         *lipgloss.Style
	SelectorHintKey       *lipgloss.Style
	Cursor                *lipgloss.Style
	PreviewTitle          *lipgloss.Style
	PreviewBody           *lipgloss.Style
	PreviewError          *lipgloss.Style
	Checkbox              *lipgloss.Style
	CheckboxChecked       *lipgloss.Style
	CheckboxAll           *lipgloss.Style
	ProgressFilled        *lipgloss.Style
	ProgressEmpty         *lipgloss.Style
	ProgressEmptyBg       *lipgloss.Style
	HeaderItem            *lipgloss.Style
	CompletionBorder      *lipgloss.Style
	CompletionItem        *lipgloss.Style
	CompletionSelected    *lipgloss.Style
	OptionScopeServer     *lipgloss.Style
	OptionScopeSession    *lipgloss.Style
	OptionScopeWindow     *lipgloss.Style
	OptionScopePane       *lipgloss.Style
	OptionScopeUser       *lipgloss.Style
	OptionScopeHook       *lipgloss.Style
}

var defaultStyles = Styles{
	Loading: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Italic(true),
	),
	Item: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	ItemIndicator: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	),
	SelectedItemIndicator: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Background(lipgloss.Color("238")),
	),
	SelectedItem: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("238")).Bold(true),
	),
	Error: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
	),
	Warning: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
	),
	Info: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	Header: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true),
	),
	Footer: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	Filter: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	FilterPrompt: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true),
	),
	FilterPlaceholder: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	),
	// SelectorValue styles the active value in the extract selector bar
	// ("mode: <value>", "area: <value>") with the same accent blue (33) as
	// the active item indicator.
	SelectorValue: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true),
	),
	// SelectorHintKey styles the key names inside the angle brackets on the
	// extract bar — the selector hotkeys ("<^f>", "<^g>") and the action hints
	// ("insert: <Enter>", "copy: <Tab>") — a slightly lighter grey (245) than
	// the surrounding labels/brackets (FilterPlaceholder, 241).
	SelectorHintKey: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	),
	Cursor: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("33")).Blink(true),
	),
	PreviewTitle: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true),
	),
	PreviewBody: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("250")),
	),
	PreviewError: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
	),
	Checkbox: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	),
	CheckboxChecked: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true),
	),
	CheckboxAll: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true),
	),
	ProgressFilled: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")),
	),
	ProgressEmpty: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("238")),
	),
	ProgressEmptyBg: new(
		lipgloss.NewStyle().Background(lipgloss.Color("#222222")),
	),
	HeaderItem: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
	),
	CompletionBorder: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	),
	CompletionItem: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	CompletionSelected: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Background(lipgloss.Color("240")),
	),
	OptionScopeServer: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
	),
	OptionScopeSession: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("39")),
	),
	OptionScopeWindow: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("170")),
	),
	OptionScopePane: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("84")),
	),
	OptionScopeUser: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("220")),
	),
	OptionScopeHook: new(
		lipgloss.NewStyle().Foreground(lipgloss.Color("139")),
	),
}

// Default exposes the standard style set used across the application.
func Default() *Styles {
	return &defaultStyles
}
