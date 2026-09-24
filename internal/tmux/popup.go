package tmux

import "context"

// PopupStyle holds the optional styling applied to the floating pane the
// plugin opens as its popup. Empty fields are left to tmux's own defaults, so
// the plugin never imposes a look of its own.
type PopupStyle struct {
	BorderLines string // new-pane -B (see pane-border-lines)
	BorderStyle string // new-pane -S and -R
	Style       string // new-pane -s
}

// ResolvePopupStyle reads the popup styling from the TMUX_POPUP_CONTROL_POPUP_*
// env vars, falling back to the matching @tmux-popup-control-popup-* options.
func ResolvePopupStyle(socketPath string) PopupStyle {
	ctx := context.Background()
	return PopupStyle{
		BorderLines: envOrOption(ctx, socketPath, "TMUX_POPUP_CONTROL_POPUP_BORDER_LINES", "@tmux-popup-control-popup-border-lines"),
		BorderStyle: envOrOption(ctx, socketPath, "TMUX_POPUP_CONTROL_POPUP_BORDER_STYLE", "@tmux-popup-control-popup-border-style"),
		Style:       envOrOption(ctx, socketPath, "TMUX_POPUP_CONTROL_POPUP_STYLE", "@tmux-popup-control-popup-style"),
	}
}

// Args returns the new-pane flags for the fields that are set. The popup is
// modal and therefore always the active pane, but the border style is applied
// to both the active (-S) and inactive (-R) borders so it holds either way.
func (p PopupStyle) Args() []string {
	var args []string
	if p.BorderLines != "" {
		args = append(args, "-B", p.BorderLines)
	}
	if p.BorderStyle != "" {
		args = append(args, "-S", p.BorderStyle, "-R", p.BorderStyle)
	}
	if p.Style != "" {
		args = append(args, "-s", p.Style)
	}
	return args
}
