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

	raw, err := client.Command("show-options", "-g")
	if err != nil {
		return nil, fmt.Errorf("show-options -g: %w", err)
	}
	var pairs []optionPair
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		pairs = append(pairs, optionPair{Key: parts[0], Value: parts[1]})
	}
	return pairs, nil
}

// parsePluginEntry parses a single @plugin value like "user/repo#branch".
func parsePluginEntry(value string) Plugin {
	value = strings.TrimSpace(value)
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
