package menu

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func CustomizeModeAction(ctx Context, item Item) tea.Cmd {
	return func() tea.Msg {
		if err := runTmuxCommand(ctx.SocketPath, "customize-mode"); err != nil {
			return ActionResult{Err: fmt.Errorf("tmux customize-mode failed: %w", err)}
		}
		return ActionResult{Info: "Executed customize-mode"}
	}
}
