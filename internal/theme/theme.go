package theme

import "github.com/charmbracelet/lipgloss"

// Styles describes reusable Lip Gloss styles shared across the UI.
type Styles struct {
	Loading           *lipgloss.Style
	Item              *lipgloss.Style
	SelectedItem      *lipgloss.Style
	Error             *lipgloss.Style
	Info              *lipgloss.Style
	Header            *lipgloss.Style
	Footer            *lipgloss.Style
	Filter            *lipgloss.Style
	FilterPrompt      *lipgloss.Style
	FilterPlaceholder *lipgloss.Style
	Cursor            *lipgloss.Style
}

var defaultStyles = Styles{
	Loading: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Italic(true),
	),
	Item: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	SelectedItem: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Underline(true),
	),
	Error: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true),
	),
	Info: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	Header: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true),
	),
	Footer: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	Filter: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("249")),
	),
	FilterPrompt: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true),
	),
	FilterPlaceholder: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	),
	Cursor: ptr(
		lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("33")).Blink(true),
	),
}

// Default exposes the standard style set used across the application.
func Default() *Styles {
	return &defaultStyles
}

func ptr(style lipgloss.Style) *lipgloss.Style {
	return &style
}
