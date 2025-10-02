# Testing TODOs

These items track the next steps for exercising `tmux-popup-control` end-to-end.

- [ ] Spin up the application inside the temporary tmux server provided by
  `internal/testutil.StartTmuxServer` and drive an interaction cycle.
- [ ] Use `CapturePane` to grab the rendered popup after an action and compare it
  with a golden fixture so regressions in layout, colouring, or key hints are
  immediately visible.
- [ ] Extend the helper to feed scripted key sequences into the popup (for
  example via `tmux send-keys`) so navigation across submenus can be tested.
- [ ] Ensure the temporary server is torn down even when tests fail so existing
  user sessions remain untouched.

