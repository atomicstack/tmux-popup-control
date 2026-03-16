package plugin

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)

// optionPair is an internal representation of a tmux option key-value pair.
type optionPair struct {
	Key   string
	Value string
}

type tmuxClient interface {
	Command(parts ...string) (string, error)
	Close() error
}

var newTmuxClient = func(socketPath string) (tmuxClient, error) {
	return gotmux.NewTmux(socketPath)
}

// optionsFn fetches global tmux options. Swapped in tests.
var optionsFn = defaultOptionsFn

func defaultOptionsFn(socketPath string) ([]optionPair, error) {
	client, err := newTmuxClient(socketPath)
	if err != nil {
		return nil, fmt.Errorf("connecting to tmux: %w", err)
	}
	defer client.Close()

	// tmux global options are uniquely keyed — setting @plugin multiple times
	// overwrites rather than appending. Read config file(s) directly instead,
	// the same approach tpm uses.
	raw, err := client.Command("display-message", "-p", "#{config_files}")
	if err != nil {
		return nil, fmt.Errorf("display-message config_files: %w", err)
	}
	var allPairs []optionPair
	for _, cfgPath := range strings.Split(strings.TrimSpace(raw), ",") {
		cfgPath = strings.TrimSpace(cfgPath)
		if cfgPath == "" {
			continue
		}
		pairs, err := parseConfigFile(cfgPath)
		if err != nil {
			continue // skip unreadable files
		}
		allPairs = append(allPairs, pairs...)
	}
	return allPairs, nil
}

// parseConfigFile reads a tmux config file and extracts @plugin declarations.
func parseConfigFile(path string) ([]optionPair, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseConfigLines(string(data)), nil
}

// parseConfigLines extracts @plugin declarations from tmux config content.
// Matches lines like: set -g @plugin 'user/repo' or set-option -g @plugin "user/repo#branch"
func parseConfigLines(content string) []optionPair {
	var pairs []optionPair
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] != "set" && fields[0] != "set-option" {
			continue
		}
		pluginIdx := -1
		for i := 1; i < len(fields); i++ {
			if fields[i] == "@plugin" {
				pluginIdx = i
				break
			}
		}
		if pluginIdx < 0 || pluginIdx+1 >= len(fields) {
			continue
		}
		pairs = append(pairs, optionPair{Key: "@plugin", Value: fields[pluginIdx+1]})
	}
	return pairs
}

// parsePluginEntry parses a single @plugin value like "user/repo#branch".
// Strips surrounding quotes that tmux control-mode may include in option values.
func parsePluginEntry(value string) Plugin {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "'\"")
	source := value
	branch := ""
	if idx := strings.LastIndex(value, "#"); idx >= 0 {
		source = value[:idx]
		branch = value[idx+1:]
	}
	name := path.Base(source)
	name = strings.TrimSuffix(name, ".git")
	return Plugin{
		Name:   name,
		Source: source,
		Branch: branch,
	}
}

// parsePluginEntries filters option pairs for @plugin keys and parses them.
func parsePluginEntries(options []optionPair) []Plugin {
	var plugins []Plugin
	for _, opt := range options {
		if opt.Key != "@plugin" {
			continue
		}
		p := parsePluginEntry(opt.Value)
		plugins = append(plugins, p)
	}
	return plugins
}

// ParseConfig reads @plugin entries from tmux global options and resolves
// each to a Plugin struct with install status.
func ParseConfig(socketPath string) ([]Plugin, error) {
	options, err := optionsFn(socketPath)
	if err != nil {
		return nil, err
	}
	pluginDir := PluginDir()
	plugins := parsePluginEntries(options)
	for i := range plugins {
		plugins[i].Dir = filepath.Join(pluginDir, plugins[i].Name)
		if info, err := os.Stat(plugins[i].Dir); err == nil && info.IsDir() {
			plugins[i].Installed = true
			gitDir := filepath.Join(plugins[i].Dir, ".git")
			if gi, err := os.Stat(gitDir); err == nil {
				plugins[i].UpdatedAt = gi.ModTime()
			}
			if li, err := os.Lstat(plugins[i].Dir); err == nil {
				plugins[i].IsSymlink = li.Mode()&os.ModeSymlink != 0
			}
		}
	}
	return plugins, nil
}
