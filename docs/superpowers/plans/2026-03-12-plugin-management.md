# Plugin Management Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace tpm by adding plugin management to tmux-popup-control — an `internal/plugin/` package for git-based operations, a "plugins" menu in the popup UI, and an `init-plugins` CLI subcommand.

**Architecture:** New `internal/plugin/` package encapsulates all plugin logic (parse config, install, update, uninstall, tidy, source). Menu layer (`internal/menu/plugins.go`) provides loaders and actions that call into the plugin package. Two new UI modes (`ModePluginConfirm`, `ModePluginReload`) handle deletion confirmation and reload prompts. The `init-plugins` subcommand reuses the plugin package for tmux startup sourcing.

**Tech Stack:** Go, Bubble Tea (charm.land/bubbletea/v2), lipgloss, gotmuxcc (vendored), os/exec for git

**Spec:** `docs/superpowers/specs/2026-03-12-plugin-management-design.md`

---

## Chunk 1: Core plugin package

### Task 1: Plugin types and directory resolution

**Files:**
- Create: `internal/plugin/plugin.go`
- Create: `internal/plugin/plugin_test.go`

- [ ] **Step 1: Write tests for PluginDir resolution**

In `internal/plugin/plugin_test.go`:

```go
package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginDir_EnvVar(t *testing.T) {
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", "/custom/plugins")
	got := PluginDir()
	if got != "/custom/plugins" {
		t.Errorf("PluginDir() = %q, want %q", got, "/custom/plugins")
	}
}

func TestPluginDir_XDG(t *testing.T) {
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", "")
	xdg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	// Create the tmux config directory to trigger XDG detection
	tmuxDir := filepath.Join(xdg, "tmux")
	if err := os.MkdirAll(tmuxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmuxDir, "tmux.conf"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := PluginDir()
	want := filepath.Join(xdg, "tmux", "plugins")
	if got != want {
		t.Errorf("PluginDir() = %q, want %q", got, want)
	}
}

func TestPluginDir_Default(t *testing.T) {
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	got := PluginDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".tmux", "plugins")
	if got != want {
		t.Errorf("PluginDir() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run TestPluginDir -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement Plugin type and PluginDir**

In `internal/plugin/plugin.go`:

```go
package plugin

import (
	"os"
	"path/filepath"
	"time"
)

// Plugin represents a declared or installed tmux plugin.
type Plugin struct {
	Name      string
	Source    string
	Branch    string
	Dir       string
	Installed bool
	UpdatedAt time.Time
	IsSymlink bool
}

// AllPluginsSentinel is the ID used for the "update all" toggle in the menu.
const AllPluginsSentinel = "__all__"

