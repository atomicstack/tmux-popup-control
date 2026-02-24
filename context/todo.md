# TODO

- (TBD) Identify further UI cleanups or feature work once the refactor settles.
- Review whether remaining tmux helpers (e.g., LinkWindow/MoveWindow/SwapWindows, pane move/break flows) need similar handle abstractions or expanded fake scenarios.
- Consider extending integration coverage to cover pane moves/swaps and multi-session client interactions if gaps surface during manual testing.
- Evaluate whether additional message handlers in model.go (e.g., handler registry management) should be decomposed further or covered with focused tests now that structural pieces live in dedicated files.
- Consider adding reconnection logic to the shared control-mode connection: if the cached client's transport dies mid-session, `newTmux` could detect the error and transparently reconnect. Low priority since the popup runs inside `tmux display-popup` and a dead tmux server means the popup is closing anyway.
- Investigate pre-existing `TestRootMenuRendering` integration test failure ("cannot create : Directory nonexistent") which appears unrelated to recent changes.
