## Current state

Here’s what’s happened so far:

- internal/ui/state/level.go owns all menu level state (fields, cursor movement, filtering, fuzzy match helpers).
- internal/ui/model.go aliases that `state.Level`, calling its exported methods instead of the old private struct logic; helpers such as limitHeight, applyWidth, renderLines, truncateText, waitForBackendEvent, and applyBackendEvent remain in the UI package.
- Tests (internal/ui/model_test.go, internal/ui/integration_test.go) were updated to reference the capitalised fields/methods provided by the new Level type (e.g., Cursor, IndexOf, Selected).
- Message handling is now routed through a typed handler registry inside model.Update, letting each message type delegate to focused helpers.
- Menu navigation/stack helpers and swap/escape flows now live in internal/ui/navigation.go.
- Filter/input handling, including cursor styling and prompt rendering, now lives in internal/ui/input.go.
- internal/ui/doc.go documents the UI package layout, message flow, and backend integration for future contributors.
- Rendering utilities (View, breadcrumb helpers, width/height/clipping logic) were moved into internal/ui/view.go.
- Prompt handling goes through a shared withPrompt helper, reducing duplication across form, swap, and command prompts.
- New unit tests cover navigation (escape behaviour), input cursor math, and the prompt helper to guard the refactor.
- Command/result handling and menu loader helpers now live in internal/ui/commands.go, keeping Model focused on wiring.
- Remaining form/prompt/back-end handlers were extracted from model.go into focused files (forms.go, prompt.go, backend.go), and layout helpers moved into view.go, further shrinking the core model.
- internal/tmux gained injectable exec/tmux constructors and a new test suite covering helper behaviours (argument trimming, fallbacks, formatting, validation) to raise coverage without requiring a live tmux server.
- internal/tmux now also ships integration coverage that spins up a temporary tmux server (via StartTmuxServer) to exercise snapshot fetching and window renaming against a real tmux instance.
- internal/tmux uses lightweight window/session handle interfaces, letting lifecycle helpers operate on abstracted handles while the production client wraps gotmux types.
- The fake tmux client now vends stub handles with call tracking, unlocking unit coverage for Select/Kill-window flows, session detach/kill/rename, and SwitchPane command wiring.
- Integration coverage exercises creating/killing windows and detaching/killing sessions against a live tmux server to mirror the new fake scenarios.
- internal/ui/state now splits `Level` responsibilities across focused files (level.go, selection.go, cursor.go, filter.go, items.go) to keep filtering, cursor movement, and selection logic isolated and easier to maintain.
- internal/tmux/tmux.go was decomposed into dedicated files (`types.go`, `snapshots.go`, `windows.go`, `panes.go`, `sessions.go`, `command.go`), isolating fetch logic, operations, and shared utilities so each concern stays below 200 lines and is simpler to navigate.
- All Go tests (`GOCACHE=$(pwd)/.gocache go test ./...`) pass with the new layout.

Outstanding work from the broader plan:
- (TBD) Identify further UI cleanups or feature work once the refactor settles.
- Review whether remaining tmux helpers (e.g., LinkWindow/MoveWindow/SwapWindows, pane move/break flows) need similar handle abstractions or expanded fake scenarios.
- Consider extending integration coverage to cover pane moves/swaps and multi-session client interactions if gaps surface during manual testing.
- Evaluate whether additional message handlers in model.go (e.g., handler registry management) should be decomposed further or covered with focused tests now that structural pieces live in dedicated files.
- Added dedicated unit suites for `internal/ui/state` covering cursor navigation, viewport math, filter editing, fuzzy matching, and cloning helpers to raise coverage across the new state package split.
