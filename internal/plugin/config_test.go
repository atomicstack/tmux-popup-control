package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePluginEntry(t *testing.T) {
	tests := []struct {
		value  string
		name   string
		source string
		branch string
	}{
		{"tmux-plugins/tmux-sensible", "tmux-sensible", "tmux-plugins/tmux-sensible", ""},
		{"tmux-plugins/tmux-resurrect#v4.0.0", "tmux-resurrect", "tmux-plugins/tmux-resurrect", "v4.0.0"},
		{"git@github.com:user/my-plugin", "my-plugin", "git@github.com:user/my-plugin", ""},
		{"https://github.com/user/my-plugin.git", "my-plugin", "https://github.com/user/my-plugin.git", ""},
		{"user/plugin-name#main", "plugin-name", "user/plugin-name", "main"},
		{`"tmux-plugins/tmux-sensible"`, "tmux-sensible", "tmux-plugins/tmux-sensible", ""},
		{`'tmux-plugins/tmux-yank'`, "tmux-yank", "tmux-plugins/tmux-yank", ""},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			p := parsePluginEntry(tt.value)
			if p.Name != tt.name {
				t.Errorf("Name = %q, want %q", p.Name, tt.name)
			}
			if p.Source != tt.source {
				t.Errorf("Source = %q, want %q", p.Source, tt.source)
			}
			if p.Branch != tt.branch {
				t.Errorf("Branch = %q, want %q", p.Branch, tt.branch)
			}
		})
	}
}

func TestParseConfigLines(t *testing.T) {
	content := `
# comment line
set -g @plugin 'tmux-plugins/tmux-sensible'
set -g @plugin 'tmux-plugins/tmux-yank'
set-option -g @plugin "user/my-plugin#dev"
set -g status-left ''
# another comment
set -gq @plugin bare-plugin
run '$HOME/.tmux/plugins/tpm/tpm'
`
	pairs := parseConfigLines(content)
	want := []struct {
		key   string
		value string
	}{
		{"@plugin", "'tmux-plugins/tmux-sensible'"},
		{"@plugin", "'tmux-plugins/tmux-yank'"},
		{"@plugin", `"user/my-plugin#dev"`},
		{"@plugin", "bare-plugin"},
	}
	if len(pairs) != len(want) {
		t.Fatalf("got %d pairs, want %d: %+v", len(pairs), len(want), pairs)
	}
	for i, w := range want {
		if pairs[i].Key != w.key || pairs[i].Value != w.value {
			t.Errorf("pairs[%d] = {%q, %q}, want {%q, %q}", i, pairs[i].Key, pairs[i].Value, w.key, w.value)
		}
	}
}

func TestParseConfigLines_Empty(t *testing.T) {
	pairs := parseConfigLines("")
	if len(pairs) != 0 {
		t.Fatalf("got %d pairs, want 0", len(pairs))
	}
	pairs = parseConfigLines("# only comments\n# more comments\n")
	if len(pairs) != 0 {
		t.Fatalf("got %d pairs for comments-only, want 0", len(pairs))
	}
}

func TestParseConfigFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "tmux.conf")
	content := "set -g @plugin 'tmux-plugins/tmux-sensible'\nset -g @plugin 'tmux-plugins/tmux-yank'\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	pairs, err := parseConfigFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 {
		t.Fatalf("got %d pairs, want 2", len(pairs))
	}
	if pairs[0].Value != "'tmux-plugins/tmux-sensible'" {
		t.Errorf("pairs[0].Value = %q, want quoted sensible", pairs[0].Value)
	}
	if pairs[1].Value != "'tmux-plugins/tmux-yank'" {
		t.Errorf("pairs[1].Value = %q, want quoted yank", pairs[1].Value)
	}
}

func TestParsePluginEntries(t *testing.T) {
	options := []optionPair{
		{"@plugin", "tmux-plugins/tpm"},
		{"@plugin", "tmux-plugins/tmux-sensible"},
		{"status-left", "something"},
		{"@plugin", "user/my-plugin#dev"},
	}
	plugins := parsePluginEntries(options)
	if len(plugins) != 3 {
		t.Fatalf("got %d plugins, want 3", len(plugins))
	}
	if plugins[0].Name != "tpm" {
		t.Errorf("plugins[0].Name = %q, want %q", plugins[0].Name, "tpm")
	}
	if plugins[2].Branch != "dev" {
		t.Errorf("plugins[2].Branch = %q, want %q", plugins[2].Branch, "dev")
	}
}

func TestParseConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", dir)

	sensibleDir := filepath.Join(dir, "tmux-sensible")
	if err := os.MkdirAll(filepath.Join(sensibleDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	origFn := optionsFn
	t.Cleanup(func() { optionsFn = origFn })
	optionsFn = func(socketPath string) ([]optionPair, error) {
		return []optionPair{
			{"@plugin", "tmux-plugins/tpm"},
			{"@plugin", "tmux-plugins/tmux-sensible"},
			{"@plugin", "user/not-installed"},
		}, nil
	}

	plugins, err := ParseConfig("/tmp/test.sock")
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 3 {
		t.Fatalf("got %d plugins, want 3", len(plugins))
	}

	var sensible Plugin
	for _, p := range plugins {
		if p.Name == "tmux-sensible" {
			sensible = p
			break
		}
	}
	if !sensible.Installed {
		t.Error("tmux-sensible should be marked installed")
	}

	var notInstalled Plugin
	for _, p := range plugins {
		if p.Name == "not-installed" {
			notInstalled = p
			break
		}
	}
	if notInstalled.Installed {
		t.Error("not-installed should not be marked installed")
	}
}
