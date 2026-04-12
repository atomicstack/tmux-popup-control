package menu

import (
	"github.com/atomicstack/tmux-popup-control/internal/logging"
	"github.com/atomicstack/tmux-popup-control/internal/tmux"
)

// loadUserOptionsFn fetches the set of user-defined tmux option names (those
// starting with `@`) from the running tmux server. It is swappable for tests
// so completion flows can be exercised without a live tmux.
var loadUserOptionsFn = func(socket string) ([]string, error) {
	return tmux.UserOptions(socket)
}

// LoadUserOptions returns the sorted list of user-defined option names
// visible to the menu context. Errors are logged at the call site rather
// than surfaced: completion should still work against the static catalog
// if the live query fails.
func LoadUserOptions(ctx Context) ([]string, error) {
	span := logging.StartSpan("menu", "tmux.user_options", logging.SpanOptions{
		Target: "user-options",
		Attrs: map[string]any{
			"socket_path": ctx.SocketPath,
		},
	})
	names, err := loadUserOptionsFn(ctx.SocketPath)
	span.AddAttr("name_count", len(names))
	span.End(err)
	if err != nil {
		return nil, err
	}
	return names, nil
}
