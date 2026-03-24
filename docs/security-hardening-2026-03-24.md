# security hardening — 2026-03-24

synthesised from two independent security reviews:
- `security-review-2026-03-24` (primary)
- `security-analysis-2026-03-24`

## scope

full codebase review covering:
- command injection and OS command execution
- input validation and data flow
- file system safety, permissions, and information disclosure
- concurrency safety, race conditions, and denial of service
- tmux plugin trust boundaries

## summary

| #  | severity | status | location | issue |
|----|----------|--------|----------|-------|
| 1  | high     | fixed  | `resurrect/restore.go`, `tmux/restore.go` | shell injection via unescaped `contentPath` and `defaultCmd` in `paneStartupCommand`; unsafe `ShellCommand` wrapper in tmux layer |
| 2  | high     | fixed  | `resurrect/storage.go` | path traversal via user-typed snapshot name in `savePath` and `SaveFileExists` |
| 3  | high     | fixed  | `plugin/install.go` | git argument injection via unsanitised `p.Source` in `git clone` args |
| 4  | high     | fixed  | `state/session.go`, `window.go`, `pane.go` | state stores lack mutex — concurrent read from preview goroutines, write from tea loop |
| 5  | medium   | fixed  | `main.tmux` | `#{session_name}` not shell-escaped in `run-shell` binding args |
| 6  | medium   | fixed  | `plugin/config.go`, `uninstall.go` | plugin name from `path.Base(source)` can be `.` or `..`; `os.RemoveAll(p.Dir)` lacks containment check |
| 7  | medium   | fixed  | `logging/logging.go` | log file created `0644` (world-readable); log directory created `0755` |
| 8  | medium   | fixed  | `logging/sqlite.go` | sqlite debug database and directory created without restricted permissions |
| 9  | medium   | fixed  | `resurrect/pane_contents.go` | pane archive, extracted files, and directories created with permissive modes |
| 10 | medium   | fixed  | `resurrect/storage.go` | save directory created `0755`, save files created `0644` |
| 11 | medium   | fixed  | `logging/sqlite.go` | `sqliteSink.writer` goroutine has no panic recovery — panic in `writeEvent`/`writeSpan` deadlocks `Close()` |
| 12 | low      | fixed  | `menu/session.go`, `window.go`, `pane.go` | no control-character or colon validation on rename form inputs |
| 13 | low      | noted  | `resurrect/storage.go` | `os.ExpandEnv` on tmux-option-controlled storage dir allows env-var injection into path |
| 14 | info     | noted  | `tmux/types.go` | shared gotmuxcc client returned after mutex release — concurrent use depends on gotmuxcc being goroutine-safe |
| 15 | info     | noted  | `plugin/install.go`, `plugin/source.go` | plugin system is an explicit trust boundary — clones and executes arbitrary code from declared sources |

## detailed findings

### 1. shell injection in pane restore (high) — fixed

**files:** `internal/resurrect/restore.go`, `internal/tmux/restore.go`

the pane restore path builds a startup command (`cat "<path>"; exec <shell>`)
passed to tmux's `new-session`/`split-window`. `contentPath` is derived from
saved pane keys (including session name), and `defaultCmd` comes from
`tmux show-option default-command`. both were embedded unescaped in a shell
command string.

additionally, the tmux layer used gotmuxcc's `ShellCommand` wrapper for
`NewSession` and `SplitWindow`, which added its own quoting layer that could
interact poorly with already-quoted strings.

**fix (two layers of defence):**
- `resurrect/restore.go`: both `contentPath` and `defaultCmd` are now
  single-quote shell-escaped via a local `shellQuote()` function
- `tmux/restore.go`: `CreateSession` and `SplitPane` now use raw
  `client.Command(...)` argument passing instead of the `ShellCommand` helper,
  eliminating the extra quoting layer

### 2. path traversal via snapshot name (high) — fixed

**file:** `internal/resurrect/storage.go`

`name` comes from user input (save form, CLI flag, or env var) with no path
validation. a name like `../../tmp/pwned` writes outside the save directory.
`SaveFileExists` uses `filepath.Glob` with the name, which also accepts glob
metacharacters.

