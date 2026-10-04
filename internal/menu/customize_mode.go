package menu

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// CustomizeModeAction opens customize-mode in the pane the popup was launched
// from. The popup is a modal floating pane, and tmux resolves an untargeted
// command to the modal pane, so without an explicit target customize-mode
// would open inside the popup and vanish when it closes.
func CustomizeModeAction(ctx Context, item Item) tea.Cmd {
	args := []string{"customize-mode"}
	if pane := strings.TrimSpace(ctx.CurrentPaneID); pane != "" {
		args = append(args, "-t", ctx.paneTarget(pane))
	}
	return func() tea.Msg {
		if err := runTmuxCommand(ctx.SocketPath, args...); err != nil {
			return ActionResult{Err: fmt.Errorf("tmux customize-mode failed: %w", err)}
		}
		return ActionResult{Info: "Executed customize-mode"}
	}
}
