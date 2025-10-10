// Package ui contains the Bubble Tea program that powers the tmux popup menu.
// The package is structured so the Model type focuses on message orchestration,
// while dedicated helpers own navigation, input, rendering, and state updates.
//
// Message flow:
//   - Bubble Tea invokes Model.Update with incoming messages.
//   - Update forwards form-specific messages to the active form (session, window,
//     or pane rename prompts). When no form is active, the message is routed
//     through a typed handler registry so each tea.Msg is handled by a focused
//     function (for example, navigation for key presses or backend updates).
//   - Navigation helpers (internal/ui/navigation.go) manage the stack of menu
//     levels, cursor movement, and swap workflows. Filter/input helpers
//     (internal/ui/input.go) keep all text entry concerns isolated from the
//     Bubble Tea event loop.
//
// State ownership:
//   - Menu level state lives in internal/ui/state.Level, which tracks items,
//     filtering, selection, and viewport calculations.
//   - Session, window, and pane stores are provided by internal/state and kept
//     in sync by the dispatcher so menu loaders always see current tmux data.
//   - Command execution is handled through the internal/ui/command package,
//     letting actions run asynchronously via the central command bus.
//
// Backend interactions:
//   - A backend.Watcher streams tmux events; Update waits for those events and
//     hands them to applyBackendEvent, which refreshes the session/window/pane
//     stores and any on-screen menus that depend on them.
//   - Asynchronous menu loaders run via tea.Cmd values returned by helper
//     functions (e.g., loadMenuCmd). When a loader completes, the typed handler
//     for categoryLoadedMsg pushes the new level onto the stack.
//
// This separation keeps Model.Update compact and makes it easier to test
// independent concerns (navigation, filtering, backend sync) without needing to
// reason about the entire TUI at once.
package ui