**fix:** added `ValidateSaveName()` rejecting `/`, `\`, leading `.`, and glob
chars `*?[`. called in `SaveFileExists`, `NewSaveForm.Update`, and the save
form submission path.

### 3. git argument injection in plugin install (high) — fixed

**file:** `internal/plugin/install.go`

`p.Source` from the tmux config `@plugin` value is passed as a positional arg
to `git clone` via `exec.Command`. a value like `--upload-pack=malicious-command`
would be interpreted as a git flag.

**fix:** added `--` end-of-options separator before `p.Source` in both the
direct and fallback clone paths.

### 4. state stores lack mutex (high) — fixed

**files:** `internal/state/session.go`, `window.go`, `pane.go`

preview lambdas returned as `tea.Cmd` closures execute in goroutines (by the
tea runtime) and call `store.Entries()` concurrently with `store.SetEntries()`
from the next update tick. the stores had no synchronisation beyond value
cloning.

**fix:** added `sync.RWMutex` to all three stores. readers use `RLock`,
writers use `Lock`.

### 5. `main.tmux` session name shell injection (medium) — fixed

**file:** `main.tmux`

`#{session_name}` in `run-shell` bindings is expanded by tmux before being
passed to `/bin/sh -c`. a session name containing single quotes breaks the
argument boundary.

**fix:** uses `#{q:session_name}` which tmux shell-escapes before substitution.

### 6. plugin name / uninstall containment (medium) — fixed

**files:** `internal/plugin/config.go`, `uninstall.go`

`path.Base(source)` of a crafted source could produce `.` or `..`, making
`p.Dir` point at the plugin directory itself. `os.RemoveAll` on uninstall would
delete the entire plugin directory.

**fix:** `parsePluginEntry` rejects pathological names (empty, `.`, `..`, or
containing path separators). `Uninstall` verifies `filepath.Clean(p.Dir)` is
prefixed by `pluginDir + separator`.

### 7. log file and directory permissions (medium) — fixed

**file:** `internal/logging/logging.go`

log files were created with `0644` (world-readable) and directories with `0755`.
they contain error messages with session names, socket paths, and structured
JSON traces with full startup config.

**fix:** files changed to `0600`, directories changed to `0700`.

### 8. sqlite debug database permissions (medium) — fixed

**file:** `internal/logging/sqlite.go`

the sqlite database stores `exe_path`, `cwd`, `argv_json`, `socket_path`,
`client_id`, `session_name` in the `runs` table. file and directory permissions
were not explicitly restricted.

**fix:** directory created with `0700`, database file pre-created with `0600`
before `sql.Open`. permission assertion added to tests.

### 9. pane archive and extracted file permissions (medium) — fixed

**file:** `internal/resurrect/pane_contents.go`

`os.Create` uses mode `0666` (before umask). pane archives contain terminal
scrollback that may include passwords or API keys. tar entry headers used `0644`.
extraction directories used `0755`.

**fix:** archive files created with `0600`, tar headers set to `0600`,
extraction directories created with `0700`, extracted files use `0600`.
permission assertion tests added.

### 10. save directory and file permissions (medium) — fixed

**file:** `internal/resurrect/storage.go`

save directories were created with `0755` and save files with `0644`. save files
contain full session state including working directories, commands, and pane
layout.

**fix:** directories changed to `0700`, files changed to `0600`. permission
assertion tests added.

### 11. sqlite writer panic recovery (medium) — fixed

**file:** `internal/logging/sqlite.go`

if `writeEvent` or `writeSpan` panics, the writer goroutine dies without
processing any subsequent `closeRequest`. `Close()` blocks forever on
`<-done`.

**fix:** wrapped the inner loop body in a closure with `defer recover()`. the
writer goroutine survives individual write panics and continues processing the
queue.

### 12. rename form control-character validation (low) — fixed

**files:** `internal/menu/session.go`, `window.go`, `pane.go`

session/window/pane rename forms accepted any text. newlines or colons in
names can confuse tmux's target-parsing syntax.

**fix:** session form rejects `\n`, `\r`, `\t`, and `:`. window and pane forms
reject `\n`, `\r`, `\t`.

### 13. `os.ExpandEnv` on storage path (low) — noted

**file:** `internal/resurrect/storage.go`

`os.ExpandEnv(d)` on the tmux option value means env vars in the path are
expanded. if `@tmux-popup-control-session-storage-dir` is attacker-controlled,
arbitrary directories can be targeted.

**status:** documented only. the threat model assumes the user controls their
own tmux config.

### 14. shared gotmuxcc client concurrency (info) — noted

**file:** `internal/tmux/types.go`

`newTmux` returns the cached `cachedClient` after releasing `clientMu`. multiple
goroutines (3 pollers + action handlers) then use the same client concurrently.
whether this is safe depends on gotmuxcc's internal thread-safety.

**status:** documented for upstream verification.

### 15. plugin execution trust boundary (info) — noted

**files:** `internal/plugin/install.go`, `source.go`

plugin management intentionally clones and executes local plugin code. tmux
plugin declarations are trusted input; installed plugins have arbitrary code
execution as the local user. compromise of a plugin source, repo, or config is
out of scope for local hardening.

**status:** documented. this is expected behaviour for plugin systems but should
be treated as an explicit trust boundary.

## items checked and found safe

- `runExecCommand` and all `exec.Command` call sites use separate args (no shell interpolation)
- `ExtractPaneArchive` has explicit `validateEntryName` rejecting `..` and absolute paths
- `main.go` `shellQuote()` correctly escapes embedded single quotes in `run-shell` commands
- `input.go` filter text handler rejects `unicode.IsControl` chars
- `config.go` integer/boolean env var parsing uses `strconv.Atoi`/`ParseBool` with safe fallbacks
- `GIT_TERMINAL_PROMPT=0` correctly set in git operations
- no hardcoded credentials found
- `testutil` package is only imported from `_test.go` files
- `backend.events` channel is buffered (16) with `ctx.Done()` fallback — no goroutine leak
- `clientMu` lock ordering is correct — no nested locking, no deadlock possible
- `watcher.go` goroutine lifecycle (wg/context/channel close) is correct
- preview sequence numbers correctly discard stale async responses

## validation

```
make test  — all packages pass
make build — clean compilation
```
