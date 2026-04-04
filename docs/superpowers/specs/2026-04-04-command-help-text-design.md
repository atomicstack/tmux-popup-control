# command prompt help text

## summary

add checked-in tmux command help metadata and use it to enrich the command
prompt submenu. the selected command should show a short summary beneath the
input field, and command flag/parameter completion rows should show aligned
descriptions in the completion popup.

## current state

the command prompt submenu already has:
- command-name ghost completion
- argument completion with a popup dropdown
- labeled completion rows, but only a single visible column

an external file exists at `~/git_tree/tmux/command-summary.md` with:
- one short summary per tmux command
- short descriptions for each flag or positional parameter

the current UI does not use this file.

## requirements

### checked-in go help data

the markdown file should be parsed into a standalone native go data structure
that lives in this repository and can be edited later without depending on
runtime markdown parsing.

the native structure should expose:
- a summary for each command
- ordered argument help entries for each command

each argument help entry should contain:
- the visible argument token, such as `-t` or `template`
- the description text

it is acceptable for the checked-in go file to be generated once from the
markdown source, but the UI must consume only the go package.

### prompt help line

when the user is in the command submenu, render one dim help line directly
under the input field showing the summary for the currently selected command.

behavior:
- if the highlighted command has a summary, show it
- if there is no summary, show nothing
- when filtering changes the selected command, the help line updates
- when the user has typed a full command and moved into argument completion,
  the help line should follow that resolved command rather than disappearing

example:

```text
» move-window -t
move a window to another position
```

### completion popup descriptions

for command flag and positional completion rows, render descriptions in a
second aligned column inside the popup.

example:

```text
-a                  insert after target window
-r                  renumber windows
-s <src-window>     source window
-t <dst-window>     destination window
```

rules:
- do not render a header row
- keep live runtime candidates plain for now, such as `main:0` or `%3`
- rows without descriptions should still render correctly
- alignment should be computed from the visible left column width

## architecture

### new package: `internal/cmdhelp`

add a small package that contains:
- exported go types for command help metadata
- a checked-in `map[string]CommandHelp`

proposed types:

```go
type CommandHelp struct {
	Summary string
	Args    []ArgHelp
}

type ArgHelp struct {
	Name        string
	Description string
}
```

lookup is by canonical command name.

### generator tool

add a small repo-local tool that reads the markdown file and emits the checked
in go data file. this tool is for maintenance, not for runtime use.

the generated go output should be deterministic:
- stable command ordering
- stable argument ordering
- gofmt-compatible output

### ui wiring

the command preload path should attach help metadata to the command menu model.

the completion dropdown should support optional per-item descriptions:
- command flag completion rows use command help descriptions
- command positional completion rows use command help descriptions
- live resolved values do not carry descriptions

the input prompt renderer should ask the model for a current command summary
string and render it as a dim line below the prompt when available.

## testing

add focused tests for:
- markdown parsing into `CommandHelp`
- deterministic go data generation
- prompt help line rendering for selected command and resolved command input
- aligned popup rendering for described command arguments
- unchanged plain rendering for live value candidates