// PluginDir resolves the plugin installation directory.
// Priority: TMUX_PLUGIN_MANAGER_PATH env var > XDG > ~/.tmux/plugins/
func PluginDir() string {
	if p := os.Getenv("TMUX_PLUGIN_MANAGER_PATH"); p != "" {
		return p
	}
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg != "" {
		conf := filepath.Join(xdg, "tmux", "tmux.conf")
		if _, err := os.Stat(conf); err == nil {
			return filepath.Join(xdg, "tmux", "plugins")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".tmux", "plugins")
	}
	return filepath.Join(home, ".tmux", "plugins")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run TestPluginDir -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/plugin.go internal/plugin/plugin_test.go
git commit -m "feat(plugin): add Plugin type and PluginDir resolution"
```

---

### Task 2: Scan installed plugins

**Files:**
- Modify: `internal/plugin/plugin.go`
- Modify: `internal/plugin/plugin_test.go`

- [ ] **Step 1: Write tests for Installed**

Append to `internal/plugin/plugin_test.go`:

```go
func TestInstalled_ScansDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create a fake cloned plugin with .git dir
	pluginDir := filepath.Join(dir, "tmux-sensible")
	gitDir := filepath.Join(pluginDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlinked plugin
	symlinkTarget := t.TempDir()
	symlinkPath := filepath.Join(dir, "tmux-popup-control")
	if err := os.Symlink(symlinkTarget, symlinkPath); err != nil {
		t.Fatal(err)
	}

	plugins, err := Installed(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}

	byName := map[string]Plugin{}
	for _, p := range plugins {
		byName[p.Name] = p
	}

	sensible, ok := byName["tmux-sensible"]
	if !ok {
		t.Fatal("missing tmux-sensible")
	}
	if sensible.IsSymlink {
		t.Error("tmux-sensible should not be a symlink")
	}
	if !sensible.Installed {
		t.Error("tmux-sensible should be marked installed")
	}

	popup, ok := byName["tmux-popup-control"]
	if !ok {
		t.Fatal("missing tmux-popup-control")
	}
	if !popup.IsSymlink {
		t.Error("tmux-popup-control should be a symlink")
	}
}

func TestInstalled_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	plugins, err := Installed(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestInstalled_NonexistentDir(t *testing.T) {
	plugins, err := Installed("/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run TestInstalled -v`
Expected: FAIL — `Installed` undefined

- [ ] **Step 3: Implement Installed**

Append to `internal/plugin/plugin.go`:

```go
// Installed scans pluginDir and returns a Plugin for each subdirectory found.
// Returns nil (not error) for nonexistent directories.
func Installed(pluginDir string) ([]Plugin, error) {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var plugins []Plugin
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := entry.Name()
		dir := filepath.Join(pluginDir, name)

		info, err := os.Lstat(dir)
		if err != nil {
			continue
		}
		isSymlink := info.Mode()&os.ModeSymlink != 0

		var updatedAt time.Time
		gitDir := filepath.Join(dir, ".git")
		if gi, err := os.Stat(gitDir); err == nil {
			updatedAt = gi.ModTime()
		} else {
			updatedAt = info.ModTime()
		}

		plugins = append(plugins, Plugin{
			Name:      name,
			Dir:       dir,
			Installed: true,
			UpdatedAt: updatedAt,
			IsSymlink: isSymlink,
		})
	}
	return plugins, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run TestInstalled -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/plugin.go internal/plugin/plugin_test.go
git commit -m "feat(plugin): add Installed to scan plugin directory"
```

---

### Task 3: Parse plugin config from tmux

**Files:**
- Create: `internal/plugin/config.go`
- Create: `internal/plugin/config_test.go`

The `ParseConfig` function uses gotmuxcc's `Options("", "-g")` to read global tmux options, then filters for `@plugin` keys. For testability, inject the tmux client via a package-level variable.

- [ ] **Step 1: Write tests for ParseConfig**

In `internal/plugin/config_test.go`:

```go
package plugin

import (
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run "TestParsePlugin" -v`
Expected: FAIL

- [ ] **Step 3: Implement config parsing**

In `internal/plugin/config.go`:

```go
package plugin

import (
	"path"
	"strings"
)

// optionPair is an internal representation of a tmux option key-value pair,
// decoupled from gotmuxcc types for testability.
type optionPair struct {
	Key   string
	Value string
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
	// Extract plugin name from the source URL/path.
	name := path.Base(source)
	// Strip .git suffix if present.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run "TestParsePlugin" -v`
Expected: PASS

- [ ] **Step 5: Add ParseConfig with injectable tmux client**

Add to `internal/plugin/config.go`:

```go
import (
	"fmt"
	"path/filepath"
)

// optionsFn is the function used to fetch global tmux options.
// Swapped in tests to avoid needing a live tmux server.
var optionsFn = defaultOptionsFn

// defaultOptionsFn connects to tmux via gotmuxcc and fetches global options.
// Uses client.Command("show-options", "-g") rather than client.Options("", "-g")
// because Options() unconditionally appends -t <target>, and an empty target
// produces an invalid tmux command.
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
```

Note: `newTmuxClient` needs to be a package-level var wrapping gotmuxcc. Add to `internal/plugin/config.go`:

```go
import gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"

type tmuxClient interface {
	Command(parts ...string) (string, error)
	Close()
}

var newTmuxClient = func(socketPath string) (tmuxClient, error) {
	return gotmux.NewTmux(socketPath)
}
```

**Important:** All imports across the snippets in this task must be merged into a single `import (...)` block. The final import set for `config.go` is:

```go
import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	gotmux "github.com/atomicstack/gotmuxcc/gotmuxcc"
)
```

- [ ] **Step 6: Write test for ParseConfig with stubbed tmux**

Append to `internal/plugin/config_test.go`:

```go
func TestParseConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMUX_PLUGIN_MANAGER_PATH", dir)

	// Create an installed plugin on disk
	sensibleDir := filepath.Join(dir, "tmux-sensible")
	if err := os.MkdirAll(filepath.Join(sensibleDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Stub the options function
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

	// tmux-sensible should be installed
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

	// not-installed should not be installed
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
```

- [ ] **Step 7: Run all plugin tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/plugin/config.go internal/plugin/config_test.go
git commit -m "feat(plugin): add ParseConfig to read @plugin entries from tmux"
```

---

### Task 4: Git operations — install and update

**Files:**
- Create: `internal/plugin/git.go`
- Create: `internal/plugin/install.go`
- Create: `internal/plugin/update.go`
- Create: `internal/plugin/git_test.go`

- [ ] **Step 1: Create the injectable git command runner**

In `internal/plugin/git.go`:

```go
package plugin

import (
	"os"
	"os/exec"
)

// runGitCommand executes a git command and returns its combined output.
// Swapped in tests via withStubGit.
var runGitCommand = defaultRunGitCommand

func defaultRunGitCommand(args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd.CombinedOutput()
}

// withStubGit replaces runGitCommand for the duration of a test.
func withStubGit(t interface{ Cleanup(func()) }, fn func(args ...string) ([]byte, error)) {
	orig := runGitCommand
	runGitCommand = fn
	t.Cleanup(func() { runGitCommand = orig })
}
```

- [ ] **Step 2: Write tests for Install**

In `internal/plugin/git_test.go`:

```go
package plugin

import (
	"strings"
	"testing"
)

func TestInstall_ClonesUninstalledPlugins(t *testing.T) {
	var calls [][]string
	withStubGit(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte("ok"), nil
	})

	plugins := []Plugin{
		{Name: "tmux-sensible", Source: "tmux-plugins/tmux-sensible", Dir: "/tmp/plugins/tmux-sensible", Installed: false},
		{Name: "already-here", Source: "user/already-here", Dir: "/tmp/plugins/already-here", Installed: true},
	}

	err := Install("/tmp/plugins", plugins)
	if err != nil {
		t.Fatal(err)
	}

	// Only the uninstalled plugin should have been cloned
	if len(calls) != 1 {
		t.Fatalf("got %d git calls, want 1", len(calls))
	}
	args := strings.Join(calls[0], " ")
	if !strings.Contains(args, "clone") {
		t.Errorf("expected clone command, got: %s", args)
	}
	if !strings.Contains(args, "--single-branch") {
		t.Errorf("expected --single-branch, got: %s", args)
	}
	if !strings.Contains(args, "--recursive") {
		t.Errorf("expected --recursive, got: %s", args)
	}
}

func TestInstall_WithBranch(t *testing.T) {
	var calls [][]string
	withStubGit(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte("ok"), nil
	})

	plugins := []Plugin{
		{Name: "my-plugin", Source: "user/my-plugin", Branch: "dev", Dir: "/tmp/plugins/my-plugin"},
	}

	if err := Install("/tmp/plugins", plugins); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(calls[0], " ")
	if !strings.Contains(args, "-b dev") {
		t.Errorf("expected -b dev, got: %s", args)
	}
}
```

- [ ] **Step 3: Implement Install**

In `internal/plugin/install.go`:

```go
package plugin

import (
	"fmt"
	"os"
)

// Install clones plugins that are not yet installed.
func Install(pluginDir string, plugins []Plugin) error {
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}
	var errs []error
	for _, p := range plugins {
		if p.Installed {
			continue
		}
		if err := clonePlugin(p); err != nil {
			errs = append(errs, fmt.Errorf("installing %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to install: %v", len(errs), errs)
	}
	return nil
}

func clonePlugin(p Plugin) error {
	args := []string{"clone"}
	if p.Branch != "" {
		args = append(args, "-b", p.Branch)
	}
	args = append(args, "--single-branch", "--recursive")

	// Try direct URL first
	directArgs := append(args, p.Source, p.Dir)
	if _, err := runGitCommand(directArgs...); err == nil {
		return nil
	}

	// Fall back to GitHub HTTPS shorthand
	ghURL := fmt.Sprintf("https://git::@github.com/%s", p.Source)
	fallbackArgs := append(args, ghURL, p.Dir)
	if _, err := runGitCommand(fallbackArgs...); err != nil {
		return fmt.Errorf("git clone failed for %s: %w", p.Source, err)
	}
	return nil
}
```

- [ ] **Step 4: Run install tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run TestInstall -v`
Expected: PASS

- [ ] **Step 5: Write tests for Update**

Append to `internal/plugin/git_test.go`:

```go
func TestUpdate_PullsAndUpdatesSubmodules(t *testing.T) {
	var calls [][]string
	withStubGit(t, func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte("ok"), nil
	})

	plugins := []Plugin{
		{Name: "tmux-sensible", Dir: "/tmp/plugins/tmux-sensible", Installed: true},
	}

	if err := Update("/tmp/plugins", plugins); err != nil {
		t.Fatal(err)
	}

	if len(calls) != 2 {
		t.Fatalf("got %d git calls, want 2 (pull + submodule update)", len(calls))
	}

	pullArgs := strings.Join(calls[0], " ")
	if !strings.Contains(pullArgs, "pull") {
		t.Errorf("first call should be pull, got: %s", pullArgs)
	}

	subArgs := strings.Join(calls[1], " ")
	if !strings.Contains(subArgs, "submodule") {
		t.Errorf("second call should be submodule update, got: %s", subArgs)
	}
}
```

- [ ] **Step 6: Implement Update**

In `internal/plugin/update.go`:

```go
package plugin

import "fmt"

// Update pulls the latest changes for each plugin and updates submodules.
func Update(pluginDir string, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if !p.Installed || p.Dir == "" {
			continue
		}
		if _, err := runGitCommand("-C", p.Dir, "pull"); err != nil {
			errs = append(errs, fmt.Errorf("updating %s: %w", p.Name, err))
			continue
		}
		if _, err := runGitCommand("-C", p.Dir, "submodule", "update", "--init", "--recursive"); err != nil {
			errs = append(errs, fmt.Errorf("updating submodules for %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to update: %v", len(errs), errs)
	}
	return nil
}
```

- [ ] **Step 7: Run all git tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run "TestInstall|TestUpdate" -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/plugin/git.go internal/plugin/install.go internal/plugin/update.go internal/plugin/git_test.go
git commit -m "feat(plugin): add Install and Update with injectable git runner"
```

---

### Task 5: Uninstall and Tidy

**Files:**
- Create: `internal/plugin/uninstall.go`
- Create: `internal/plugin/uninstall_test.go`

- [ ] **Step 1: Write tests**

In `internal/plugin/uninstall_test.go`:

```go
package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstall_RemovesPluginDirs(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "tmux-sensible")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := Uninstall(dir, []Plugin{{Name: "tmux-sensible", Dir: pluginDir}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Error("plugin directory should have been removed")
	}
}

func TestTidy_ReturnsUndeclaredPlugins(t *testing.T) {
	dir := t.TempDir()

	// Create installed plugins
	for _, name := range []string{"tmux-sensible", "orphaned-plugin", "tmux-popup-control"} {
		if err := os.MkdirAll(filepath.Join(dir, name, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	declared := []Plugin{
		{Name: "tmux-sensible"},
	}

	toRemove, err := Tidy(dir, declared)
	if err != nil {
		t.Fatal(err)
	}

	// Should include orphaned-plugin but NOT tmux-popup-control (self) or tmux-sensible (declared)
	if len(toRemove) != 1 {
		t.Fatalf("got %d plugins to remove, want 1", len(toRemove))
	}
	if toRemove[0].Name != "orphaned-plugin" {
		t.Errorf("expected orphaned-plugin, got %s", toRemove[0].Name)
	}
}

func TestTidy_NeverRemovesSelf(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tmux-popup-control", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	toRemove, err := Tidy(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range toRemove {
		if p.Name == "tmux-popup-control" {
			t.Error("Tidy should never include tmux-popup-control")
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run "TestUninstall|TestTidy" -v`
Expected: FAIL

- [ ] **Step 3: Implement Uninstall and Tidy**

In `internal/plugin/uninstall.go`:

```go
package plugin

import (
	"fmt"
	"os"
)

const selfName = "tmux-popup-control"

// Uninstall removes the specified plugin directories.
func Uninstall(pluginDir string, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if p.Dir == "" {
			continue
		}
		if err := os.RemoveAll(p.Dir); err != nil {
			errs = append(errs, fmt.Errorf("removing %s: %w", p.Name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d plugin(s) failed to uninstall: %v", len(errs), errs)
	}
	return nil
}

// Tidy computes the set of installed plugins not in the declared list.
// Never includes self (tmux-popup-control). Does not delete anything —
// callers use Uninstall after confirmation.
func Tidy(pluginDir string, declared []Plugin) ([]Plugin, error) {
	installed, err := Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	declaredNames := make(map[string]struct{}, len(declared))
	for _, d := range declared {
		declaredNames[d.Name] = struct{}{}
	}
	var toRemove []Plugin
	for _, p := range installed {
		if p.Name == selfName {
			continue
		}
		if _, ok := declaredNames[p.Name]; !ok {
			toRemove = append(toRemove, p)
		}
	}
	return toRemove, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run "TestUninstall|TestTidy" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/uninstall.go internal/plugin/uninstall_test.go
git commit -m "feat(plugin): add Uninstall and Tidy"
```

---

### Task 6: Source plugins

**Files:**
- Create: `internal/plugin/source.go`
- Create: `internal/plugin/source_test.go`

- [ ] **Step 1: Write tests**

In `internal/plugin/source_test.go`:

```go
package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSource_ExecutesTmuxFiles(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a *.tmux file that writes a marker
	marker := filepath.Join(dir, "marker.txt")
	script := filepath.Join(pluginDir, "test.tmux")
	content := "#!/bin/sh\necho sourced > " + marker + "\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	plugins := []Plugin{{Name: "test-plugin", Dir: pluginDir, Installed: true}}
	if err := Source(dir, plugins); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal("marker file not created — script was not executed")
	}
	if string(data) != "sourced\n" {
		t.Errorf("marker content = %q, want %q", string(data), "sourced\n")
	}
}

func TestSource_SkipsUninstalledPlugins(t *testing.T) {
	plugins := []Plugin{{Name: "ghost", Dir: "/nonexistent", Installed: false}}
	// Should not error
	if err := Source("/tmp", plugins); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSource_SkipsNonExecutableFiles(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a non-executable .tmux file
	script := filepath.Join(pluginDir, "readme.tmux")
	if err := os.WriteFile(script, []byte("not a script"), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins := []Plugin{{Name: "test-plugin", Dir: pluginDir, Installed: true}}
	// Should not error even though file is not executable — silently skip
	err := Source(dir, plugins)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run TestSource -v`
Expected: FAIL

- [ ] **Step 3: Implement Source**

In `internal/plugin/source.go`:

```go
package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Source executes each installed plugin's *.tmux files.
func Source(pluginDir string, plugins []Plugin) error {
	var errs []error
	for _, p := range plugins {
		if !p.Installed || p.Dir == "" {
			continue
		}
		pattern := filepath.Join(p.Dir, "*.tmux")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			errs = append(errs, fmt.Errorf("globbing %s: %w", pattern, err))
			continue
		}
		for _, tmuxFile := range matches {
			info, err := os.Stat(tmuxFile)
			if err != nil || info.Mode()&0o111 == 0 {
				continue // skip non-executable
			}
			cmd := exec.Command(tmuxFile)
			cmd.Stdout = nil
			cmd.Stderr = nil
			if err := cmd.Run(); err != nil {
				errs = append(errs, fmt.Errorf("sourcing %s: %w", tmuxFile, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors sourcing plugins: %v", errs)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/plugin/... -run TestSource -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/plugin/source.go internal/plugin/source_test.go
git commit -m "feat(plugin): add Source to execute plugin *.tmux files"
```

---

### Task 7: Event tracing

**Files:**
- Create: `internal/logging/events/plugins.go`

- [ ] **Step 1: Create the plugin tracer**

In `internal/logging/events/plugins.go`:

```go
package events

import "github.com/atomicstack/tmux-popup-control/internal/logging"

// PluginTracer emits structured trace events for plugin operations.
type PluginTracer struct{}

// Plugins is the singleton tracer for plugin events.
var Plugins = PluginTracer{}

func (PluginTracer) Install(name string) {
	logging.Trace("plugins.install", map[string]interface{}{"name": name})
}

func (PluginTracer) Update(name string) {
	logging.Trace("plugins.update", map[string]interface{}{"name": name})
}

func (PluginTracer) Uninstall(name string) {
	logging.Trace("plugins.uninstall", map[string]interface{}{"name": name})
}

func (PluginTracer) Tidy(name string) {
	logging.Trace("plugins.tidy", map[string]interface{}{"name": name})
}

func (PluginTracer) Source(name string) {
	logging.Trace("plugins.source", map[string]interface{}{"name": name})
}

func (PluginTracer) InitPlugins(count int) {
	logging.Trace("plugins.init", map[string]interface{}{"count": count})
}
```

- [ ] **Step 2: Run full test suite to verify no breakage**

Run: `make test`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/logging/events/plugins.go
git commit -m "feat(plugin): add plugin event tracer"
```

---

## Chunk 2: Menu integration

### Task 8: Plugin menu loaders

**Files:**
- Create: `internal/menu/plugins.go`
- Modify: `internal/menu/menu.go`

- [ ] **Step 1: Add plugins to RootItems and maps**

In `internal/menu/menu.go`, add `{ID: "plugins", Label: "plugins"}` to `RootItems()`, add `"plugins": loadPluginsMenu` to `CategoryLoaders()`, add the action loaders to `ActionLoaders()`, and add the action handlers to `ActionHandlers()`.

Add to `RootItems()`:
```go
{ID: "plugins", Label: "plugins"},
```

Add to `CategoryLoaders()`:
```go
"plugins": loadPluginsMenu,
```

Add to `ActionLoaders()`:
```go
"plugins:list":      loadPluginsListMenu,
"plugins:update":    loadPluginsUpdateMenu,
"plugins:uninstall": loadPluginsUninstallMenu,
```

Add to `ActionHandlers()`:
```go
"plugins:install":   PluginsInstallAction,
"plugins:update":    PluginsUpdateAction,
"plugins:uninstall": PluginsUninstallAction,
"plugins:tidy":      PluginsTidyAction,
```

- [ ] **Step 2: Create plugins.go with loaders**

In `internal/menu/plugins.go`:

```go
package menu

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/format/table"
	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

func loadPluginsMenu(Context) ([]Item, error) {
	items := []string{"list", "install", "update", "uninstall", "tidy"}
	return menuItemsFromIDs(items), nil
}

func loadPluginsListMenu(_ Context) ([]Item, error) {
	pluginDir := plugin.PluginDir()
	installed, err := plugin.Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	if len(installed) == 0 {
		return []Item{{ID: "__empty__", Label: "No plugins installed"}}, nil
	}
	rows := make([][]string, 0, len(installed))
	ids := make([]string, 0, len(installed))
	for _, p := range installed {
		updated := "unknown"
		if !p.UpdatedAt.IsZero() {
			updated = p.UpdatedAt.Format(time.DateOnly)
		}
		symlink := ""
		if p.IsSymlink {
			symlink = "(symlink)"
		}
		rows = append(rows, []string{p.Name, updated, symlink})
		ids = append(ids, p.Name)
	}
	aligned := table.Format(rows, []table.Alignment{table.AlignLeft, table.AlignRight, table.AlignLeft})
	items := make([]Item, len(aligned))
	for i, label := range aligned {
		items[i] = Item{ID: ids[i], Label: label}
	}
	return items, nil
}

func loadPluginsUpdateMenu(_ Context) ([]Item, error) {
	pluginDir := plugin.PluginDir()
	installed, err := plugin.Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(installed)+1)
	items = append(items, Item{ID: plugin.AllPluginsSentinel, Label: "all"})
	for _, p := range installed {
		items = append(items, Item{ID: p.Name, Label: p.Name})
	}
	return items, nil
}

func loadPluginsUninstallMenu(_ Context) ([]Item, error) {
	pluginDir := plugin.PluginDir()
	installed, err := plugin.Installed(pluginDir)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(installed))
	for _, p := range installed {
		items = append(items, Item{ID: p.Name, Label: p.Name})
	}
	return items, nil
}
```

- [ ] **Step 3: Stub out the action functions (will be completed in Task 9)**

Append to `internal/menu/plugins.go`:

```go
// PluginReloadPrompt is sent after a plugin operation succeeds.
// The UI handles it by asking "Reload plugins? [y/n]".
type PluginReloadPrompt struct {
	Plugins   []plugin.Plugin
	PluginDir string
	Summary   string
}

// PluginConfirmPrompt is sent before destructive plugin operations.
// The UI handles it by confirming each plugin individually.
type PluginConfirmPrompt struct {
	Plugins   []plugin.Plugin
	PluginDir string
	Operation string // "uninstall" or "tidy"
}

func PluginsInstallAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		events.Plugins.Install("all")
		pluginDir := plugin.PluginDir()
		plugins, err := plugin.ParseConfig(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: err}
		}
		if err := plugin.Install(pluginDir, plugins); err != nil {
			return ActionResult{Err: err}
		}
		// Count what was installed
		var installed int
		for _, p := range plugins {
			if !p.Installed {
				installed++
			}
		}
		if installed == 0 {
			return ActionResult{Info: "All plugins already installed"}
		}
		return PluginReloadPrompt{
			Plugins:   plugins,
			PluginDir: pluginDir,
			Summary:   fmt.Sprintf("Installed %d plugin(s)", installed),
		}
	}
}

func PluginsUpdateAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		installed, err := plugin.Installed(pluginDir)
		if err != nil {
			return ActionResult{Err: err}
		}

		// Determine which plugins to update
		selected := parseMultiSelectIDs(item.ID)
		updateAll := false
		for _, id := range selected {
			if id == plugin.AllPluginsSentinel {
				updateAll = true
				break
			}
		}

		var toUpdate []plugin.Plugin
		if updateAll {
			toUpdate = installed
		} else {
			nameSet := make(map[string]struct{}, len(selected))
			for _, id := range selected {
				nameSet[id] = struct{}{}
			}
			for _, p := range installed {
				if _, ok := nameSet[p.Name]; ok {
					toUpdate = append(toUpdate, p)
				}
			}
		}

		for _, p := range toUpdate {
			events.Plugins.Update(p.Name)
		}
		if err := plugin.Update(pluginDir, toUpdate); err != nil {
			return ActionResult{Err: err}
		}
		return PluginReloadPrompt{
			Plugins:   toUpdate,
			PluginDir: pluginDir,
			Summary:   fmt.Sprintf("Updated %d plugin(s)", len(toUpdate)),
		}
	}
}

func PluginsUninstallAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		installed, err := plugin.Installed(pluginDir)
		if err != nil {
			return ActionResult{Err: err}
		}

		selected := parseMultiSelectIDs(item.ID)
		nameSet := make(map[string]struct{}, len(selected))
		for _, id := range selected {
			nameSet[id] = struct{}{}
		}
		var toRemove []plugin.Plugin
		for _, p := range installed {
			if _, ok := nameSet[p.Name]; ok {
				toRemove = append(toRemove, p)
			}
		}
		if len(toRemove) == 0 {
			return ActionResult{Info: "No plugins selected"}
		}
		return PluginConfirmPrompt{
			Plugins:   toRemove,
			PluginDir: pluginDir,
			Operation: "uninstall",
		}
	}
}

func PluginsTidyAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		pluginDir := plugin.PluginDir()
		declared, err := plugin.ParseConfig(ctx.SocketPath)
		if err != nil {
			return ActionResult{Err: err}
		}
		toRemove, err := plugin.Tidy(pluginDir, declared)
		if err != nil {
			return ActionResult{Err: err}
		}
		if len(toRemove) == 0 {
			return ActionResult{Info: "No undeclared plugins found"}
		}
		for _, p := range toRemove {
			events.Plugins.Tidy(p.Name)
		}
		return PluginConfirmPrompt{
			Plugins:   toRemove,
			PluginDir: pluginDir,
			Operation: "tidy",
		}
	}
}

// parseMultiSelectIDs splits a newline-joined ID string from multi-select.
func parseMultiSelectIDs(id string) []string {
	if id == "" {
		return nil
	}
	var ids []string
	for _, s := range strings.Split(id, "\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			ids = append(ids, s)
		}
	}
	return ids
}
```

Add `"strings"` to the import block.

- [ ] **Step 4: Register multi-select in registry.go**

In `internal/menu/registry.go`, add to the `markMultiSelect` slice:

```go
"plugins:update",
"plugins:uninstall",
```

- [ ] **Step 5: Run the full test suite**

Run: `make test`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/menu/plugins.go internal/menu/menu.go internal/menu/registry.go
git commit -m "feat(plugin): add plugins menu with loaders and actions"
```

---

### Task 9: Plugin menu tests

**Files:**
- Create: `internal/menu/plugins_test.go`

- [ ] **Step 1: Write loader tests**

In `internal/menu/plugins_test.go`:

```go
package menu

import (
	"strings"
	"testing"
)

func TestLoadPluginsMenu(t *testing.T) {
	items, err := loadPluginsMenu(Context{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"list", "install", "update", "uninstall", "tidy"}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, item := range items {
		if item.ID != want[i] {
			t.Errorf("items[%d].ID = %q, want %q", i, item.ID, want[i])
		}
	}
}

func TestParseMultiSelectIDs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"foo", []string{"foo"}},
		{"foo\nbar\nbaz", []string{"foo", "bar", "baz"}},
		{"foo\n\nbar\n", []string{"foo", "bar"}},
	}
	for _, tt := range tests {
		got := parseMultiSelectIDs(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseMultiSelectIDs(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseMultiSelectIDs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}
```

- [ ] **Step 2: Run tests**

Run: `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache GOFLAGS=-modcacherw GOPROXY=off go test ./internal/menu/... -run "TestLoadPluginsMenu|TestParseMultiSelectIDs" -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/menu/plugins_test.go
git commit -m "test(plugin): add plugin menu loader tests"
```

---

## Chunk 3: UI modes and init-plugins subcommand

### Task 10: Plugin confirmation and reload UI modes

**Files:**
- Modify: `internal/ui/model.go`
- Create: `internal/ui/plugin_confirm.go`

This task adds `ModePluginConfirm` and `ModePluginReload` to the Mode enum, registers the new message types, and implements the handlers.

- [ ] **Step 1: Add modes to model.go**

In `internal/ui/model.go`, extend the Mode enum:

```go
const (
	ModeMenu Mode = iota
	ModePaneForm
	ModeWindowForm
	ModeSessionForm
	ModePluginConfirm
	ModePluginReload
)
```

Add fields to the `Model` struct:

```go
pluginConfirmState *pluginConfirmState
pluginReloadState  *pluginReloadState
```

- [ ] **Step 2: Register new message handlers in registerHandlers**

Add to `registerHandlers()`:

```go
reflect.TypeOf(menu.PluginConfirmPrompt{}): m.handlePluginConfirmPromptMsg,
reflect.TypeOf(menu.PluginReloadPrompt{}):  m.handlePluginReloadPromptMsg,
```

- [ ] **Step 3: Add cases to handleActiveForm**

In `handleActiveForm()`, add before `default`:

```go
case ModePluginConfirm:
	return m.handlePluginConfirm(msg)
case ModePluginReload:
	return m.handlePluginReload(msg)
```

- [ ] **Step 4: Create plugin_confirm.go**

In `internal/ui/plugin_confirm.go`:

```go
package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/atomicstack/tmux-popup-control/internal/logging/events"
	"github.com/atomicstack/tmux-popup-control/internal/menu"
	"github.com/atomicstack/tmux-popup-control/internal/plugin"
)

type pluginConfirmState struct {
	pending   []plugin.Plugin // plugins remaining to confirm
	confirmed []plugin.Plugin // plugins confirmed for removal
	current   plugin.Plugin   // currently being confirmed
	pluginDir string
	operation string // "uninstall" or "tidy"
}

type pluginReloadState struct {
	plugins   []plugin.Plugin
	pluginDir string
	summary   string
}

func (m *Model) handlePluginConfirmPromptMsg(msg tea.Msg) tea.Cmd {
	prompt := msg.(menu.PluginConfirmPrompt)
	if len(prompt.Plugins) == 0 {
		return func() tea.Msg {
			return menu.ActionResult{Info: "Nothing to remove"}
		}
	}
	m.pluginConfirmState = &pluginConfirmState{
		pending:   prompt.Plugins[1:],
		current:   prompt.Plugins[0],
		pluginDir: prompt.PluginDir,
		operation: prompt.Operation,
	}
	m.mode = ModePluginConfirm
	m.loading = false
	return nil
}

func (m *Model) handlePluginConfirm(msg tea.Msg) (bool, tea.Cmd) {
	if m.pluginConfirmState == nil {
		return false, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}

	s := m.pluginConfirmState
	switch keyMsg.String() {
	case "y", "Y":
		s.confirmed = append(s.confirmed, s.current)
		return true, m.advancePluginConfirm()
	case "n", "N":
		return true, m.advancePluginConfirm()
	case "esc":
		m.pluginConfirmState = nil
		m.mode = ModeMenu
		return true, nil
	}
	return true, nil
}

func (m *Model) advancePluginConfirm() tea.Cmd {
	s := m.pluginConfirmState
	if len(s.pending) > 0 {
		s.current = s.pending[0]
		s.pending = s.pending[1:]
		return nil
	}

	// All confirmed — execute removal
	confirmed := s.confirmed
	pluginDir := s.pluginDir
	operation := s.operation
	m.pluginConfirmState = nil
	m.mode = ModeMenu

	if len(confirmed) == 0 {
		return func() tea.Msg {
			return menu.ActionResult{Info: "No plugins removed"}
		}
	}

	m.loading = true
	m.pendingLabel = fmt.Sprintf("removing %d plugin(s)", len(confirmed))
	return func() tea.Msg {
		for _, p := range confirmed {
			events.Plugins.Uninstall(p.Name)
		}
		if err := plugin.Uninstall(pluginDir, confirmed); err != nil {
			return menu.ActionResult{Err: err}
		}
		action := "Uninstalled"
		if operation == "tidy" {
			action = "Tidied"
		}
		return menu.PluginReloadPrompt{
			Plugins:   confirmed,
			PluginDir: pluginDir,
			Summary:   fmt.Sprintf("%s %d plugin(s)", action, len(confirmed)),
		}
	}
}

func (m *Model) handlePluginReloadPromptMsg(msg tea.Msg) tea.Cmd {
	prompt := msg.(menu.PluginReloadPrompt)
	m.pluginReloadState = &pluginReloadState{
		plugins:   prompt.Plugins,
		pluginDir: prompt.PluginDir,
		summary:   prompt.Summary,
	}
	m.mode = ModePluginReload
	m.loading = false
	return nil
}

func (m *Model) handlePluginReload(msg tea.Msg) (bool, tea.Cmd) {
	if m.pluginReloadState == nil {
		return false, nil
	}
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false, nil
	}

	s := m.pluginReloadState
	switch keyMsg.String() {
	case "y", "Y":
		plugins := s.plugins
		pluginDir := s.pluginDir
		summary := s.summary
		m.pluginReloadState = nil
		m.mode = ModeMenu
		m.loading = true
		m.pendingLabel = "reloading plugins"
		return true, func() tea.Msg {
			for _, p := range plugins {
				events.Plugins.Source(p.Name)
			}
			if err := plugin.Source(pluginDir, plugins); err != nil {
				return menu.ActionResult{Err: fmt.Errorf("reload failed: %w", err)}
			}
			return menu.ActionResult{Info: summary + " (reloaded)"}
		}
	case "n", "N":
		summary := s.summary
		m.pluginReloadState = nil
		m.mode = ModeMenu
		return true, func() tea.Msg {
			return menu.ActionResult{Info: summary}
		}
	case "esc":
		summary := s.summary
		m.pluginReloadState = nil
		m.mode = ModeMenu
		return true, func() tea.Msg {
			return menu.ActionResult{Info: summary}
		}
	}
	return true, nil
}

// pluginConfirmView renders the confirmation prompt.
func (m *Model) pluginConfirmView() string {
	if m.pluginConfirmState == nil {
		return ""
	}
	s := m.pluginConfirmState
	return fmt.Sprintf(
		"Are you sure you want to remove the plugin named %s in the directory %s? [y/n]",
		s.current.Name,
		s.current.Dir,
	)
}

// pluginReloadView renders the reload prompt.
func (m *Model) pluginReloadView() string {
	if m.pluginReloadState == nil {
		return ""
	}
	return fmt.Sprintf(
		"%s. Reload plugins? [y/n]",
		m.pluginReloadState.summary,
	)
}
```

- [ ] **Step 5: Wire confirmation/reload views into View()**

In `internal/ui/view.go`, the `View()` method has a `switch m.mode` block (around line 78) that handles `ModePaneForm`, `ModeWindowForm`, and `ModeSessionForm`. Add two new `case` arms to this existing switch, using `m.wrapView()` to match the existing pattern:

```go
case ModePluginConfirm:
	content = m.pluginConfirmView()
	return m.wrapView(content)
case ModePluginReload:
	content = m.pluginReloadView()
	return m.wrapView(content)
```

Do NOT use bare `if` statements outside the switch — they would be unreachable and would not compile correctly with `View()`'s return type.

- [ ] **Step 6: Run the full test suite**

Run: `make test`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/ui/model.go internal/ui/plugin_confirm.go internal/ui/view.go
git commit -m "feat(plugin): add ModePluginConfirm and ModePluginReload UI modes"
```

---

### Task 11: Checkbox styling for multi-select

**Files:**
- Modify: `internal/ui/view.go` (or `internal/theme/theme.go`)

The existing multi-select uses the `SelectedItem` / `SelectedItemIndicator` styles from `internal/theme/theme.go`. Check how multi-select items are currently rendered in `view.go` (the indicator prefix logic). The update and uninstall menus should render checkboxes using these existing styles. The "all" item in the update menu uses `AllPluginsSentinel` as its ID — the view can check for this to apply a distinct style.

- [ ] **Step 1: Add checkbox styles to theme**

In `internal/theme/theme.go`, add to the `Styles` struct:

```go
Checkbox         *lipgloss.Style
CheckboxChecked  *lipgloss.Style
CheckboxAll      *lipgloss.Style
```

Add defaults:

```go
Checkbox: ptr(
	lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
),
CheckboxChecked: ptr(
	lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Bold(true),
),
CheckboxAll: ptr(
	lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true),
),
```

- [ ] **Step 2: Update the item rendering in view.go**

Locate the section in `view.go` where multi-select items render their indicator prefix. Add checkbox rendering for multi-select levels: checked items get `☑` styled with `CheckboxChecked`, unchecked get `☐` styled with `Checkbox`. The "all" sentinel item gets styled with `CheckboxAll`.

The exact code depends on the current rendering loop structure. The key logic:

```go
if level.MultiSelect {
	if item.ID == plugin.AllPluginsSentinel {
		prefix = styles.CheckboxAll.Render("☑ ") // or "☐ " based on selection
	} else if level.IsSelected(item.ID) {
		prefix = styles.CheckboxChecked.Render("☑ ")
	} else {
		prefix = styles.Checkbox.Render("☐ ")
	}
}
```

Import `"github.com/atomicstack/tmux-popup-control/internal/plugin"` in view.go for `AllPluginsSentinel`.

- [ ] **Step 3: Run the full test suite**

Run: `make test`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/theme/theme.go internal/ui/view.go
git commit -m "feat(plugin): add coloured checkbox rendering for multi-select"
```

---

### Task 12: init-plugins CLI subcommand

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Add init-plugins dispatch**

In `main.go`, after `config.MustLoad()` and logging setup, add the subcommand check. Place it after `logging.Configure` so logging is available, but before `app.Run`:

```go
if len(os.Args) > 1 && os.Args[1] == "init-plugins" {
	if err := runInitPlugins(runtimeCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
```

Add the `runInitPlugins` function. It returns an error rather than calling `os.Exit` internally — this keeps it testable and follows the same pattern as the `--version` check where `os.Exit` lives in the caller:

```go
func runInitPlugins(cfg config.Config) error {
	socketPath, err := tmux.ResolveSocketPath(cfg.App.SocketPath)
	if err != nil {
		return fmt.Errorf("resolving socket: %w", err)
	}

	plugins, err := plugin.ParseConfig(socketPath)
	if err != nil {
		return fmt.Errorf("reading plugin config: %w", err)
	}

	pluginDir := plugin.PluginDir()
	events.Plugins.InitPlugins(len(plugins))

	if err := plugin.Source(pluginDir, plugins); err != nil {
		return fmt.Errorf("sourcing plugins: %w", err)
	}
	return nil
}
```

Add imports for `"github.com/atomicstack/tmux-popup-control/internal/plugin"` and `"github.com/atomicstack/tmux-popup-control/internal/tmux"` (tmux is likely already imported — the `events` import may need adding).

- [ ] **Step 2: Build to verify compilation**

Run: `make build`
Expected: Compiles successfully

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat(plugin): add init-plugins CLI subcommand"
```

---

### Task 13: Final integration — full test suite

**Files:** None new — this is a verification task.

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: All tests PASS

- [ ] **Step 2: Run build**

Run: `make build`
Expected: Binary compiles

- [ ] **Step 3: Verify init-plugins runs**

Run: `./tmux-popup-control init-plugins 2>&1; echo "exit: $?"`
Expected: Either succeeds (exit 0) or fails gracefully with "Error resolving socket" if not inside tmux (exit 1). Should not panic.

- [ ] **Step 4: Final commit if any fixups were needed**

```bash
git add -u
git commit -m "fix: address issues from integration testing"
```

(Skip if no fixups needed.)
