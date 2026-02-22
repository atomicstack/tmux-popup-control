# TODO

- (TBD) Identify further UI cleanups or feature work once the refactor settles.
- Review whether remaining tmux helpers (e.g., LinkWindow/MoveWindow/SwapWindows, pane move/break flows) need similar handle abstractions or expanded fake scenarios.
- Consider extending integration coverage to cover pane moves/swaps and multi-session client interactions if gaps surface during manual testing.
- Evaluate whether additional message handlers in model.go (e.g., handler registry management) should be decomposed further or covered with focused tests now that structural pieces live in dedicated files.
