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
- internal/tmux uses lightweight window/session handle interfaces, letting lifecycle helpers operate on abstracted handles while the production client wraps gotmuxcc types.
- The fake tmux client now vends stub handles with call tracking, unlocking unit coverage for Select/Kill-window flows, session detach/kill/rename, and SwitchPane command wiring.
- Integration coverage exercises creating/killing windows and detaching/killing sessions against a live tmux server to mirror the new fake scenarios.
- internal/ui/state now splits `Level` responsibilities across focused files (level.go, selection.go, cursor.go, filter.go, items.go) to keep filtering, cursor movement, and selection logic isolated and easier to maintain.
- internal/tmux/tmux.go was decomposed into dedicated files (`types.go`, `snapshots.go`, `windows.go`, `panes.go`, `sessions.go`, `command.go`), isolating fetch logic, operations, and shared utilities so each concern stays below 200 lines and is simpler to navigate.
- All Go tests (`GOCACHE=$(pwd)/.gocache go test ./...`) pass with the new layout.
- Swapped the tmux client dependency from gotmux to gotmuxcc, updating imports and build wiring to use the new control-mode transport.
- Backend polling is now rate-limited inside `internal/backend/watcher`, keeping automatic refreshes below ~4Hz while leaving user-triggered commands unthrottled.
- Snapshot loading became resilient to control-mode races: session/window/pane fetchers fall back to direct tmux calls, fill in missing session IDs, and ignore vanished resources instead of failing whole queries.
- Session/window helpers pick up more fallbacks (e.g., renames, kills) so actions succeed even when stale data races with tmux; tests now disable throttling via control-mode and validate the new behaviour.
- The test harness logs and tears down each tmux server via control mode (`KillServer`), clearing `TMUX` env vars so we never reuse the user’s session and preventing orphaned tmux processes between runs.
- Hardened the vendored gotmuxcc router so event emission respects router shutdown, avoiding `send on closed channel` panics when tmux exits during tests.
- internal/tmux session detach/kill helpers now poll for control-mode visibility, fall back to direct tmux commands, and wait for `has-session` to fail so integration tests stop flaking when new sessions race control-mode.
- gotmuxcc control transport now issues a one-shot `attach-session` to the first existing session when available, preventing throwaway sessions while keeping switch menus populated; unit tests cover the detection path.
- Planned the preview feature architecture: previews will live alongside the UI model with per-level state, fire asynchronous `tmux` fetch commands for sessions/windows/panes, and render via a dedicated preview section in the View so cursor moves never block the TUI.
- Added tmux preview helpers (session/window listings plus pane capture) and new UI preview plumbing that fires asynchronous commands whenever the session/window/pane switch menus change selection, tracking requests via per-level state and tests so stale responses are ignored without blocking cursor motion.
- Rendered the preview block inside the TUI with new themed styles, trimming output to a manageable height and surfacing either content, loading state, or errors; view/unit tests cover the new presentation.
- Switch actions now detect the invoking tmux client via `display-message`, pass that client id through the UI/menu context, and target it in `switch-client` calls so pane/window/session switches work again even with the new attach-based control transport; session/window previews fall back to cached snapshot data so they display reliably without extra tmux CLI calls.
- Every control-mode client we spawn (sessions/windows helpers, snapshots, switch-client flows) now calls `Close()` once finished, so tmux-popup-control no longer leaks background `tmux -C` processes after exiting.
- Replaced per-call `newTmux`/`defer client.Close()` pattern with a shared long-lived control-mode connection: `newTmux` now caches a single `tmuxClient` behind a `sync.Mutex`, returning the cached instance when the socket path matches and reconnecting when it changes. Removed all 28 `defer client.Close()` calls across 6 files (`client.go`, `command.go`, `sessions.go`, `windows.go`, `panes.go`, `snapshots.go`). Added `tmux.Shutdown()` for cleanup at app exit (called via `defer` in `app.Run`). Updated `withStubTmux` test helper to save/restore cached state, and added `TestShutdownClosesClient` and `TestNewTmuxCachesConnection` tests.

- Session:switch and window:switch previews now capture the active pane's content via `tmux capture-pane` (async, same as pane:switch) rather than a static window/pane list; `activePaneIDForSession` and `activePaneIDForWindow` helpers find the best pane from cached state, falling back to text lists when no pane data is available. `maxVisibleItems()` now subtracts the preview block height (blank + title + content lines) from the item viewport budget so previews are never clipped off-screen by `limitHeight`; a 3-line reservation holds space while the async capture is in flight. `handlePreviewLoadedMsg` calls `syncViewport` after storing loaded data so the cursor stays visible with the updated budget. New unit tests cover the pane-capture path, the `Current`-pane preference, the window-list fallback, and the `maxVisibleItems` shrinkage.

- Added dedicated unit suites for `internal/ui/state` covering cursor navigation, viewport math, filter editing, fuzzy matching, and cloning helpers to raise coverage across the new state package split.

- Side-by-side preview panel implemented: the preview now renders in a rounded-border box on the right side of the popup (60% of total width) when the current level is a switch menu (session/window/pane) and the terminal is wide enough. Mouse wheel scrolling supported. Falls back to vertical layout when too narrow. `hasSidePreview()`, `previewPanelWidth()`, `menuColumnWidth()`, `renderPreviewPanel()`, `viewSideBySide()`, `handleMouseMsg()` added to `internal/ui/view.go`. `tea.WithMouseCellMotion()` enabled in `internal/app/app.go`. `scrollOffset int` added to `previewData`; pane previews default-scroll to the bottom so recent output is visible. `maxVisibleItems()` no longer reserves inline preview rows when in side-by-side mode.

- Fixed `TestFetchSnapshotsIntegration` failure caused by tab-separated format strings being split by tmux's control-mode argument parser. Root cause: `List*Format` functions in gotmuxcc were not quoting format strings before sending them via control-mode. Fix implemented by the gotmuxcc maintainer (added `quoteArgument()` in vendor `options.go`, applied in `display.go`). A workaround added to `snapshots.go` was removed once the upstream fix landed.

- Fixed `KillWindows` fragility: replaced `findWindow()` + `window.Kill()` with `client.Command("kill-window", "-t", target)` directly, avoiding issues when newly-created windows aren't yet visible via `ListAllWindows`. Same applied to `KillWindow`. Unit tests updated.

- Fixed preview panel showing blank content: `PanePreview` was using gotmuxcc's `CapturePane` via control-mode transport, which produced unreliable/empty output. Switched to `runExecCommand("tmux", "capture-pane", "-p", ...)` (direct exec, same pattern as other exec-based ops). Added ANSI escape stripping (`ansiEscapeRe`) so raw terminal output is cleaned before display. Removed `CapturePane` from `tmuxClient` interface and updated `preview_test.go` to use `withStubCommander`. All tests pass.
