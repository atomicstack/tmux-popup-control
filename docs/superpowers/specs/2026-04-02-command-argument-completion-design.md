# command argument tab completion

## summary

extend the command-prompt submenu with intelligent tab completion for tmux
command arguments. currently, only command names have ghost-text autocomplete.
this feature adds completion for flags, flag values, and positional arguments,
with a dropdown popup for multiple candidates and contextual ghost hints.

## current state

the command menu loads ~90 tmux commands via `tmux list-commands`. each
command's full synopsis is available (e.g.
`attach-session (attach) [-dErx] [-c working-directory] [-f flags] [-t target-session]`).

the existing completion system:
- ghost text shows the remaining suffix of the best-matching command name
- tab accepts the ghost text, inserting the full command name
- enter executes the filter text as a tmux command
- the command list is pre-loaded at startup and cached

## feature overview

after the command name is entered, the system will:
1. parse the command's synopsis to understand its flags and positional args
2. analyse the current input to determine the completion context
3. show ghost hints for argument type names (e.g. `src-window` after `-s`)
4. show a dropdown popup with live candidates when values can be resolved
5. support type-to-filter, arrow navigation, tab-accept, escape-dismiss

## ux specification

### ghost hint behaviour

the ghost text in the filter prompt extends to cover argument hints:

| state | prompt display |
|---|---|
| typed `swap-window -s` | ghost: ` src-window` (type name from synopsis, dimmed) |
| dropdown open, arrow to `main:0` | ghost updates to: `main:0` |
| user types `ma` with one match `main:0` | ghost: `in:0` (unique suffix) |
| user types `ma` with multiple matches | no ghost suffix, dropdown filters |
| user types free-form for non-completable type | no ghost, no dropdown |

ghost text reflects the **current best completion state**:
- **no input, completable type**: show the argument type name as placeholder
- **dropdown selection highlighted**: show the selected candidate
- **partial input, unique prefix match**: show the remaining suffix
- **partial input, multiple matches**: no ghost (dropdown shows options)
- **non-completable argument type**: no ghost

### dropdown popup behaviour

**trigger:** appears automatically when the analyser detects a completion point
with resolvable candidates. this happens:
- after a space following a flag that expects a value (e.g. `-t `)
- after the command name when positional args are expected
- after completing a previous argument, if another is expected

**rendering:** a floating box overlaying the menu item list, anchored at the
cursor column position in the prompt. max 10 visible rows. distinct border and
background styling to separate from the menu. scrolls when more than 10 items.

**interaction:**
- **up/down arrows**: navigate the dropdown (instead of menu cursor)
- **typing**: text goes into the prompt AND filters the dropdown
- **tab**: accepts the selected/highlighted item, inserts into prompt, dismisses dropdown
- **escape**: dismisses dropdown without accepting, typed text remains
- **enter while dropdown visible**: accepts the selection, inserts into prompt, does NOT execute the command

**auto-dismiss:** the dropdown closes when:
- no candidates match the typed filter
- the user moves the cursor away from the completion point (e.g. backspace past the flag)
- escape is pressed

### completion flow example

```
1. user types: swap-window
   → command ghost completed via existing system

2. user presses space
   → analyser: expects flags or positional args
   → dropdown: shows available flags [-d, -s, -t]
   
3. user types: -s
   → dropdown dismissed (flag accepted)
   → ghost: " src-window" (argument type hint)

4. user presses space
   → analyser: -s expects src-window value
   → dropdown: shows window list [main:0, main:1, work:0, work:1]
   → ghost: first dropdown item shown

5. user arrows down to "work:0"
   → ghost updates to "work:0"

6. user presses tab
   → "work:0" inserted into prompt
   → prompt now: "swap-window -s work:0"
   → analyser: next completion point
   → dropdown: shows remaining flags [-d, -t]
```

## architecture

### new package: `internal/cmdparse/`

contains all command parsing, analysis, and resolution logic. no UI
dependencies — pure data transformation.

#### schema types (`schema.go`)

```go
// CommandSchema is the parsed representation of a tmux command synopsis.
type CommandSchema struct {
    Name        string
    Alias       string
    BoolFlags   []rune          // flags that take no argument (e.g. -d, -E)
    ArgFlags    []ArgFlagDef    // flags that take a typed argument
    Positionals []PositionalDef // positional arguments after all flags
}

// ArgFlagDef is a flag that expects a typed argument value.
type ArgFlagDef struct {
    Short   rune   // e.g. 't'
    ArgType string // e.g. "target-session", "working-directory"
}

// PositionalDef is a positional argument.
type PositionalDef struct {
    Name     string // e.g. "key", "command", "template"
    Required bool   // true if not wrapped in []
    Variadic bool   // true if followed by "..."
}
```

