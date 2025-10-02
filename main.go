package main

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"os"
	"fmt"
)

// Options for the session menu
var options = []string{"kill", "detach", "rename", "new", "switch"}

// Model for the Bubbletea program
type model struct {
	filter string
	selected int
}

// Update function handles input and updates the model
func (m model) Update(msg bubbletea.Msg) (bubbletea.Model, bubbletea.Cmd) {
	switch msg := msg.(type) {
	case bubbletea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, bubbletea.Quit
		case "up":
			if m.selected > 0 {
				m.selected--
			}
		case "down":
			if m.selected < len(options)-1 {
				m.selected++
			}
		}
		
		// Handle filtering
		if msg.String() == "" {
			// Reset filter
			m.filter = ""
		} else {
			m.filter += msg.String()
		}
	}

	return m, nil
}

// View function renders the menu
func (m model) View() string {
	var s string
	filteredOptions := filterOptions(options, m.filter)

	for i, option := range filteredOptions {
		if i == m.selected {
			s += fmt.Sprintf("> %s
", option)
		} else {
			s += fmt.Sprintf("  %s
", option)
		}
	}

	return s
}

// Filter options based on the user's input
func filterOptions(options []string, filter string) []string {
	var filtered []string
	for _, option := range options {
		if fuzzy.Match(option, filter) {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

// Main function to run the program
func main() {
	p := bubbletea.NewProgram(model{})
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting program: %v", err)
		os.Exit(1)
	}
}