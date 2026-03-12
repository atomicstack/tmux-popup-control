# Plugin Management for tmux-popup-control

**Date:** 2026-03-12
**Status:** Approved
**Goal:** Replace tpm (tmux plugin manager) entirely, providing plugin management via the popup menu and a CLI subcommand for plugin sourcing at tmux startup.

## Background

tpm is a shell-script-based tmux plugin manager. It reads `set -g @plugin '...'` declarations from tmux.conf, clones plugins into `~/.tmux/plugins/`, and sources their `*.tmux` files at startup. It provides key bindings for install, update, and clean operations.

tmux-popup-control already provides an interactive popup menu for tmux management. Adding plugin management as a new menu category lets users manage plugins through the same interface and eliminates the dependency on tpm.

## Plugin declaration format

Plugins are declared in `~/.tmux.conf` (or `$XDG_CONFIG_HOME/tmux/tmux.conf`) using the existing tpm format:

```bash
set -g @plugin 'tmux-plugins/tmux-sensible'
set -g @plugin 'tmux-plugins/tmux-resurrect'
set -g @plugin 'github_username/plugin_name#branch'
set -g @plugin 'git@github.com:user/plugin'
```

This ensures zero-friction migration from tpm — users only need to swap the `run` line in their tmux.conf.

## Architecture

### New package: `internal/plugin/`

A self-contained package with no dependency on Bubble Tea or the menu system. All plugin operations live here. Git operations use `os/exec` to call `git` directly (not gotmuxcc — this is filesystem work).

#### Types

```go
// Plugin represents a declared or installed tmux plugin.
type Plugin struct {
    Name      string    // e.g. "tmux-resurrect"
    Source    string    // e.g. "tmux-plugins/tmux-resurrect"
    Branch    string    // e.g. "main" (empty = default branch)
    Dir       string    // absolute path to install directory
    Installed bool
    UpdatedAt time.Time // mtime of .git directory
    IsSymlink bool      // true for local dev symlinks
}
```

#### Functions