#### synopsis parser (`parse.go`)

parses raw `list-commands` lines into `CommandSchema`. the format is regular:

- `command-name (alias)` — name and optional alias
- `[-abcXY]` — clustered boolean flags (no arguments)
- `[-f flag-arg]` — flag with typed argument
- `word` — required positional
- `[word]` — optional positional
- `[word ...]` — optional variadic positional

parsing strategy: split on whitespace, process each token/group. the `(alias)`
is easy to detect. flag groups starting with `[-` contain only booleans if they
have no space-separated second word. individual `-X arg-type` pairs are arg
flags. remaining tokens after all flags are positionals.

edge cases:
- some commands have no flags at all (`kill-server`)
- some have only boolean flags (`kill-session [-aC]`)
- `display-menu` has `name [key] [command] ...` with unusual positional patterns
- `if-shell` has `command [command]` — same-named positional twice
- `bind-key` has `key [command [argument ...]]` — nested optionals

the parser must handle these but does not need to model nesting — flattening
to a sequence of positionals is sufficient for completion purposes.

#### input analyser (`analyse.go`)

given a `CommandSchema` and the current filter text, determines what completion
is appropriate:

```go
// CompletionContext describes what kind of completion is available at the
// current cursor position.
type CompletionContext struct {
    Kind      ContextKind // CommandName, FlagName, FlagValue, PositionalValue
    ArgType   string      // for FlagValue/PositionalValue: the argument type
    TypeLabel string      // display label (e.g. "src-window") for ghost hint
    Prefix    string      // text already typed for the current token
    FlagsUsed []rune      // flags already present in the input
}

type ContextKind int

const (
    ContextNone ContextKind = iota
    ContextCommandName
    ContextFlagName
    ContextFlagValue
    ContextPositionalValue
)
```

analysis strategy:
1. split input into tokens via `strings.Fields`
2. first token is command name — look up schema
3. walk remaining tokens: if token starts with `-`, it's a flag; consume its argument if it's an arg flag
4. track which flags have been used
5. determine cursor position: at end of input? mid-token? after a space?
6. return the appropriate context

#### value resolver (`resolve.go`)

maps argument types to completion candidates. uses an interface so the UI can
inject state store accessors:

```go
// Resolver provides completion candidates for argument types.
type Resolver interface {
    Resolve(argType string) []string
}
```

resolvable types and their sources:

| argument type | source |
|---|---|
| `target-session`, `session-name` | session store → session names |
| `target-window`, `window-name`, `src-window`, `dst-window` | window store → `session:window` format |
| `target-pane`, `src-pane`, `dst-pane`, `pane` | pane store → `%ID` format |
| `target-client` | tmux `list-clients` |
| `key-table` | hardcoded: `root`, `prefix`, `copy-mode`, `copy-mode-vi` |
| `buffer-name` | tmux `list-buffers` |
| `command` (positional) | command schema registry names |
| `layout-name` | hardcoded: `even-horizontal`, `even-vertical`, `main-horizontal`, `main-vertical`, `tiled` |
| `prompt-type` | hardcoded: `command`, `search`, `target`, `window-target` |

non-resolvable types (`format`, `filter`, `shell-command`, `working-directory`,
`style`, `border-lines`, `border-style`, `environment`, `note`, `flags`,
`size`, `position`, `width`, `height`, etc.) return nil — no dropdown, no ghost
hint beyond the type name placeholder.

### ui changes

#### completion state (`internal/ui/completion.go`)

new file managing the dropdown state as part of the `Model`:

```go
// completionState tracks the state of the argument completion dropdown.
type completionState struct {
    visible    bool
    items      []string // all candidates
    filtered   []string // after prefix filter
    cursor     int      // index in filtered
    prefix     string   // typed text for current token
    anchorCol  int      // prompt column where dropdown anchors
    argType    string   // for ghost hint when no input
    typeLabel  string   // display name of the argument type
}
```

methods:
- `filter(prefix string)` — re-filters items, resets cursor
- `selected() string` — returns the currently highlighted candidate
- `view(maxWidth, maxHeight int) string` — renders the dropdown box
- `ghostHint() string` — returns appropriate ghost text based on state

#### key routing changes

when `completionState.visible` is true, key handling changes:

| key | normal behaviour | with dropdown |
|---|---|---|
| up/down | menu cursor | dropdown cursor |
| tab | ghost complete | accept selected, insert into prompt |
| escape | pop level | dismiss dropdown |
| enter | execute command | accept selected, stay in prompt |
| typing | filter menu items | filter dropdown + update prompt |

this is implemented by checking `m.completion.visible` at the top of
`handleKeyMsg` and `handleTextInput`, routing to completion-specific handlers
before the normal paths.

#### ghost hint integration

`autoCompleteGhost()` in `input.go` is extended:

1. if completion dropdown is visible and a candidate is selected → ghost is
   the selected candidate minus the typed prefix
2. if no dropdown but the analyser indicates a completable type with no
   typed prefix → ghost is the type label (e.g. `src-window`)
3. if partial typed text matches exactly one candidate → ghost is the
   unique suffix
4. otherwise → existing command-name ghost logic (for the first token)

#### dropdown rendering

the dropdown renders as an overlay in `viewVertical` and `viewSideBySide`.
it's positioned:
- horizontally: anchored at the completion token's column in the prompt
- vertically: above the prompt line (growing upward), since the prompt is
  at the bottom

the dropdown is rendered AFTER the main view, overwriting the relevant
lines of the menu item area. this uses lipgloss's `Place` or direct
string manipulation to overlay the box.

styling:
- border: single-line rounded, using `styles.CompletionBorder`
- background: distinct from both menu and preview (e.g. `color("236")`)
- selected item: highlighted background matching the menu selection style
- max 10 visible items, scroll indicator when more exist

### flag completion details

when the analyser determines the user is at a flag-name position (typed
`-` or at a space after a completed argument), available flags are shown:

- only flags not already used in the input
- displayed as `-f arg-type` for arg flags, `-d` for boolean flags
- sorted: arg flags first (more useful), then boolean flags
- the dropdown label shows the flag with its argument type for context

### schema registry

the parsed schemas are stored in a registry (map of command name/alias to
`*CommandSchema`) built once from the pre-loaded command items. this happens
in `handleCommandPreloadMsg` when the cache is populated:

```go
m.commandSchemas = cmdparse.BuildRegistry(items)
```

the `BuildRegistry` function parses each item's `Label` field (the full
synopsis line) and indexes by both name and alias.

## testing strategy

### unit tests

- **`internal/cmdparse/parse_test.go`**: parse every `list-commands` line.
  golden file containing all 90 parsed schemas. ensures parser handles all
  edge cases. test individual tricky commands explicitly (bind-key,
  display-menu, if-shell).

- **`internal/cmdparse/analyse_test.go`**: table-driven tests for cursor
  context detection. cases: mid-command-name, after-command-name-space,
  after-flag, after-flag-with-arg-space, mid-flag-value, after-positional,
  all-flags-used, unknown-command.

- **`internal/cmdparse/resolve_test.go`**: mock resolver returns known
  values. verify correct argument types are mapped.

- **`internal/ui/completion_test.go`**: dropdown state management — filter,
  cursor movement, selection, ghost hint computation.

- **`internal/ui/` harness tests**: drive the model through completion
  scenarios using the existing `Harness`. verify key routing (arrows,
  tab, escape, enter) behaves correctly with dropdown visible/hidden.

### integration tests

- navigate to command menu, type a command, verify dropdown appears with
  real tmux data (sessions/windows), tab-complete a value, execute.

## implementation chunks

the work is split into independently implementable and testable chunks:

### chunk 1: schema parser
- `internal/cmdparse/schema.go` — types
- `internal/cmdparse/parse.go` — synopsis parser
- `internal/cmdparse/parse_test.go` — unit tests + golden file
- no UI changes, no integration needed

### chunk 2: input analyser
- `internal/cmdparse/analyse.go` — context detection
- `internal/cmdparse/analyse_test.go` — table-driven tests
- depends on chunk 1 (uses CommandSchema)

### chunk 3: value resolver
- `internal/cmdparse/resolve.go` — resolver interface + concrete impl
- `internal/cmdparse/resolve_test.go` — tests with mock data
- depends on chunk 1 (uses schema types)

### chunk 4: completion dropdown widget
- `internal/ui/completion.go` — state, rendering, ghost hints
- `internal/ui/completion_test.go` — unit tests
- no dependency on chunks 1-3 (can use stub data)

### chunk 5: integration — wire it all together
- build schema registry in `commands.go` from preloaded items
- extend `autoCompleteGhost()` in `input.go`
- add completion key routing in `navigation.go` and `input.go`
- render dropdown overlay in `view.go`
- extend `handleTextInput` to trigger analysis on each keystroke
- harness tests for end-to-end key sequences
- integration tests with live tmux

### chunk 6: polish and edge cases
- handle backspace through completed values (re-open dropdown)
- handle cursor movement within the command (left/right arrow)
- handle window resize while dropdown is open
- verify styling across different terminal sizes
- verify no regression in existing command-name completion

## out of scope

- completion for `set-option`/`show-option` option names (separate data source)
- filesystem path completion for `working-directory`/`start-directory` types
- command history-based completion
- completion for nested command arguments (e.g. the `command` arg in `bind-key`)
  beyond offering command names — the nested command's own args are not completed