| Function | Purpose |
|---|---|
| `PluginDir() string` | Resolve plugin directory: checks `os.Getenv("TMUX_PLUGIN_MANAGER_PATH")` (matching tpm's behavior — only works if the variable was exported into the shell environment before tmux started), then XDG (`$XDG_CONFIG_HOME/tmux/plugins/`), then falls back to `~/.tmux/plugins/` |
| `ParseConfig(socketPath string) ([]Plugin, error)` | Read `@plugin` entries from tmux via gotmuxcc's `client.Command("show-options", "-g")` (because `client.Options()` unconditionally appends `-t <target>` which breaks for global options with no target). Parse the raw output lines as `key value` pairs, filter for `@plugin` keys, parse `user/repo#branch` format from each value, resolve each to a Plugin struct with install status. The client must be `Close()`d after use to avoid leaking `tmux -C` processes |
| `Installed(pluginDir string) ([]Plugin, error)` | Scan the plugin directory for what is on disk. Detect symlinks via `os.Lstat`. Read `.git` mtime for `UpdatedAt` |
| `Install(pluginDir string, plugins []Plugin) error` | `git clone --single-branch --recursive` for each uninstalled plugin. Uses `GIT_TERMINAL_PROMPT=0`. Tries direct URL first, falls back to `https://git::@github.com/user/repo` for shorthand sources |
| `Update(pluginDir string, plugins []Plugin) error` | `git pull` + `git submodule update --init --recursive` for each plugin. Uses `GIT_TERMINAL_PROMPT=0` |
| `Uninstall(pluginDir string, plugins []Plugin) error` | `os.RemoveAll` for each specified plugin directory |
| `Tidy(pluginDir string, declared []Plugin) (toRemove []Plugin, err error)` | Compute the set of installed plugins not in the declared list. Never includes self (tmux-popup-control). Returns the list of plugins eligible for removal — does **not** delete them (callers use `Uninstall` after confirmation) |
| `Source(pluginDir string, plugins []Plugin) error` | Execute each plugin's `*.tmux` files (glob `pluginDir/pluginName/*.tmux`, run each as a subprocess). Silent on success, returns errors |

#### Testability

A package-level variable `var runGitCommand func(name string, args ...string) ([]byte, error)` follows the same pattern as `internal/tmux/`'s `runExecCommand`. Tests swap this with a stub via a `withStubGit` helper to avoid real git operations.

### Menu integration: `internal/menu/plugins.go`

A new "plugins" entry in the root menu with five submenu items.

#### Root menu

Add to `RootItems()` in `menu.go`:
```go
{ID: "plugins", Label: "plugins"}
```

Add to `CategoryLoaders()`:
```go
"plugins": loadPluginsMenu
```

#### Submenu structure

```
plugins
├── list         → read-only table of installed plugins (name + last updated)
├── install      → installs all declared-but-not-cloned plugins
├── update       → multi-select list with "all" toggle + individual plugins
├── uninstall    → multi-select list of installed plugins
└── tidy         → removes plugins not declared in tmux.conf
```

#### Loaders

| Loader | Registered in | Behavior |
|---|---|---|
| `loadPluginsMenu` | `CategoryLoaders` | Returns 5 static items: list, install, update, uninstall, tidy |
| `loadPluginsListMenu` | `ActionLoaders["plugins:list"]` | Calls `plugin.Installed()`, returns formatted table with plugin name + last-updated time. Each item uses the plugin `Name` as its ID (for fuzzy filtering). Items are informational only — no action is registered for `plugins:list`, so pressing Enter on a list item is a no-op |
| `loadPluginsUpdateMenu` | `ActionLoaders["plugins:update"]` | Calls `plugin.Installed()`, returns an "all" item (ID: `__all__`, defined as `const AllPluginsSentinel = "__all__"`) at top + one item per plugin. Multi-select enabled |
| `loadPluginsUninstallMenu` | `ActionLoaders["plugins:uninstall"]` | Calls `plugin.Installed()`, returns one item per plugin. Multi-select enabled |

`install` and `tidy` have no loaders — they are direct actions.

**Loader/Action coexistence note:** For `plugins:update` and `plugins:uninstall`, the same node has both a Loader and an Action. This follows the existing pattern used by `window:kill`, `pane:kill`, etc. When the user navigates into the node, the Loader fires and populates the submenu (the plugin list). When the user presses Enter on items *within* that loaded submenu, the node's Action fires with the selected item(s). The Action does not fire on initial navigation — only after the list is loaded and an item is selected.

#### Actions

| Action | Registered in | Behavior |
|---|---|---|
| `PluginsInstallAction` | `ActionHandlers["plugins:install"]` | Calls `plugin.ParseConfig()` + `plugin.Install()` for all uninstalled-but-declared plugins. Returns `PluginReloadPrompt` on success |
| `PluginsUpdateAction` | `ActionHandlers["plugins:update"]` | Calls `plugin.Update()` for selected plugins. If the `AllPluginsSentinel` ID is among the selected items, updates all installed plugins. Returns `PluginReloadPrompt` on success |
| `PluginsUninstallAction` | `ActionHandlers["plugins:uninstall"]` | For each selected plugin, shows confirmation prompt: "Are you sure you want to remove the plugin named {name} in the directory {dir}?" — proceeds only on explicit confirmation. Then calls `plugin.Uninstall()`. Returns `PluginReloadPrompt` on success |
| `PluginsTidyAction` | `ActionHandlers["plugins:tidy"]` | Computes the removal set by calling `plugin.Installed()` and `plugin.ParseConfig()` to find the diff (installed but not declared). Returns a `PluginConfirmPrompt` with this set. After per-plugin confirmation, calls `plugin.Uninstall()` on the confirmed subset (not `plugin.Tidy()` directly — `Tidy` is only used by `init-plugins` or non-interactive contexts). Returns `PluginReloadPrompt` on success |

#### Deletion confirmation

Both `uninstall` and `tidy` must confirm each plugin removal individually before proceeding. The confirmation message format:

> Are you sure you want to remove the plugin named **{name}** in the directory **{dir}**?

**Implementation:** Add a new `ModePluginConfirm` value to the `Mode` enum in `internal/ui/model.go`. The model holds a `PluginConfirmState` struct containing:
- The queue of plugins pending confirmation (remaining after the current one)
- The current plugin being confirmed
- The plugins already confirmed for removal
- The originating operation (`uninstall` or `tidy`)
- The plugin directory path

When a `PluginConfirmPrompt` message arrives, the UI enters `ModePluginConfirm` and displays the confirmation for the first plugin. On "y", the plugin is added to the confirmed list and the next plugin is shown. On "n", the plugin is skipped. After all plugins have been prompted, the confirmed plugins are passed to `plugin.Uninstall()` (or the tidy equivalent), and the result flows into the reload prompt.

A `handlePluginConfirm` method in `model.go` handles key input in this mode. The `handleActiveForm` switch statement must be extended with a `case ModePluginConfirm` arm that delegates to `handlePluginConfirm`, following the same dispatch pattern as the existing `ModePaneForm`/`ModeWindowForm`/`ModeSessionForm` cases.

#### Multi-select and checkboxes

The update and uninstall menus use the existing multi-select pattern (tab to mark). Register in `BuildRegistry()`:

```go
markMultiSelect := []string{
    // ... existing entries
    "plugins:update",
    "plugins:uninstall",
}
```

Checkboxes should be styled with color using `internal/theme/` lipgloss styles:
- Checked items rendered with a highlight/accent color
- The "all" toggle item visually distinguished from individual plugin items

#### Reload prompt

After install, update, uninstall, or tidy operations succeed, the action returns a `PluginReloadPrompt` message instead of a plain `ActionResult`. The UI enters `ModePluginReload` (a new `Mode` enum value) and displays:

> Reload plugins? [y/n]

A `handlePluginReload` method handles key input in this mode. The `handleActiveForm` switch statement must be extended with a `case ModePluginReload` arm that delegates to `handlePluginReload`. On "y", it fires a `tea.Cmd` that calls `plugin.Source()` for the affected plugins, then returns `ActionResult` with the success message. On "n", it returns `ActionResult` directly and exits the mode.

The `PluginReloadPrompt` message carries the list of affected plugins, the plugin directory, and a summary string describing what was done (e.g. "Updated 3 plugins").

#### Context

Plugin loaders do not need data injected into `menu.Context`. Unlike sessions/windows/panes (which come from the backend poller), plugin data is loaded on-demand from the filesystem. Loaders only need `SocketPath` (already in Context) to resolve the plugin directory.

### CLI subcommand: `init-plugins`

Replaces tpm's startup role. Users put this in `~/.tmux.conf`:

```bash
run '~/.local/bin/tmux-popup-control init-plugins'
```

#### Behavior

1. Resolve socket path via `tmux.ResolveSocketPath("")` — this uses the same resolution chain as the rest of the app (`TMUX_POPUP_CONTROL_SOCKET` → `TMUX_POPUP_SOCKET` → `$TMUX`)
2. Call `plugin.ParseConfig()` to read `@plugin` declarations
3. Call `plugin.PluginDir()` to resolve the install directory
4. Call `plugin.Source()` to execute each installed plugin's `*.tmux` files
5. Exit 0 silently on success, print to stderr and exit 1 on error

Does **not** auto-install missing plugins — that is an explicit user action via the popup menu. This keeps startup fast and predictable.

#### Implementation in `main.go`

The `init-plugins` check is placed **after** `config.MustLoad()` so that the `-socket` flag and `TMUX_POPUP_CONTROL_SOCKET` env var are parsed and available, and logging is configured. The socket path comes from the parsed config's `SocketPath` field, falling back to `tmux.ResolveSocketPath("")` if empty. Errors from `ResolveSocketPath` must be checked and cause a non-zero exit.

```go
cfg := config.MustLoad()
if len(os.Args) > 1 && os.Args[1] == "init-plugins" {
    socketPath, err := tmux.ResolveSocketPath(cfg.App.SocketPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
    // parse config, source plugins, exit
}
```

### Event tracing

Add `internal/logging/events/plugins.go` with trace points:

- `events.Plugins.Install(name)`
- `events.Plugins.Update(name)`
- `events.Plugins.Uninstall(name)`
- `events.Plugins.Tidy(name)`
- `events.Plugins.Source(name)`
- `events.Plugins.InitPlugins(count)`

Follows the existing pattern in `internal/logging/events/`.

## Testing

### `internal/plugin/` unit tests

| Test | What it verifies |
|---|---|
| `TestParseConfig` | Stub `tmux show-options` output, verify parsed Plugin structs (name, source, branch) |
| `TestPluginDir` | Resolution priority: env var → XDG → default |
| `TestInstall` | Stub `runGitCommand`, verify correct args (`--single-branch`, `--recursive`, `GIT_TERMINAL_PROMPT=0`), verify fallback URL |
| `TestUpdate` | Stub `runGitCommand`, verify `git pull` + `git submodule update` args |
| `TestUninstall` | Temp directory with fake plugin dirs, verify removal |
| `TestTidy` | Temp directory with declared + undeclared plugins, verify only undeclared removed, self preserved |
| `TestSource` | Temp directory with `*.tmux` files, verify they are executed |
| `TestInstalled` | Temp directory with mix of cloned repos, symlinks, non-plugin dirs |

### `internal/menu/plugins_test.go`

- Loaders return correct items (list with formatted table, update with "all" item, uninstall list)
- Actions dispatch correct `plugin.*` calls via stubs

### Integration tests (optional)

In `internal/plugin/integration_test.go`. Skipped when `git` is unavailable. Clone a tiny test repo into a temp dir, verify install/update/source lifecycle end-to-end.

## Files to create or modify

### New files

| File | Purpose |
|---|---|
| `internal/plugin/plugin.go` | Core types and plugin directory resolution |
| `internal/plugin/config.go` | `ParseConfig` — read `@plugin` entries from tmux |
| `internal/plugin/install.go` | `Install` — git clone logic |
| `internal/plugin/update.go` | `Update` — git pull logic |
| `internal/plugin/uninstall.go` | `Uninstall` + `Tidy` |
| `internal/plugin/source.go` | `Source` — execute `*.tmux` files |
| `internal/plugin/git.go` | `runGitCommand` var and helpers |
| `internal/plugin/plugin_test.go` | Unit tests |
| `internal/menu/plugins.go` | Menu loaders and actions |
| `internal/menu/plugins_test.go` | Menu tests |
| `internal/logging/events/plugins.go` | Trace events |

### Modified files

| File | Change |
|---|---|
| `main.go` | Add `init-plugins` subcommand dispatch before Bubble Tea startup |
| `internal/menu/menu.go` | Add "plugins" to `RootItems()`, `CategoryLoaders()`, `ActionLoaders()`, `ActionHandlers()` |
| `internal/menu/registry.go` | Add `plugins:update` and `plugins:uninstall` to `markMultiSelect` |
| `internal/ui/model.go` | Add `ModePluginConfirm` and `ModePluginReload` to Mode enum, add `PluginConfirmState` struct, register handlers for `PluginConfirmPrompt` and `PluginReloadPrompt` message types, add `handlePluginConfirm` and `handlePluginReload` methods |

## Migration from tpm

Users migrating from tpm:

1. Keep all `set -g @plugin '...'` lines unchanged
2. Replace `run '~/.tmux/plugins/tpm/tpm'` with `run '/path/to/tmux-popup-control init-plugins'`
3. Optionally remove tpm from `~/.tmux/plugins/` and its `@plugin` declaration
